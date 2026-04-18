package slack

import (
	"context"
	"errors"

	"github.com/jswortz/sclawion/pkg/event"
)

type Encoder struct {
	// TODO: inject Slack client (uses chat.postMessage), correlation store, secrets.
}

func (e *Encoder) Encode(ctx context.Context, ev *event.Envelope) error {
	return errors.New("slack: encoder not implemented")
}
