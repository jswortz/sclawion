package secrets

import "context"

// SecretRef points at a specific Secret Manager version. The Name is the full
// resource path (projects/<id>/secrets/<name>) and Version is the numeric
// version returned by AddVersion.
type SecretRef struct {
	Name    string
	Version string
}

// Writer adds new versions to existing Secret Manager secrets. The admin plane
// uses this for secret rotation; the secret resource itself is created by
// Terraform (creation = IAM change), so AddVersion must fail if the secret
// does not exist.
//
// Kept separate from Manager.Get so the data-plane services that only need to
// read can depend on the smaller interface.
//
// Implementation TODO: wrap cloud.google.com/go/secretmanager/apiv1.
type Writer interface {
	// AddVersion appends a new version to the named secret and returns the
	// resulting reference. The plaintext value is never logged or echoed back.
	AddVersion(ctx context.Context, name string, value []byte) (SecretRef, error)
}
