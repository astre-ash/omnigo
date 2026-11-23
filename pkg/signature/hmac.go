package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

const HeaderHashSHA256 = "HashSHA256"

type HMACSHA256 struct {
	key []byte
}

func NewHMACSHA256(key string) *HMACSHA256 {
	return &HMACSHA256{
		key: []byte(key),
	}
}

func (h *HMACSHA256) Sign(data []byte) string {
	mac := hmac.New(sha256.New, h.key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *HMACSHA256) Verify(data []byte, signature string) bool {
	gotMAC, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, h.key)
	mac.Write(data)
	expectedMac := mac.Sum(nil)

	return hmac.Equal(gotMAC, expectedMac)

}
