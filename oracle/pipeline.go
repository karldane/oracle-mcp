package oracle

import (
	"crypto/hmac"
	"crypto/sha256"
)

func derivePipelineKey(baseKey []byte, sessionID string) []byte {
	if sessionID == "" {
		return baseKey
	}
	h := hmac.New(sha256.New, baseKey)
	h.Write([]byte(sessionID))
	return h.Sum(nil)
}
