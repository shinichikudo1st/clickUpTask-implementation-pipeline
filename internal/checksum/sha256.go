package checksum

import (
	"crypto/sha256"
	"encoding/hex"
)

// Of returns the hex-encoded SHA-256 of b.
func Of(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
