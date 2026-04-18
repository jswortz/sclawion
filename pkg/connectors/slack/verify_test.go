package slack

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/jswortz/sclawion/pkg/auth"
)

// signSlack returns ("v0=" + hex(HMAC-SHA256(secret, "v0:" + ts + ":" + body))).
func signSlack(secret, body []byte, ts int64) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("v0:" + strconv.FormatInt(ts, 10) + ":"))
	mac.Write(body)
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func newReq(t *testing.T, sig, ts string, body []byte) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/v1/slack", nil)
	r.Header.Set("X-Slack-Signature", sig)
	r.Header.Set("X-Slack-Request-Timestamp", ts)
	return r
}

func TestSlackVerifier_Accepts(t *testing.T) {
	secret := []byte("8f3c-test")
	body := []byte(`{"event":{"type":"message","text":"hi"}}`)
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	ts := now.Unix()

	v := &Verifier{SigningSecret: secret, Now: func() time.Time { return now }}
	r := newReq(t, signSlack(secret, body, ts), strconv.FormatInt(ts, 10), body)

	if err := v.Verify(context.Background(), r, body); err != nil {
		t.Fatalf("expected valid request, got %v", err)
	}
}

func TestSlackVerifier_RejectsTamperedBody(t *testing.T) {
	secret := []byte("8f3c-test")
	body := []byte(`{"text":"original"}`)
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	ts := now.Unix()
	v := &Verifier{SigningSecret: secret, Now: func() time.Time { return now }}

	r := newReq(t, signSlack(secret, body, ts), strconv.FormatInt(ts, 10), body)
	if err := v.Verify(context.Background(), r, []byte(`{"text":"tampered"}`)); !errors.Is(err, auth.ErrSignatureMismatch) {
		t.Fatalf("expected ErrSignatureMismatch, got %v", err)
	}
}

func TestSlackVerifier_RejectsStaleTimestamp(t *testing.T) {
	secret := []byte("8f3c-test")
	body := []byte(`{"text":"hi"}`)
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-10 * time.Minute).Unix()
	v := &Verifier{SigningSecret: secret, Now: func() time.Time { return now }}

	r := newReq(t, signSlack(secret, body, stale), strconv.FormatInt(stale, 10), body)
	if err := v.Verify(context.Background(), r, body); !errors.Is(err, auth.ErrTimestampSkew) {
		t.Fatalf("expected ErrTimestampSkew, got %v", err)
	}
}

func TestSlackVerifier_RejectsMissingHeaders(t *testing.T) {
	v := &Verifier{SigningSecret: []byte("x")}
	r := httptest.NewRequest(http.MethodPost, "/v1/slack", nil)
	if err := v.Verify(context.Background(), r, []byte("body")); err == nil {
		t.Fatal("expected error for missing headers")
	}
}

func TestSlackVerifier_RejectsMissingV0Prefix(t *testing.T) {
	secret := []byte("x")
	body := []byte("body")
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	ts := strconv.FormatInt(now.Unix(), 10)
	v := &Verifier{SigningSecret: secret, Now: func() time.Time { return now }}

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("v0:" + ts + ":"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil)) // no v0= prefix

	r := newReq(t, sig, ts, body)
	if err := v.Verify(context.Background(), r, body); err == nil {
		t.Fatal("expected error for missing v0= prefix")
	}
}

func TestSlackDecoder_BuildsConversationID(t *testing.T) {
	body := []byte(`{
		"team_id":"T1","event_id":"Ev1","event_time":1700000000,
		"event":{"type":"message","user":"U1","text":"hi","channel":"C0123","ts":"1700000000.000100","thread_ts":"1700000000.000050"}
	}`)
	d := &Decoder{}
	env, err := d.Decode(context.Background(), body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := "slack:C0123:1700000000.000050"
	if env.ConversationID != want {
		t.Fatalf("conversation_id: got %q want %q", env.ConversationID, want)
	}
	if env.ID != "Ev1" {
		t.Fatalf("event id: got %q", env.ID)
	}
}

func TestSlackDecoder_FallsBackToTsWhenNoThread(t *testing.T) {
	body := []byte(`{
		"event_id":"Ev2","event_time":1700000000,
		"event":{"channel":"C0456","ts":"1700000000.000200"}
	}`)
	env, err := (&Decoder{}).Decode(context.Background(), body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.ConversationID != "slack:C0456:1700000000.000200" {
		t.Fatalf("got %q", env.ConversationID)
	}
}
