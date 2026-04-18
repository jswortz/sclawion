package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jswortz/sclawion/pkg/event"
)

// ErrInvalid is returned when a config struct fails validation.
var ErrInvalid = errors.New("config: invalid")

var (
	// idPattern is the allowed shape for tenant/agent/swarm IDs. Lowercase
	// letters, digits, hyphens; 2-63 chars. Matches DNS-label rules so the same
	// IDs can be reused as Cloud resource names if needed.
	idPattern    = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)
	emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// validPlatforms is the closed set of chat platforms sclawion supports today.
var validPlatforms = map[event.Platform]struct{}{
	event.PlatformSlack:    {},
	event.PlatformDiscord:  {},
	event.PlatformGChat:    {},
	event.PlatformWhatsApp: {},
}

// validTopologies and validRoles mirror docs/clawpath/SWARMS.md.
var (
	validTopologies = map[string]struct{}{
		"pipeline":     {},
		"fanout":       {},
		"mesh":         {},
		"hierarchical": {},
	}
	validRoles = map[string]struct{}{
		"planner":  {},
		"coder":    {},
		"reviewer": {},
		"deployer": {},
		"monitor":  {},
	}
	validMemoryScopes = map[string]struct{}{
		"conversation": {},
		"user":         {},
	}
)

// ValidateTenant checks tenant-level invariants.
func ValidateTenant(t *Tenant) error {
	if !idPattern.MatchString(t.ID) {
		return fmt.Errorf("%w: tenant id %q must match %s", ErrInvalid, t.ID, idPattern)
	}
	if strings.TrimSpace(t.DisplayName) == "" {
		return fmt.Errorf("%w: display_name required", ErrInvalid)
	}
	return nil
}

// ValidateConnector checks connector-level invariants. The two SecretRef
// fields are not required on PUT (set via :rotate) — they are validated only
// when present.
func ValidateConnector(c *ConnectorConfig) error {
	if !idPattern.MatchString(c.TenantID) {
		return fmt.Errorf("%w: tenant id %q invalid", ErrInvalid, c.TenantID)
	}
	if _, ok := validPlatforms[c.Platform]; !ok {
		return fmt.Errorf("%w: platform %q unknown", ErrInvalid, c.Platform)
	}
	wantPath := "/v1/" + string(c.Platform)
	if c.WebhookPath != "" && c.WebhookPath != wantPath {
		return fmt.Errorf("%w: webhook_path must be %s", ErrInvalid, wantPath)
	}
	if c.RateLimitPerConv < 0 || c.RateLimitPerConv > 600 {
		return fmt.Errorf("%w: rate_limit_per_conv must be 0..600 (got %d)", ErrInvalid, c.RateLimitPerConv)
	}
	return nil
}

// ValidateAgent checks agent-profile invariants.
func ValidateAgent(a *AgentProfile) error {
	if !idPattern.MatchString(a.TenantID) {
		return fmt.Errorf("%w: tenant id %q invalid", ErrInvalid, a.TenantID)
	}
	if !idPattern.MatchString(a.ID) {
		return fmt.Errorf("%w: agent id %q invalid", ErrInvalid, a.ID)
	}
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("%w: name required", ErrInvalid)
	}
	if strings.TrimSpace(a.Model) == "" {
		return fmt.Errorf("%w: model required", ErrInvalid)
	}
	if a.MemoryScope != "" {
		if _, ok := validMemoryScopes[a.MemoryScope]; !ok {
			return fmt.Errorf("%w: memory_scope must be conversation|user", ErrInvalid)
		}
	}
	if a.BudgetUSDPerHour < 0 {
		return fmt.Errorf("%w: budget_usd_per_hour must be >= 0", ErrInvalid)
	}
	return nil
}

// ValidateSwarm checks swarm-definition invariants. Topology and roster must
// be from the closed sets defined above.
func ValidateSwarm(s *SwarmDef) error {
	if !idPattern.MatchString(s.TenantID) {
		return fmt.Errorf("%w: tenant id %q invalid", ErrInvalid, s.TenantID)
	}
	if !idPattern.MatchString(s.ID) {
		return fmt.Errorf("%w: swarm id %q invalid", ErrInvalid, s.ID)
	}
	if _, ok := validTopologies[s.Topology]; !ok {
		return fmt.Errorf("%w: topology %q unknown", ErrInvalid, s.Topology)
	}
	if len(s.Roster) == 0 {
		return fmt.Errorf("%w: roster must not be empty", ErrInvalid)
	}
	for _, r := range s.Roster {
		if _, ok := validRoles[r]; !ok {
			return fmt.Errorf("%w: role %q unknown (see docs/clawpath/SWARMS.md)", ErrInvalid, r)
		}
	}
	if s.BudgetEnvelope.MaxTokens < 0 || s.BudgetEnvelope.MaxDeploys < 0 {
		return fmt.Errorf("%w: budget envelope counts must be >= 0", ErrInvalid)
	}
	return nil
}

// ValidateAdminUser checks an AdminUser before write.
func ValidateAdminUser(u *AdminUser) error {
	if !emailPattern.MatchString(u.Email) {
		return fmt.Errorf("%w: email %q invalid", ErrInvalid, u.Email)
	}
	switch u.Role {
	case RoleOwner, RoleOperator, RoleViewer:
	default:
		return fmt.Errorf("%w: role must be owner|operator|viewer (got %q)", ErrInvalid, u.Role)
	}
	return nil
}
