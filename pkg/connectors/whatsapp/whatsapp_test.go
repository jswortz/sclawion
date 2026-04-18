package whatsapp

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
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWhatsAppVerifier_Accepts(t *testing.T) {
	secret := []byte("meta-app-secret")
	body := []byte(`{"object":"whatsapp_business_account"}`)

	r := httptest.NewRequest(http.MethodPost, "/v1/whatsapp", nil)
	r.Header.Set("X-Hub-Signature-256", sign(secret, body))

	if err := (&Verifier{AppSecret: secret}).Verify(context.Background(), r, body); err != nil {
		t.Fatalf("expected valid signature, got %v", err)
	}
}

func TestWhatsAppVerifier_RejectsMissingPrefix(t *testing.T) {
	secret := []byte("s")
	body := []byte("body")
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	r := httptest.NewRequest(http.MethodPost, "/v1/whatsapp", nil)
	r.Header.Set("X-Hub-Signature-256", hex.EncodeToString(mac.Sum(nil))) // no sha256= prefix

	if err := (&Verifier{AppSecret: secret}).Verify(context.Background(), r, body); err == nil {
		t.Fatal("expected error for missing sha256= prefix")
	}
}

func TestWhatsAppVerifier_RejectsTampered(t *testing.T) {
	secret := []byte("s")
	r := httptest.NewRequest(http.MethodPost, "/v1/whatsapp", nil)
	r.Header.Set("X-Hub-Signature-256", sign(secret, []byte("real")))

	err := (&Verifier{AppSecret: secret}).Verify(context.Background(), r, []byte("tampered"))
	if !errors.Is(err, auth.ErrSignatureMismatch) {
		t.Fatalf("expected ErrSignatureMismatch, got %v", err)
	}
}

func TestWhatsAppVerifier_RejectsMissingHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/whatsapp", nil)
	if err := (&Verifier{AppSecret: []byte("s")}).Verify(context.Background(), r, []byte("body")); err == nil {
		t.Fatal("expected error for missing header")
	}
}
