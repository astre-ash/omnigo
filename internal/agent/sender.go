package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/astre-ash/omnigo/pkg/retry"
)

const (
	defaultClientTimeout = 5 * time.Second
	endpointUpdateBatch  = "/updates/"
)

type Compressor interface {
	Compress(data []byte) ([]byte, error)
}

type MetricSender struct {
	serverAddr string
	client     retry.HTTPDoer
	compressor Compressor
}

func NewMetricSender(serverAddr string, client retry.HTTPDoer, compressor Compressor) *MetricSender {
	if client == nil {
		client = &http.Client{Timeout: defaultClientTimeout}
	}

	return &MetricSender{
		serverAddr: strings.TrimRight(serverAddr, "/"),
		client:     client,
		compressor: compressor,
	}
}

func (s *MetricSender) SendBatch(ctx context.Context, metrics []domain.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	payloadBytes, isCompressed, err := s.preparePayload(metrics)
	if err != nil {
		return fmt.Errorf("failed to prepare batch payload: %w", err)
	}

	return s.sendRequest(ctx, endpointUpdateBatch, payloadBytes, isCompressed)
}

func (s *MetricSender) preparePayload(payload any) ([]byte, bool, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if s.compressor != nil {
		compressedData, err := s.compressor.Compress(data)
		if err != nil {
			return nil, false, fmt.Errorf("failed to compress payload: %w", err)
		}
		return compressedData, true, nil
	}

	return data, false, nil
}

func (s *MetricSender) sendRequest(ctx context.Context, path string, bodyBytes []byte, isCompressed bool) error {
	targetUrl, err := url.JoinPath(s.serverAddr, path)
	if err != nil {
		return fmt.Errorf("failed to build request url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetUrl, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if isCompressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, path, strings.TrimSpace(string(respBody)))
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
