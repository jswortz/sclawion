// Package scion is a typed client for the Scion Hub REST API.
//
// This is the only package in sclawion that knows about Scion's wire format.
// If Scion's API changes, fix it here.
//
//   POST /api/v1/agents             — dispatch a new agent
//   GET  /api/v1/agents/:id         — fetch agent state
//   POST /api/v1/agents/:id/message — queue a message to a running agent
//   GET  /api/v1/agents/:id/logs    — WebSocket log stream
package scion

import (
	"context"
	"errors"
)

type Client struct {
	BaseURL string // e.g., https://hub.example.internal
	// HTTP client + auth token source TODO.
}

type DispatchRequest struct {
	Template string            `json:"template"`
	Task     string            `json:"task"`
	Labels   map[string]string `json:"labels,omitempty"`
}

type Agent struct {
	ID    string `json:"id"`
	Phase string `json:"phase"` // pending, running, succeeded, failed
}

func (c *Client) Dispatch(ctx context.Context, req DispatchRequest) (*Agent, error) {
	return nil, errors.New("scion: Dispatch not implemented")
}

func (c *Client) Message(ctx context.Context, agentID, message string) error {
	return errors.New("scion: Message not implemented")
}

func (c *Client) Get(ctx context.Context, agentID string) (*Agent, error) {
	return nil, errors.New("scion: Get not implemented")
}
