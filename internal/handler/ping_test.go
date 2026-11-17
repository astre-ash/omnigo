package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/astre-ash/omnigo/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockPinger struct {
	pingFn func(ctx context.Context) error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}

func TestPingHandler_Ping(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		pinger         handler.Pinger
		logger         *zap.Logger
		opts           []handler.PingOption
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "success ping",
			pinger:         &mockPinger{},
			logger:         logger,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "success with NoopPinger",
			pinger:         handler.NoopPinger{},
			logger:         logger,
			expectedStatus: http.StatusOK,
		},
		{
			name: "failed ping returns 500",
			pinger: &mockPinger{
				pingFn: func(context.Context) error {
					return errors.New("db connection lost")
				},
			},
			logger:         logger,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   http.StatusText(http.StatusInternalServerError),
		},
		{
			name:           "nil pinger returns 500",
			pinger:         nil,
			logger:         logger,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   http.StatusText(http.StatusInternalServerError),
		},
		{
			name:           "nil logger does not panic",
			pinger:         nil,
			logger:         nil,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   http.StatusText(http.StatusInternalServerError),
		},
		{
			name:           "nil option is ignored",
			pinger:         &mockPinger{},
			logger:         logger,
			opts:           []handler.PingOption{nil},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := handler.NewPingHandler(tt.pinger, tt.logger, tt.opts...)

			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			rec := httptest.NewRecorder()

			h.Ping(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rec.Body.String(), tt.expectedBody)
			} else {
				assert.Empty(t, rec.Body.String())
			}
		})
	}
}

func TestPingHandler_ZeroValueStruct(t *testing.T) {
	h := &handler.PingHandler{}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		h.Ping(rec, req)
	})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestPingHandler_Timeout(t *testing.T) {
	t.Run("uses default timeout when option is not provided or non-positive", func(t *testing.T) {
		var capturedDeadline time.Time
		var hasDeadline bool

		pinger := &mockPinger{
			pingFn: func(ctx context.Context) error {
				capturedDeadline, hasDeadline = ctx.Deadline()
				return nil
			},
		}

		h := handler.NewPingHandler(
			pinger,
			zap.NewNop(),
			handler.WithPingTimeout(0),
			handler.WithPingTimeout(-time.Second),
		)

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		rec := httptest.NewRecorder()

		h.Ping(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.True(t, hasDeadline)

		remaining := time.Until(capturedDeadline)
		assert.Greater(t, remaining, time.Second)
		assert.LessOrEqual(t, remaining, 2*time.Second)
	})

	t.Run("cancels ping when custom timeout expires", func(t *testing.T) {
		pinger := &mockPinger{
			pingFn: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			},
		}

		h := handler.NewPingHandler(
			pinger,
			zap.NewNop(),
			handler.WithPingTimeout(20*time.Millisecond),
		)

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		rec := httptest.NewRecorder()

		h.Ping(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), http.StatusText(http.StatusInternalServerError))
	})
}
