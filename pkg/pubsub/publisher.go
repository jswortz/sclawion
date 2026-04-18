// Package pubsub provides a thin publisher that JSON-encodes Envelopes and
// sets the right attributes + ordering key.
package pubsub

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jswortz/sclawion/pkg/event"
)

const (
	TopicInbound  = "sclawion.inbound"
	TopicOutbound = "sclawion.outbound"
)

type Publisher interface {
	Publish(ctx context.Context, topic string, e *event.Envelope) error
}

// EnvelopeBytes returns the JSON-encoded Envelope used as the Pub/Sub message body.
func EnvelopeBytes(e *event.Envelope) ([]byte, error) {
	if e.SpecVersion == "" {
		return nil, errors.New("pubsub: envelope missing spec_version")
	}
	return json.Marshal(e)
}
