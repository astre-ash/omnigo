package compress

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

const Level = gzip.BestSpeed

type GzipCompressor struct{}

func NewGzipCompressor() *GzipCompressor {
	return &GzipCompressor{}
}

func (g *GzipCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := NewWriter(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}

	if _, err := zw.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write gzip data: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize gzip compression: %w", err)
	}

	return buf.Bytes(), nil
}

func NewWriter(w io.Writer) (*gzip.Writer, error) {
	return gzip.NewWriterLevel(w, Level)
}

func NewReader(r io.Reader) (*gzip.Reader, error) {
	return gzip.NewReader(r)
}
