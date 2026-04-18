package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

func TestVerifyHMACSHA256_AcceptsValid(t *testing.T) {
	secret := []byte("super-secret")
	body := []byte("v0:1700000000:hello world")

	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if err := VerifyHMACSHA256(secret, body, sig); err != nil {
		t.Fatalf("expected valid signature to verify, got %v", err)
	}
}

func TestVerifyHMACSHA256_RejectsTamperedBody(t *testing.T) {
	secret := []byte("super-secret")
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("original"))
	sig := hex.EncodeToString(mac.Sum(nil))

	if err := VerifyHMACSHA256(secret, []byte("tampered"), sig); !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("expected ErrSignatureMismatch, got %v", err)
	}
}

func TestVerifyHMACSHA256_RejectsBadSecret(t *testing.T) {
	mac := hmac.New(sha256.New, []byte("real"))
	mac.Write([]byte("body"))
	sig := hex.EncodeToString(mac.Sum(nil))

	if err := VerifyHMACSHA256([]byte("wrong"), []byte("body"), sig); !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("expected ErrSignatureMismatch, got %v", err)
	}
}

func TestCheckTimestampSkew(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		req  time.Time
		want error
	}{
		{"now", now, nil},
		{"4m past", now.Add(-4 * time.Minute), nil},
		{"4m future", now.Add(4 * time.Minute), nil},
		{"6m past", now.Add(-6 * time.Minute), ErrTimestampSkew},
		{"6m future", now.Add(6 * time.Minute), ErrTimestampSkew},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CheckTimestampSkew(c.req, now)
			if !errors.Is(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
