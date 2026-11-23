package middleware

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
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

type trackingReadCloser struct {
	io.Reader
	closeCount int
}

func (t *trackingReadCloser) Close() error {
	t.closeCount++
	return nil
}

type statusSpyResponseWriter struct {
	header   http.Header
	statuses []int
	body     bytes.Buffer
}

func (s *statusSpyResponseWriter) Header() http.Header {
	if s.header == nil {
		s.header = make(http.Header)
	}
	return s.header
}

func (s *statusSpyResponseWriter) WriteHeader(statusCode int) {
	s.statuses = append(s.statuses, statusCode)
}

func (s *statusSpyResponseWriter) Write(b []byte) (int, error) {
	return s.body.Write(b)
}

func TestCompressMiddleware(t *testing.T) {
	t.Run("decompresses inbound request body with gzip", func(t *testing.T) {
		expectedBody := `{"id":"alloc","type":"gauge","value":42.5}`
		compressedBody := gzipCompress(t, []byte(expectedBody))

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
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

	t.Run("returns 400 Bad Request and skips next handler on invalid gzip body", func(t *testing.T) {
		nextCalled := false
		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not a valid gzip payload")))
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.False(t, nextCalled, "next handler must not be called on invalid gzip body")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Empty(t, rec.Header().Get("Content-Encoding"), "400 error response should not be gzipped")
		assert.Contains(t, rec.Body.String(), "failed to decode gzip body")
	})

	t.Run("gzipReadCloser closes original body once even on multiple Close calls", func(t *testing.T) {
		compressedBody := gzipCompress(t, []byte("payload"))
		origBody := &trackingReadCloser{Reader: bytes.NewReader(compressedBody)}

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, r.Body.Close())
			require.NoError(t, r.Body.Close())
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodPost, "/", origBody)
		req.Header.Set("Content-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, 1, origBody.closeCount, "original request body must be closed exactly once")
	})

	t.Run("passes through uncompressed inbound request body unchanged", func(t *testing.T) {
		expectedBody := `{"id":"poll_count","type":"counter","delta":10}`

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

	t.Run("compresses response for supported content types when client accepts gzip", func(t *testing.T) {
		tests := []struct {
			name        string
			contentType string
			body        string
		}{
			{
				name:        "application/json",
				contentType: "application/json",
				body:        `{"status":"ok"}`,
			},
			{
				name:        "application/json with charset",
				contentType: "application/json; charset=utf-8",
				body:        `{"metric":"Alloc"}`,
			},
			{
				name:        "case-insensitive application/json",
				contentType: "Application/JSON ; charset=utf-8",
				body:        `{"case":"insensitive"}`,
			},
			{
				name:        "text/html",
				contentType: "text/html",
				body:        `<html><body><h1>Metrics</h1></body></html>`,
			},
			{
				name:        "case-insensitive text/html",
				contentType: "TEXT/HTML; charset=utf-8",
				body:        `<html><body>OK</body></html>`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", tt.contentType)
					_, err := w.Write([]byte(tt.body))
					require.NoError(t, err)
				}))

				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("Accept-Encoding", "gzip")
				rec := httptest.NewRecorder()

				handler.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
				assert.Contains(t, rec.Header().Values("Vary"), "Accept-Encoding")

				decompressed := gzipDecompress(t, rec.Body.Bytes())
				assert.Equal(t, tt.body, string(decompressed))
			})
		}
	})

	t.Run("removes Content-Length header only when compressing response", func(t *testing.T) {
		t.Run("compressed response deletes Content-Length", func(t *testing.T) {
			jsonResponse := `{"metric":"Alloc","val":123}`

			handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Length", "100")
				_, _ = w.Write([]byte(jsonResponse))
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
			assert.Empty(t, rec.Header().Get("Content-Length"), "Content-Length must be deleted on compression")
		})

		t.Run("uncompressed response preserves Content-Length", func(t *testing.T) {
			plainResponse := "plain"

			handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Header().Set("Content-Length", "5")
				_, _ = w.Write([]byte(plainResponse))
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Empty(t, rec.Header().Get("Content-Encoding"))
			assert.Equal(t, "5", rec.Header().Get("Content-Length"))
			assert.Equal(t, plainResponse, rec.Body.String())
		})
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
		assert.Equal(t, jsonResponse, string(gzipDecompress(t, rec.Body.Bytes())))
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
		assert.Empty(t, rec.Header().Get("Vary"))
		assert.Equal(t, jsonResponse, rec.Body.String())
	})

	t.Run("does not compress response for HEAD requests", func(t *testing.T) {
		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", "15")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodHead, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Get("Content-Encoding"))
		assert.Empty(t, rec.Header().Get("Vary"))
		assert.Equal(t, "15", rec.Header().Get("Content-Length"))
		assert.Empty(t, rec.Body.Bytes())
	})

	t.Run("does not compress response for unsupported content types", func(t *testing.T) {
		for _, contentType := range []string{"text/plain", ""} {
			plainResponse := "plain text data"

			handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if contentType != "" {
					w.Header().Set("Content-Type", contentType)
				}
				_, _ = w.Write([]byte(plainResponse))
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Empty(t, rec.Header().Get("Content-Encoding"))
			assert.Empty(t, rec.Header().Get("Vary"))
			assert.Equal(t, plainResponse, rec.Body.String())
		}
	})

	t.Run("does not compress response when Content-Encoding is already set", func(t *testing.T) {
		payload := "already-encoded-data"

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Encoding", "br")
			_, err := w.Write([]byte(payload))
			require.NoError(t, err)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "br", rec.Header().Get("Content-Encoding"))
		assert.Empty(t, rec.Header().Get("Vary"))
		assert.Equal(t, payload, rec.Body.String())
	})

	t.Run("preserves non-200 HTTP status code and ignores duplicate WriteHeader calls", func(t *testing.T) {
		errorResponse := `{"error":"not found"}`

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.WriteHeader(http.StatusInternalServerError)
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

	t.Run("does not compress responses with status codes that forbid a body", func(t *testing.T) {
		for _, status := range []int{http.StatusNoContent, http.StatusNotModified} {
			t.Run(http.StatusText(status), func(t *testing.T) {
				handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
				}))

				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("Accept-Encoding", "gzip")
				rec := httptest.NewRecorder()

				handler.ServeHTTP(rec, req)

				assert.Equal(t, status, rec.Code)
				assert.Empty(t, rec.Header().Get("Content-Encoding"))
				assert.Empty(t, rec.Header().Get("Vary"))
				assert.Empty(t, rec.Body.Bytes())
			})
		}
	})

	t.Run("forwards 1xx informational statuses without preventing subsequent compression", func(t *testing.T) {
		spy := &statusSpyResponseWriter{}

		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusEarlyHints)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"early":"hints"}`))
			require.NoError(t, err)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")

		handler.ServeHTTP(spy, req)

		assert.Equal(t, []int{http.StatusEarlyHints, http.StatusOK}, spy.statuses)
		assert.Equal(t, "gzip", spy.Header().Get("Content-Encoding"))
		assert.Equal(t, `{"early":"hints"}`, string(gzipDecompress(t, spy.body.Bytes())))
	})

	t.Run("does not compress empty response when WriteHeader is called without Write", func(t *testing.T) {
		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
		}))

		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Empty(t, rec.Header().Get("Content-Encoding"))
		assert.Empty(t, rec.Header().Get("Vary"))
		assert.Empty(t, rec.Body.Bytes())
	})

	t.Run("ignores empty Write before headers are sent", func(t *testing.T) {
		t.Run("only empty Write does not trigger compression", func(t *testing.T) {
			handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				n, err := w.Write([]byte{})
				require.NoError(t, err)
				assert.Equal(t, 0, n)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Empty(t, rec.Header().Get("Content-Encoding"))
			assert.Empty(t, rec.Body.Bytes())
		})

		t.Run("empty Write followed by non-empty Write compresses normally", func(t *testing.T) {
			handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte{})
				require.NoError(t, err)
				_, err = w.Write([]byte(`{"ok":true}`))
				require.NoError(t, err)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
			assert.Equal(t, `{"ok":true}`, string(gzipDecompress(t, rec.Body.Bytes())))
		})
	})

	t.Run("Flush initializes compression and flushes underlying ResponseWriter", func(t *testing.T) {
		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"chunk":1}`))
			require.NoError(t, err)

			rc := http.NewResponseController(w)
			require.NoError(t, rc.Flush())

			_, err = w.Write([]byte(`{"chunk":2}`))
			require.NoError(t, err)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, rec.Flushed)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
		assert.Equal(t, `{"chunk":1}{"chunk":2}`, string(gzipDecompress(t, rec.Body.Bytes())))
	})

	t.Run("Flush without Write still initializes headers and flushes empty gzip stream", func(t *testing.T) {
		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			rc := http.NewResponseController(w)
			require.NoError(t, rc.Flush())
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, rec.Flushed)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
		assert.Empty(t, gzipDecompress(t, rec.Body.Bytes()))
	})

	t.Run("combines multiple Write calls into a single valid gzip stream", func(t *testing.T) {
		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"part1":"hello",`))
			require.NoError(t, err)
			_, err = w.Write([]byte(`"part2":"world"}`))
			require.NoError(t, err)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
		assert.Equal(t, `{"part1":"hello","part2":"world"}`, string(gzipDecompress(t, rec.Body.Bytes())))
	})

	t.Run("handles bidirectional gzip concurrently and reuses pool objects safely", func(t *testing.T) {
		handler := Compress()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			inbound, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, err = w.Write(append([]byte("echo: "), inbound...))
			require.NoError(t, err)
		}))

		const workers = 10
		var wg sync.WaitGroup
		wg.Add(workers)

		for i := 0; i < workers; i++ {
			go func(idx int) {
				defer wg.Done()

				reqBody := fmt.Sprintf(`{"index":%d}`, idx)
				compressedReq := gzipCompress(t, []byte(reqBody))

				req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(compressedReq))
				req.Header.Set("Content-Encoding", "gzip")
				req.Header.Set("Accept-Encoding", "gzip")
				rec := httptest.NewRecorder()

				handler.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusCreated, rec.Code)
				assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

				decompressed := gzipDecompress(t, rec.Body.Bytes())
				assert.Equal(t, "echo: "+reqBody, string(decompressed))
			}(i)
		}

		wg.Wait()
	})
}

func TestCompressResponseWriter_CloseIsIdempotent(t *testing.T) {
	rec := httptest.NewRecorder()
	cw := newCompressResponseWriter(rec)
	cw.Header().Set("Content-Type", "application/json")

	_, err := cw.Write([]byte(`{"ok":true}`))
	require.NoError(t, err)

	require.NoError(t, cw.Close())
	require.NoError(t, cw.Close())

	assert.Equal(t, `{"ok":true}`, string(gzipDecompress(t, rec.Body.Bytes())))
}
