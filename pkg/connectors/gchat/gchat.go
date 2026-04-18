// Package gchat implements the Google Chat connector.
//
// Auth: Google signs an ID token (RS256 JWT) and sends it in the
// Authorization: Bearer <token> header. Audience must equal the configured
// project. Verify with Google's JWKS (rotated automatically by the library).
//   https://developers.google.com/workspace/chat/authenticate-authorize-chat-app
package gchat

import (
	"context"
	"errors"
	"net/http"

	"github.com/jswortz/sclawion/pkg/auth"
	"github.com/jswortz/sclawion/pkg/event"
)

type Verifier struct {
	OIDC     auth.OIDCVerifier
	Audience string // typically the project number or service URL
}

func (v *Verifier) Verify(ctx context.Context, r *http.Request, body []byte) error {
	const prefix = "Bearer "
	authz := r.Header.Get("Authorization")
	if len(authz) <= len(prefix) || authz[:len(prefix)] != prefix {
		return errors.New("gchat: missing Bearer token")
	}
	if _, err := v.OIDC.Verify(ctx, authz[len(prefix):], v.Audience); err != nil {
		return err
	}
	return nil
}

type Decoder struct{}

func (d *Decoder) Decode(ctx context.Context, body []byte) (*event.Envelope, error) {
	return nil, errors.New("gchat: decoder not implemented")
}

type Encoder struct{}

func (e *Encoder) Encode(ctx context.Context, ev *event.Envelope) error {
	return errors.New("gchat: encoder not implemented")
}
