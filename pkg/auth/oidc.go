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

// IAPClaims is the subset of an Identity-Aware Proxy JWT that the admin plane
// needs. IAP issues a signed JWT in the X-Goog-IAP-JWT-Assertion header on
// every request; the audience is the backend service resource path.
type IAPClaims struct {
	Email string // verified Google account email
	Sub   string // stable IAP subject (user@example.com → accounts.google.com:user@…)
}

// IAPVerifier validates an IAP-issued JWT and returns its claims.
//
// Implementation TODO: wrap google.golang.org/api/idtoken.Validate against the
// IAP JWK set (https://www.gstatic.com/iap/verify/public_key-jwk) and require
// audience == "/projects/<project_number>/global/backendServices/<backend_id>".
type IAPVerifier interface {
	Verify(ctx context.Context, jwt string, audience string) (*IAPClaims, error)
}
