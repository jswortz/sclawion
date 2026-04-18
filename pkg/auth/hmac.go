// Package auth provides shared signature-verification, OIDC, and replay-cache
// helpers used by all webhook receivers.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"time"
)

// MaxSkew is the timestamp window for accepting webhook requests.
// Requests older or newer than this are rejected to limit replay-attack windows.
const MaxSkew = 5 * time.Minute

var (
	ErrSignatureMismatch = errors.New("auth: signature mismatch")
	ErrTimestampSkew     = errors.New("auth: timestamp outside accepted window")
)

// VerifyHMACSHA256 compares the provided hex-encoded signature against
// HMAC-SHA256(secret, message) in constant time.
func VerifyHMACSHA256(secret, message []byte, expectedHex string) error {
	mac := hmac.New(sha256.New, secret)
	mac.Write(message)
	got := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(got), []byte(expectedHex)) != 1 {
		return ErrSignatureMismatch
	}
	return nil
}

// CheckTimestampSkew returns ErrTimestampSkew if the supplied request time
// differs from now by more than MaxSkew.
func CheckTimestampSkew(reqTime time.Time, now time.Time) error {
	d := now.Sub(reqTime)
	if d < 0 {
		d = -d
	}
	if d > MaxSkew {
		return ErrTimestampSkew
	}
	return nil
}
