package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLogger(t *testing.T) {
	core, recordedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	expectedBody := "Hello, metrics!"
	expectedStatus := http.StatusAccepted

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatus)
		_, _ = w.Write([]byte(expectedBody))
	})

	middleware := RequestLogger(logger)
	handlerToTest := middleware(testHandler)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/alloc/123.45", nil)
	rec := httptest.NewRecorder()

	handlerToTest.ServeHTTP(rec, req)

	assert.Equal(t, expectedStatus, rec.Code)
	assert.Equal(t, expectedBody, rec.Body.String())

	logs := recordedLogs.All()
	require.Len(t, logs, 2, "must produce exactly 2 log entries")

	// Request.
	reqLog := logs[0]
	assert.Equal(t, zap.InfoLevel, reqLog.Level)
	assert.Equal(t, "inbound HTTP request", reqLog.Message)

	reqFields := reqLog.ContextMap()
	assert.Equal(t, "/update/gauge/alloc/123.45", reqFields["uri"])
	assert.Equal(t, http.MethodPost, reqFields["method"])
	assert.Contains(t, reqFields, "duration")

	// Response.
	respLog := logs[1]
	assert.Equal(t, zap.InfoLevel, respLog.Level)
	assert.Equal(t, "outbound HTTP response", respLog.Message)

	respFields := respLog.ContextMap()
	assert.Equal(t, int64(expectedStatus), respFields["status"])
	assert.Equal(t, int64(len(expectedBody)), respFields["size"])
}
