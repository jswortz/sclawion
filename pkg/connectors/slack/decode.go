package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jswortz/sclawion/pkg/event"
)

type Decoder struct{}

// rawEvent captures the subset of Slack's Events API payload we currently use.
type rawEvent struct {
	TeamID    string `json:"team_id"`
	EventID   string `json:"event_id"`
	EventTime int64  `json:"event_time"`
	Event     struct {
		Type    string `json:"type"`
		User    string `json:"user"`
		Text    string `json:"text"`
		Channel string `json:"channel"`
		Ts      string `json:"ts"`
		ThreadTs string `json:"thread_ts"`
	} `json:"event"`
}

func (d *Decoder) Decode(ctx context.Context, body []byte) (*event.Envelope, error) {
	var r rawEvent
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("slack: decode: %w", err)
	}
	thread := r.Event.ThreadTs
	if thread == "" {
		thread = r.Event.Ts
	}
	return &event.Envelope{
		SpecVersion:    event.SpecVersion,
		ID:             r.EventID,
		ConversationID: fmt.Sprintf("slack:%s:%s", r.Event.Channel, thread),
		Platform:       event.PlatformSlack,
		Kind:           event.KindUserMessage,
		OccurredAt:     time.Unix(r.EventTime, 0),
		Payload:        body, // keep raw for now; router only needs ConversationID
	}, nil
}
