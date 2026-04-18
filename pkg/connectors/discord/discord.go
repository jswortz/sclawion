// Package discord implements the Discord connector.
//
// Signature scheme: Ed25519 over (X-Signature-Timestamp || raw_body), verified
// with the application's public key. See:
//   https://discord.com/developers/docs/interactions/overview#setting-up-an-endpoint
package discord

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/jswortz/sclawion/pkg/event"
)

type Verifier struct {
	PublicKey ed25519.PublicKey
}

func (v *Verifier) Verify(ctx context.Context, r *http.Request, body []byte) error {
	sigHex := r.Header.Get("X-Signature-Ed25519")
	ts := r.Header.Get("X-Signature-Timestamp")
	if sigHex == "" || ts == "" {
		return fmt.Errorf("discord: missing signature headers")
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return fmt.Errorf("discord: bad signature hex: %w", err)
	}
	msg := append([]byte(ts), body...)
	if !ed25519.Verify(v.PublicKey, msg, sig) {
		return errors.New("discord: signature invalid")
	}
	return nil
}

type Decoder struct{}

func (d *Decoder) Decode(ctx context.Context, body []byte) (*event.Envelope, error) {
	return nil, errors.New("discord: decoder not implemented")
}

type Encoder struct{}

func (e *Encoder) Encode(ctx context.Context, ev *event.Envelope) error {
	return errors.New("discord: encoder not implemented")
}
