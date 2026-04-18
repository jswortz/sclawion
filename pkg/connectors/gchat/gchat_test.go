package gchat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubOIDC satisfies auth.OIDCVerifier. It accepts a fixed (token, audience)
// pair and rejects everything else, so we can prove gchat's Verifier passes
// the right values through and surfaces the OIDC error verbatim.
type stubOIDC struct{ wantTok, wantAud string }

func (s stubOIDC) Verify(_ context.Context, token, aud string) (string, error) {
	if token != s.wantTok || aud != s.wantAud {
		return "", errors.New("oidc: bad token/aud")
	}
	return "chat-events@google.com", nil
}

func TestGChatVerifier_AcceptsValidBearer(t *testing.T) {
	v := &Verifier{
		OIDC:     stubOIDC{wantTok: "good.jwt", wantAud: "projects/123"},
		Audience: "projects/123",
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/gchat", nil)
	r.Header.Set("Authorization", "Bearer good.jwt")

	if err := v.Verify(context.Background(), r, []byte(`{}`)); err != nil {
		t.Fatalf("expected accept, got %v", err)
	}
}

func TestGChatVerifier_RejectsMissingBearer(t *testing.T) {
	v := &Verifier{OIDC: stubOIDC{}, Audience: "x"}
	r := httptest.NewRequest(http.MethodPost, "/v1/gchat", nil)
	if err := v.Verify(context.Background(), r, []byte("body")); err == nil {
		t.Fatal("expected error for missing Authorization")
	}
}

func TestGChatVerifier_RejectsBadToken(t *testing.T) {
	v := &Verifier{
		OIDC:     stubOIDC{wantTok: "real", wantAud: "aud"},
		Audience: "aud",
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/gchat", nil)
	r.Header.Set("Authorization", "Bearer forged")
	if err := v.Verify(context.Background(), r, []byte("body")); err == nil {
		t.Fatal("expected error for forged token")
	}
}

func TestGChatVerifier_RejectsWrongAudience(t *testing.T) {
	v := &Verifier{
		OIDC:     stubOIDC{wantTok: "tok", wantAud: "projects/123"},
		Audience: "projects/999", // mismatched audience
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/gchat", nil)
	r.Header.Set("Authorization", "Bearer tok")
	if err := v.Verify(context.Background(), r, []byte("body")); err == nil {
		t.Fatal("expected error for wrong audience")
	}
}
