package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/jswortz/sclawion/pkg/event"
	"github.com/jswortz/sclawion/pkg/secrets"
)

// SecretNamer derives the deterministic Secret Manager resource name for a
// connector secret slot.
func SecretNamer(projectID, tenantID string, p event.Platform, kind SecretKind) string {
	return fmt.Sprintf("projects/%s/secrets/sclawion-%s-%s-%s",
		projectID, tenantID, p, secretSlot(kind))
}

func secretSlot(k SecretKind) string {
	switch k {
	case SecretKindSigning:
		return "signing"
	case SecretKindOAuth:
		return "oauth"
	default:
		return string(k)
	}
}

// ErrSecretMissing is returned when the secret resource has not been
// pre-created by Terraform. The plane never creates secret resources.
var ErrSecretMissing = errors.New("config: secret resource missing — Terraform must create it first")

// SecretRotator wires the admin-api's rotate handler to the Secret Manager
// writer and the Store. It updates the connector doc's SecretRef but never
// writes the plaintext value anywhere except Secret Manager itself.
type SecretRotator struct {
	ProjectID string
	Writer    secrets.Writer
	Store     Store
}

// Rotate adds a new version of the named connector secret and updates the
// connector doc's SecretRef to point at the new version. Returns the new ref.
func (r *SecretRotator) Rotate(ctx context.Context, tenantID string, p event.Platform, kind SecretKind, value []byte) (secrets.SecretRef, error) {
	name := SecretNamer(r.ProjectID, tenantID, p, kind)
	ref, err := r.Writer.AddVersion(ctx, name, value)
	if err != nil {
		return secrets.SecretRef{}, fmt.Errorf("rotate: %w", err)
	}
	c, err := r.Store.GetConnector(ctx, tenantID, p)
	if err != nil {
		// Connector doc must already exist; rotate is not a connector creator.
		return ref, fmt.Errorf("rotate: connector not found: %w", err)
	}
	switch kind {
	case SecretKindSigning:
		c.SigningSecretRef = ref
	case SecretKindOAuth:
		c.OAuthTokenRef = ref
	default:
		return ref, fmt.Errorf("rotate: unknown kind %q", kind)
	}
	if err := r.Store.PutConnector(ctx, c); err != nil {
		return ref, fmt.Errorf("rotate: persist connector: %w", err)
	}
	return ref, nil
}
