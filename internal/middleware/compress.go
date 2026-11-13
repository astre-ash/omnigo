package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/astre-ash/omnigo/pkg/compress"
)

type gzipReadCloser struct {
	*gzip.Reader
	orig io.ReadCloser
	once sync.Once
}

func (gz *gzipReadCloser) Close() error {
	var err error
	gz.once.Do(func() {
		err = gz.Reader.Close()
		compress.PutReader(gz.Reader)
		_ = gz.orig.Close()
	})
	return err
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
		c.zw = compress.GetWriter(c.ResponseWriter)
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
	if c.compress && c.zw != nil {
		err := c.zw.Close()
		compress.PutWriter(c.zw)
		c.zw = nil
		return err
	}
	return nil
}

func Compress() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
				gzReader, err := compress.GetReader(r.Body)
				if err != nil {
					http.Error(w, "failed to decode gzip body", http.StatusBadRequest)
					return
				}
				r.Body = &gzipReadCloser{
					Reader: gzReader,
					orig:   r.Body,
				}
			}

			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
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
