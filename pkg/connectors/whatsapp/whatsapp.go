// Package whatsapp implements the Meta WhatsApp Cloud API connector.
//
// Signature scheme: HMAC-SHA256 over the raw body, hex-encoded, prefixed with
// "sha256=" in the X-Hub-Signature-256 header. The secret is the App Secret
// from the Meta App dashboard.
//   https://developers.facebook.com/docs/messenger-platform/webhooks#validate-payloads
package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jswortz/sclawion/pkg/auth"
	"github.com/jswortz/sclawion/pkg/event"
)

type Verifier struct {
	AppSecret []byte
}

func (v *Verifier) Verify(ctx context.Context, r *http.Request, body []byte) error {
	sig := r.Header.Get("X-Hub-Signature-256")
	const prefix = "sha256="
	if !strings.HasPrefix(sig, prefix) {
		return fmt.Errorf("whatsapp: missing or malformed X-Hub-Signature-256")
	}
	return auth.VerifyHMACSHA256(v.AppSecret, body, strings.TrimPrefix(sig, prefix))
}

type Decoder struct{}

func (d *Decoder) Decode(ctx context.Context, body []byte) (*event.Envelope, error) {
	return nil, errors.New("whatsapp: decoder not implemented")
}

type Encoder struct{}

func (e *Encoder) Encode(ctx context.Context, ev *event.Envelope) error {
	return errors.New("whatsapp: encoder not implemented")
}
