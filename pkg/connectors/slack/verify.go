// Package slack implements the Slack Verifier/Decoder/Encoder.
package slack

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jswortz/sclawion/pkg/auth"
)

// Verifier validates Slack request signatures.
//
// Spec: https://docs.slack.dev/authentication/verifying-requests-from-slack
//   sig_basestring = "v0:" + timestamp + ":" + body
//   expected       = "v0=" + hex(HMAC-SHA256(signing_secret, sig_basestring))
type Verifier struct {
	SigningSecret []byte
	Now           func() time.Time // injectable for tests
}

func (v *Verifier) Verify(ctx context.Context, r *http.Request, body []byte) error {
	tsHeader := r.Header.Get("X-Slack-Request-Timestamp")
	sig := r.Header.Get("X-Slack-Signature")
	if tsHeader == "" || sig == "" {
		return fmt.Errorf("slack: missing signature headers")
	}
	tsInt, err := strconv.ParseInt(tsHeader, 10, 64)
	if err != nil {
		return fmt.Errorf("slack: bad timestamp: %w", err)
	}
	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	if err := auth.CheckTimestampSkew(time.Unix(tsInt, 0), now()); err != nil {
		return err
	}

	const prefix = "v0="
	if !strings.HasPrefix(sig, prefix) {
		return fmt.Errorf("slack: signature missing v0= prefix")
	}
	basestring := []byte("v0:" + tsHeader + ":")
	basestring = append(basestring, body...)
	return auth.VerifyHMACSHA256(v.SigningSecret, basestring, strings.TrimPrefix(sig, prefix))
}
