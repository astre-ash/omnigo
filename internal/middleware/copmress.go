package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type gzipReadCloser struct {
	*gzip.Reader
	orig io.ReadCloser
}

func (gz *gzipReadCloser) Close() error {
	if err := gz.Reader.Close(); err != nil {
		_ = gz.orig.Close()
		return err
	}
	return gz.orig.Close()
}

type compressResponseWriter struct {
	http.ResponseWriter
	zw          *gzip.Writer
	compress    bool
	wroteHeader bool
}

func newCompressResponseWriter(w http.ResponseWriter) *compressResponseWriter {
	return &compressResponseWriter{
		ResponseWriter: w,
		zw:             gzip.NewWriter(w),
	}
}

func (c *compressResponseWriter) shouldCompress() bool {
	contentType := c.Header().Get("Content-Type")
	return strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/html")
}

func (c *compressResponseWriter) WriteHeader(statusCode int) {
	if c.wroteHeader {
		return
	}
	c.wroteHeader = true

	if c.shouldCompress() {
		c.compress = true
		c.Header().Set("Content-Encoding", "gzip")
		c.Header().Del("Content-Length")
	}

	c.ResponseWriter.WriteHeader(statusCode)
}

func (c *compressResponseWriter) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}

	if c.compress {
		return c.zw.Write(b)
	}
	return c.ResponseWriter.Write(b)
}

func (c *compressResponseWriter) Close() error {
	if c.compress {
		return c.zw.Close()
	}
	return nil
}

func Compress() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
				gzReader, err := gzip.NewReader(r.Body)
				if err != nil {
					http.Error(w, "failed to decode gzip body", http.StatusBadRequest)
					return
				}
				r.Body = &gzipReadCloser{Reader: gzReader, orig: r.Body}
			}

			acceptsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
			if !acceptsGzip {
				next.ServeHTTP(w, r)
				return
			}

			cw := newCompressResponseWriter(w)
			defer func() {
				_ = cw.Close()
			}()

			next.ServeHTTP(cw, r)
		})
	}
}
