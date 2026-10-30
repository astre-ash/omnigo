package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/astre-ash/omnigo/internal/domain"
)

type MetricSender struct {
	serverAddr string
	client     *http.Client
}

func NewMetricSender(serverAddr string) *MetricSender {
	return &MetricSender{
		serverAddr: serverAddr,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	if _, err := zw.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write gzip data: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize gzip compression: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *MetricSender) Send(m domain.Metrics) error {
	url := fmt.Sprintf("%s/update/", s.serverAddr)

	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}

	compressedData, err := compress(data)
	if err != nil {
		return fmt.Errorf("failed to compress metric payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(compressedData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (s *MetricSender) SendAll(metrics []domain.Metrics) error {
	for _, m := range metrics {
		if err := s.Send(m); err != nil {
			return fmt.Errorf("failed to send metric %s (%s): %w", m.ID, m.MType, err)
		}
	}
	return nil
}
