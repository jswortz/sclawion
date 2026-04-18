package discord

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return pub, priv
}

func sign(t *testing.T, priv ed25519.PrivateKey, ts string, body []byte) string {
	t.Helper()
	msg := append([]byte(ts), body...)
	return hex.EncodeToString(ed25519.Sign(priv, msg))
}

func TestDiscordVerifier_Accepts(t *testing.T) {
	pub, priv := newPair(t)
	body := []byte(`{"type":1}`)
	ts := "1700000000"
	v := &Verifier{PublicKey: pub}

	r := httptest.NewRequest(http.MethodPost, "/v1/discord", nil)
	r.Header.Set("X-Signature-Ed25519", sign(t, priv, ts, body))
	r.Header.Set("X-Signature-Timestamp", ts)

	if err := v.Verify(context.Background(), r, body); err != nil {
		t.Fatalf("expected valid signature, got %v", err)
	}
}

func TestDiscordVerifier_RejectsTampered(t *testing.T) {
	pub, priv := newPair(t)
	v := &Verifier{PublicKey: pub}
	ts := "1700000000"

	r := httptest.NewRequest(http.MethodPost, "/v1/discord", nil)
	r.Header.Set("X-Signature-Ed25519", sign(t, priv, ts, []byte("real")))
	r.Header.Set("X-Signature-Timestamp", ts)

	if err := v.Verify(context.Background(), r, []byte("tampered")); err == nil {
		t.Fatal("expected error for tampered body")
	}
}

func TestDiscordVerifier_RejectsWrongKey(t *testing.T) {
	pub, _ := newPair(t)
	_, priv2 := newPair(t)
	v := &Verifier{PublicKey: pub}
	ts := "1700000000"
	body := []byte("body")

	r := httptest.NewRequest(http.MethodPost, "/v1/discord", nil)
	r.Header.Set("X-Signature-Ed25519", sign(t, priv2, ts, body))
	r.Header.Set("X-Signature-Timestamp", ts)

	if err := v.Verify(context.Background(), r, body); err == nil {
		t.Fatal("expected error for foreign key")
	}
}

func TestDiscordVerifier_RejectsMissingHeaders(t *testing.T) {
	pub, _ := newPair(t)
	v := &Verifier{PublicKey: pub}
	r := httptest.NewRequest(http.MethodPost, "/v1/discord", nil)
	if err := v.Verify(context.Background(), r, []byte("body")); err == nil {
		t.Fatal("expected error for missing headers")
	}
}

func TestDiscordVerifier_RejectsMalformedHex(t *testing.T) {
	pub, _ := newPair(t)
	v := &Verifier{PublicKey: pub}
	r := httptest.NewRequest(http.MethodPost, "/v1/discord", nil)
	r.Header.Set("X-Signature-Ed25519", "not-hex-zz")
	r.Header.Set("X-Signature-Timestamp", "1700000000")
	if err := v.Verify(context.Background(), r, []byte("body")); err == nil {
		t.Fatal("expected error for malformed hex")
	}
}
