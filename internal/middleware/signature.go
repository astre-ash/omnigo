package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"github.com/astre-ash/omnigo/pkg/signature"
)

const defaultMaxSignedBodyBytes int64 = 10 << 20

type SignVerifier interface {
	Sign(data []byte) string
	Verify(data []byte, sig string) bool
}

type signingResponseWriter struct {
	w           http.ResponseWriter
	buf         bytes.Buffer
	statusCode  int
	wroteHeader bool
}

func (s *signingResponseWriter) Header() http.Header {
	return s.w.Header()
}

func (s *signingResponseWriter) WriteHeader(statusCode int) {
	if s.wroteHeader {
		return
	}
	s.wroteHeader = true
	s.statusCode = statusCode
}

func (s *signingResponseWriter) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.WriteHeader(http.StatusOK)
	}
	return s.buf.Write(b)
}

func Signature(sv SignVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if sv == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if gotHash := r.Header.Get(signature.HeaderHashSHA256); gotHash != "" {
				r.Body = http.MaxBytesReader(w, r.Body, defaultMaxSignedBodyBytes)
				bodyBytes, err := io.ReadAll(r.Body)
				_ = r.Body.Close()
				if err != nil {
					var maxErr *http.MaxBytesError
					if errors.As(err, &maxErr) {
						http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
						return
					}
					http.Error(w, "failed to read request body", http.StatusBadRequest)
					return
				}

				if !sv.Verify(bodyBytes, gotHash) {
					http.Error(w, "invalid request signature", http.StatusBadRequest)
					return
				}

				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}

			sw := &signingResponseWriter{w: w}
			next.ServeHTTP(sw, r)

			respBody := sw.buf.Bytes()
			w.Header().Set(signature.HeaderHashSHA256, sv.Sign(respBody))

			status := sw.statusCode
			if status == 0 {
				status = http.StatusOK
			}

			w.WriteHeader(status)
			if len(respBody) > 0 {
				_, _ = w.Write(respBody)
			}
		})
	}
}
