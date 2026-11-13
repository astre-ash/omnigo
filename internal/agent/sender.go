package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/astre-ash/omnigo/pkg/retry"
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
	return &MetricSender{
		serverAddr: strings.TrimRight(serverAddr, "/"),
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

	payloadBytes, isCompressed, err := s.preparePayload(metrics)
	if err != nil {
		return fmt.Errorf("failed to prepare batch payload: %w", err)
	}

	return retry.Do(ctx, retry.DefaultDelays(), s.isRetryable, func() error {
		return s.sendRequest(ctx, "/updates/", payloadBytes, isCompressed)
	})
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
	body := bytes.NewReader(bodyBytes)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.serverAddr+path, body)
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
		err := fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, path, strings.TrimSpace(string(respBody)))

		if resp.StatusCode >= http.StatusInternalServerError {
			return &serverError{err: err}
		}
		return err
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

type serverError struct {
	err error
}

func (e *serverError) Error() string {
	return e.err.Error()
}

func (s *MetricSender) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	var srvErr *serverError
	if errors.As(err, &srvErr) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}

	return false
}
