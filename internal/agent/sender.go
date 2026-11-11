package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/astre-ash/omnigo/internal/domain"
)

type Compressor interface {
	Compress(data []byte) ([]byte, error)
}

type MetricSender struct {
	serverAddr string
	client     *http.Client
	compressor Compressor
}

func NewMetricSender(serverAddr string, compressor Compressor) *MetricSender {
	serverAddr = strings.TrimRight(serverAddr, "/")

	return &MetricSender{
		serverAddr: serverAddr,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		compressor: compressor,
	}
}

func (s *MetricSender) SendBatch(ctx context.Context, metrics []domain.Metrics) error {

	if len(metrics) == 0 {
		return nil
	}
	return s.sendJSON(ctx, "/updates/", metrics)
}

func (s *MetricSender) sendJSON(ctx context.Context, path string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	var body io.Reader
	isCompressed := false

	if s.compressor != nil {
		compressedData, err := s.compressor.Compress(data)
		if err != nil {
			return fmt.Errorf("failed to compress payload: %w", err)
		}
		body = bytes.NewReader(compressedData)
		isCompressed = true
	} else {
		body = bytes.NewReader(data)
	}

	url := s.serverAddr + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")

	if isCompressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request to %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code from %s: %d", path, resp.StatusCode)
	}

	return nil
}
