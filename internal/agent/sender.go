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

func (s *MetricSender) Send(m domain.Metrics) error {
	url := fmt.Sprintf("%s/update/", s.serverAddr)
	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
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
