package clickupwebhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func hmacSHA256Hex(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyXSignature_docExample(t *testing.T) {
	t.Parallel()
	secret := "secret"
	body := `{"webhook_id":"7689a169-a000-4985-8676-6902b96d6627","event":"taskCreated","task_id":"c0j"}`
	mac := hmacSHA256Hex(secret, body)
	if !VerifyXSignature([]byte(body), secret, mac) {
		t.Fatal("expected signature match")
	}
}

func TestVerifyXSignature_rejectsTamperedBody(t *testing.T) {
	t.Parallel()
	secret := "abc"
	body := `{"event":"x"}`
	good := hmacSHA256Hex(secret, body)
	badBody := `{"event":"y"}`
	if VerifyXSignature([]byte(badBody), secret, good) {
		t.Fatal("expected mismatch")
	}
}

func TestVerifyXSignature_emptySecret(t *testing.T) {
	t.Parallel()
	if VerifyXSignature([]byte("{}"), "", "abc") {
		t.Fatal("expected false")
	}
}
