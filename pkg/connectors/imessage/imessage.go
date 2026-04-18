// Package imessage implements the iMessage connector via a webhook bridge.
//
// Apple has no public webhook API for iMessage (Apple Messages for Business is
// gated to approved Messaging Service Providers). For everyone else the
// integration goes through a third-party bridge that owns an Apple ID, hosts a
// macOS instance, and forwards incoming iMessages over HTTP. The two common
// providers are Sendblue (managed SaaS) and BlueBubbles (self-hosted on a Mac
// mini); both sign their webhooks with HMAC-SHA256 over the raw JSON body and
// expose a "sb-signature" / "X-BlueBubbles-Signature" header.
//
// Provider:           Sendblue (default; BlueBubbles uses the same scheme,
//                     just a different header — see Verifier.Header).
// Signature header:   sb-signature
// Algorithm:          HMAC-SHA256(signing_secret, raw_body), hex-encoded
// Timestamp window:   none from the bridge — we apply auth.MaxSkew against
//                     the bridge's "date_sent" field once Decoder lands.
//
// Outbound goes back through the same bridge's REST API (POST /send-message
// for Sendblue) using an OAuth-style API key stored as the connector's
// oauth_token secret slot.
//
//   https://docs.sendblue.com/docs/security
//   https://docs.bluebubbles.app/server/integrations/webhooks
package imessage

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jswortz/sclawion/pkg/auth"
	"github.com/jswortz/sclawion/pkg/event"
)

// Verifier validates an iMessage bridge webhook signature.
type Verifier struct {
	// SigningSecret is the bridge's HMAC key (Sendblue dashboard → Webhooks →
	// "Signing Secret"; BlueBubbles config → "webhook_password").
	SigningSecret []byte
	// Header is the signature header. Defaults to "sb-signature" (Sendblue);
	// override to "X-BlueBubbles-Signature" for self-hosted BlueBubbles.
	Header string
}

func (v *Verifier) Verify(ctx context.Context, r *http.Request, body []byte) error {
	header := v.Header
	if header == "" {
		header = "sb-signature"
	}
	sig := r.Header.Get(header)
	if sig == "" {
		return fmt.Errorf("imessage: missing %s header", header)
	}
	return auth.VerifyHMACSHA256(v.SigningSecret, body, sig)
}

// Decoder normalizes an iMessage bridge payload into an Envelope.
//
// Sendblue payload shape (relevant fields):
//
//	{
//	  "accountEmail": "...",
//	  "content": "hi",
//	  "from_number": "+15551234567",
//	  "to_number":   "+15559998888",
//	  "message_handle": "p:0/AB12CD-...",
//	  "date_sent": "2026-04-18T19:30:00Z",
//	  "is_outbound": false,
//	  "message_type": "message"
//	}
//
// We key the conversation on (to_number, from_number) so the same thread is
// addressable from either direction. The Sendblue message_handle is the
// idempotency ID; if a payload arrives without one, we synthesize a hash.
type Decoder struct{}

func (d *Decoder) Decode(ctx context.Context, body []byte) (*event.Envelope, error) {
	return nil, errors.New("imessage: decoder not implemented (see package doc for payload shape)")
}

// Encoder posts an outbound message back through the bridge's send API.
type Encoder struct {
	// APIKey is the bridge's REST credential (Sendblue: "API Key" on the
	// dashboard; BlueBubbles: the server's GUID + password).
	APIKey string
	// Endpoint defaults to https://api.sendblue.co/api/send-message.
	Endpoint string
}

func (e *Encoder) Encode(ctx context.Context, ev *event.Envelope) error {
	return errors.New("imessage: encoder not implemented")
}
