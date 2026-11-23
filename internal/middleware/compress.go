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
	statusCode  int
	wroteHeader bool
	sentHeader  bool
	compress    bool
}

func newCompressResponseWriter(w http.ResponseWriter) *compressResponseWriter {
	return &compressResponseWriter{
		ResponseWriter: w,
	}
}

func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == http.StatusNoContent:
		return false
	case status == http.StatusNotModified:
		return false
	}
	return true
}

func (c *compressResponseWriter) shouldCompress() bool {
	if c.Header().Get("Content-Encoding") != "" {
		return false
	}

	contentType := c.Header().Get("Content-Type")
	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = strings.TrimSpace(mediaType)

	return strings.EqualFold(mediaType, "application/json") ||
		strings.EqualFold(mediaType, "text/html")
}

func (c *compressResponseWriter) initCompression() {
	if !c.shouldCompress() {
		return
	}

	c.zw = compress.GetWriter(c.ResponseWriter)
	c.compress = true
	c.Header().Set("Content-Encoding", "gzip")
	c.Header().Add("Vary", "Accept-Encoding")
	c.Header().Del("Content-Length")
}

func (c *compressResponseWriter) WriteHeader(statusCode int) {
	if statusCode >= 100 && statusCode <= 199 {
		c.ResponseWriter.WriteHeader(statusCode)
		return
	}

	if c.wroteHeader {
		return
	}
	c.wroteHeader = true
	c.statusCode = statusCode

	if !bodyAllowedForStatus(statusCode) {
		c.sentHeader = true
		c.ResponseWriter.WriteHeader(statusCode)
	}
}

func (c *compressResponseWriter) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}

	if !bodyAllowedForStatus(c.statusCode) {
		return c.ResponseWriter.Write(b)
	}

	if len(b) == 0 && !c.sentHeader {
		return 0, nil
	}

	if !c.sentHeader {
		c.initCompression()
		c.sentHeader = true
		c.ResponseWriter.WriteHeader(c.statusCode)
	}

	if c.compress {
		return c.zw.Write(b)
	}
	return c.ResponseWriter.Write(b)
}

func (c *compressResponseWriter) Flush() {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	if !c.sentHeader {
		if bodyAllowedForStatus(c.statusCode) {
			c.initCompression()
		}
		c.sentHeader = true
		c.ResponseWriter.WriteHeader(c.statusCode)
	}
	if c.compress && c.zw != nil {
		_ = c.zw.Flush()
	}
	_ = http.NewResponseController(c.ResponseWriter).Flush()
}

func (c *compressResponseWriter) Close() error {
	if c.wroteHeader && !c.sentHeader {
		c.sentHeader = true
		c.ResponseWriter.WriteHeader(c.statusCode)
	}

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

			if r.Method == http.MethodHead || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
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
