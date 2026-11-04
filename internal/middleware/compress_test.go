package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gzipCompress(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(data)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func gzipDecompress(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer zr.Close()
	decompressed, err := io.ReadAll(zr)
	require.NoError(t, err)
	return decompressed
}

func TestCompressMiddleware(t *testing.T) {
	t.Run("decompresses inbound request body with gzip", func(t *testing.T) {
		expectedBody := `{"id":"alloc","type":"gauge","value":42.5}`
		compressedBody := gzipCompress(t, []byte(expectedBody))

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.Equal(t, expectedBody, string(body))
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(compressedBody))
		req.Header.Set("Content-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("passes through uncompressed inbound request body unchanged", func(t *testing.T) {
		expectedBody := `{"id":"poll_count",
		"type":"counter",
		"delta":10,
		}`

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.Equal(t, expectedBody, string(body))
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(expectedBody)))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("compresses response if client accepts gzip and content-type is application/json", func(t *testing.T) {
		jsonResponse := `{"status":"ok"}`

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(jsonResponse))
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

		decompressed := gzipDecompress(t, rec.Body.Bytes())
		assert.Equal(t, jsonResponse, string(decompressed))
	})

	t.Run("compresses response if client accepts gzip and content-type is text/html", func(t *testing.T) {
		htmlResponse := `<html><body><h1>Metrics</h1></body></html>`

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(htmlResponse))
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

		decompressed := gzipDecompress(t, rec.Body.Bytes())
		assert.Equal(t, htmlResponse, string(decompressed))
	})

	t.Run("compresses response when content-type has charset parameter", func(t *testing.T) {
		jsonResponse := `{"metric":"Alloc"}`

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write([]byte(jsonResponse))
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

		decompressed := gzipDecompress(t, rec.Body.Bytes())
		assert.Equal(t, jsonResponse, string(decompressed))
	})

	t.Run("compresses response when Accept-Encoding contains multiple encodings", func(t *testing.T) {
		jsonResponse := `{"status":"ok"}`

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(jsonResponse))
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "deflate, gzip, br")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	})

	t.Run("does not compress response if client does not accept gzip", func(t *testing.T) {
		jsonResponse := `{"status":"ok"}`

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(jsonResponse))
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)

		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Get("Content-Encoding"))
		assert.Equal(t, jsonResponse, rec.Body.String())
	})

	t.Run("does not compress response if content-type is text/plain", func(t *testing.T) {
		plainResponse := "plain text data"

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(plainResponse))
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Get("Content-Encoding"))
		assert.Equal(t, plainResponse, rec.Body.String())
	})

	t.Run("preserves non-200 HTTP status code when compressing response", func(t *testing.T) {
		errorResponse := `{"error":"not found"}`

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(errorResponse))
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

		decompressed := gzipDecompress(t, rec.Body.Bytes())
		assert.Equal(t, errorResponse, string(decompressed))
	})
}
