// Package secrets wraps Google Secret Manager with caching and explicit
// names — never read credentials via os.Getenv.
package secrets

import (
	"context"
	"errors"
)

type Manager interface {
	// Get returns the latest enabled version of the named secret.
	Get(ctx context.Context, name string) ([]byte, error)
}

// Names enumerates every secret this project expects. Keep in sync with
// deploy/terraform/secrets.tf.
const (
	NameSlackSigningSecret    = "slack-signing-secret"
	NameSlackBotToken         = "slack-bot-token"
	NameDiscordPublicKey      = "discord-public-key"
	NameDiscordBotToken       = "discord-bot-token"
	NameWhatsAppAppSecret     = "whatsapp-app-secret"
	NameWhatsAppAccessToken   = "whatsapp-access-token"
	NameGChatServiceAccount   = "gchat-service-account" // workload-identity audience config
)

var ErrNotFound = errors.New("secrets: not found")
