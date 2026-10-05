package agent

import (
	"fmt"
	"log"
	"net/http"
	"time"
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

func (s *MetricSender) Send(m MetricData) error {
	url := fmt.Sprintf("%s/update/%s/%s/%s", s.serverAddr, m.Type, m.Name, m.Value)

	req, err := http.NewRequest(http.MethodPost, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (s *MetricSender) SendAll(metrics []MetricData) {
	for _, m := range metrics {
		if err := s.Send(m); err != nil {
			log.Printf("MetricSender: failed to send metric %s (%s): %v", m.Name, m.Type, err)
		}
	}
}
