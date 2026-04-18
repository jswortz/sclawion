package auth

import "context"

// OIDCVerifier validates Pub/Sub push OIDC tokens. The receiver expects the
// JWT in the Authorization header signed by Google with audience set to the
// receiver service URL.
//
// Implementation TODO: wrap google.golang.org/api/idtoken.Validate.
type OIDCVerifier interface {
	Verify(ctx context.Context, token string, audience string) (subject string, err error)
}
