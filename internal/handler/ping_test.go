package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astre-ash/omnigo/internal/handler"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

func TestPingHandler_Ping(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		pinger         handler.Pinger
		expectedStatus int
	}{
		{
			name:           "success ping",
			pinger:         &mockPinger{err: nil},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "failed ping returns 500",
			pinger:         &mockPinger{err: errors.New("db connection lost")},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "nil pinger returns 500",
			pinger:         nil,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := handler.NewPingHandler(tt.pinger, logger)

			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			rec := httptest.NewRecorder()

			h.Ping(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}
