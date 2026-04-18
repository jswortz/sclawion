package imessage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jswortz/sclawion/pkg/auth"
)

func sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestIMessageVerifier_AcceptsSendblueDefault(t *testing.T) {
	secret := []byte("sb-secret")
	body := []byte(`{"content":"hi","from_number":"+15551234567"}`)

	r := httptest.NewRequest(http.MethodPost, "/v1/imessage", nil)
	r.Header.Set("sb-signature", sign(secret, body))

	if err := (&Verifier{SigningSecret: secret}).Verify(context.Background(), r, body); err != nil {
		t.Fatalf("expected valid signature, got %v", err)
	}
}

func TestIMessageVerifier_AcceptsBlueBubblesHeader(t *testing.T) {
	secret := []byte("bb-secret")
	body := []byte(`{"event":"new-message"}`)

	r := httptest.NewRequest(http.MethodPost, "/v1/imessage", nil)
	r.Header.Set("X-BlueBubbles-Signature", sign(secret, body))

	v := &Verifier{SigningSecret: secret, Header: "X-BlueBubbles-Signature"}
	if err := v.Verify(context.Background(), r, body); err != nil {
		t.Fatalf("expected valid signature for BlueBubbles header, got %v", err)
	}
}

func TestIMessageVerifier_RejectsTampered(t *testing.T) {
	secret := []byte("sb-secret")
	r := httptest.NewRequest(http.MethodPost, "/v1/imessage", nil)
	r.Header.Set("sb-signature", sign(secret, []byte("real")))

	err := (&Verifier{SigningSecret: secret}).Verify(context.Background(), r, []byte("tampered"))
	if !errors.Is(err, auth.ErrSignatureMismatch) {
		t.Fatalf("expected ErrSignatureMismatch, got %v", err)
	}
}

func TestIMessageVerifier_RejectsMissingHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/imessage", nil)
	if err := (&Verifier{SigningSecret: []byte("s")}).Verify(context.Background(), r, []byte("body")); err == nil {
		t.Fatal("expected error for missing sb-signature")
	}
}

func TestIMessageVerifier_HeaderOverrideRespected(t *testing.T) {
	// If we configure Header=X-BlueBubbles-Signature but the bridge sends
	// only sb-signature, we must reject (operator misconfig).
	secret := []byte("s")
	body := []byte("body")
	r := httptest.NewRequest(http.MethodPost, "/v1/imessage", nil)
	r.Header.Set("sb-signature", sign(secret, body))

	v := &Verifier{SigningSecret: secret, Header: "X-BlueBubbles-Signature"}
	if err := v.Verify(context.Background(), r, body); err == nil {
		t.Fatal("expected error: header override should ignore sb-signature")
	}
}
