package retry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

const defaultClientTimeout = 5 * time.Second

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type HTTPClient struct {
	client HTTPDoer
	delays []time.Duration
	logger *zap.Logger
}

func NewHTTPClient(client HTTPDoer, delays []time.Duration, logger *zap.Logger) *HTTPClient {
	if client == nil {
		client = &http.Client{Timeout: defaultClientTimeout}
	}

	if delays == nil {
		delays = DefaultDelays()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &HTTPClient{
		client: client,
		delays: delays,
		logger: logger,
	}
}

type httpStatusError struct {
	statusCode int
	body       string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("server returned status %d: %s", e.statusCode, e.body)
}

func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("request must not be nil")
	}

	canRetryBody := req.Body == nil || req.GetBody != nil

	var (
		resp     *http.Response
		lastResp *http.Response
		attempt  int
	)

	err := Do(req.Context(), c.delays, func(err error) bool {
		return canRetryBody && c.isRetryable(err)
	}, func() error {
		attemptReq := req.Clone(req.Context())
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return fmt.Errorf("failed to reset request body: %w", err)
			}
			attemptReq.Body = body
		}
		attempt++

		r, err := c.client.Do(attemptReq)
		if err != nil {
			return err
		}

		if r.StatusCode >= http.StatusInternalServerError {
			respBody, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
			_, _ = io.Copy(io.Discard, r.Body)
			_ = r.Body.Close()

			r.Body = io.NopCloser(bytes.NewReader(respBody))
			lastResp = r

			return &httpStatusError{
				statusCode: r.StatusCode,
				body:       strings.TrimSpace(string(respBody)),
			}
		}

		resp = r
		return nil
	})

	if err == nil {
		return resp, nil
	}

	var statusErr *httpStatusError
	if errors.As(err, &statusErr) && lastResp != nil {
		return lastResp, nil
	}

	return nil, err
}

func (c *HTTPClient) isRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}

	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		c.logger.Warn("retrying http request after server error",
			zap.Int("status", statusErr.statusCode),
		)
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		c.logger.Warn("retrying http request after network error", zap.Error(err))
		return true
	}

	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, io.EOF) {
		c.logger.Warn("retrying http request after socket error", zap.Error(err))
		return true
	}

	return false
}
