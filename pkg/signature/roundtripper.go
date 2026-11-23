package signature

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

type Signer interface {
	Sign(data []byte) string
}

type SigningRoundTripper struct {
	next   http.RoundTripper
	signer Signer
}

func NewSigningRoundTripper(next http.RoundTripper, signer Signer) *SigningRoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &SigningRoundTripper{
		next:   next,
		signer: signer,
	}
}

func (rt *SigningRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.signer == nil {
		return rt.next.RoundTrip(req)
	}

	clone := req.Clone(req.Context())

	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read request body for signing: %w", err)
		}

		clone.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		clone.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
		clone.ContentLength = int64(len(bodyBytes))
	}

	clone.Header.Set(HeaderHashSHA256, rt.signer.Sign(bodyBytes))

	return rt.next.RoundTrip(clone)
}
