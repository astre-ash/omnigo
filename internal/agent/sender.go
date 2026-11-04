package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	return &MetricSender{
		serverAddr: serverAddr,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		compressor: compressor,
	}
}

func (s *MetricSender) Send(m domain.Metrics) error {
	url := fmt.Sprintf("%s/update/", s.serverAddr)

	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}

	var body io.Reader
	isCompessed := false

	if s.compressor != nil {
		compressedData, err := s.compressor.Compress(data)
		if err != nil {
			return fmt.Errorf("failed to compress metric payload: %w", err)
		}
		body = bytes.NewReader(compressedData)
		isCompessed = true
	} else {
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")

	if isCompessed {
		req.Header.Set("Content-Encoding", "gzip")
	}

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
