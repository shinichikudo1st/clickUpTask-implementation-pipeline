package clickupwebhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// VerifyXSignature checks the ClickUp webhook HMAC-SHA256 signature (hex) in
// X-Signature against the raw request body. Body bytes must be exactly as received.
func VerifyXSignature(rawBody []byte, secret string, xSignature string) bool {
	secret = strings.TrimSpace(secret)
	xSignature = strings.TrimSpace(xSignature)
	if secret == "" || xSignature == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))

	// Constant-time compare on equal-length hex strings.
	if len(expected) != len(xSignature) {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(strings.ToLower(xSignature)))
}
