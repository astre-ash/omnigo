package compress

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"sync"
)

const Level = gzip.BestSpeed

var writerPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, Level)
		return w
	},
}

var readerPool sync.Pool

func GetWriter(w io.Writer) *gzip.Writer {
	zw := writerPool.Get().(*gzip.Writer)
	zw.Reset(w)
	return zw
}

func PutWriter(zw *gzip.Writer) {
	zw.Reset(io.Discard)
	writerPool.Put(zw)
}

func GetReader(r io.Reader) (*gzip.Reader, error) {
	if v := readerPool.Get(); v != nil {
		zr := v.(*gzip.Reader)
		if err := zr.Reset(r); err != nil {
			return nil, err
		}
		return zr, nil
	}
	return gzip.NewReader(r)
}

func PutReader(zr *gzip.Reader) {
	_ = zr.Close()
	readerPool.Put(zr)
}

type GzipCompressor struct{}

func NewGzipCompressor() *GzipCompressor {
	return &GzipCompressor{}
}

func (g *GzipCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	zw := GetWriter(&buf)
	defer PutWriter(zw)

	if _, err := zw.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write gzip data: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize gzip compression: %w", err)
	}

	return buf.Bytes(), nil
}
