package policy

import "fmt"

// AgentOperationPolicy controls automatic command approval on a server.
type AgentOperationPolicy string

const (
	AgentApproval  AgentOperationPolicy = "approval"
	AgentSafeRead  AgentOperationPolicy = "safe_read"
	AgentReadOnly  AgentOperationPolicy = "read_only"
	AgentSafeWrite AgentOperationPolicy = "safe_write"
	AgentTrust     AgentOperationPolicy = "trust"
)

func (p AgentOperationPolicy) Validate() error {
	switch p {
	case "", AgentApproval, AgentSafeRead, AgentReadOnly, AgentSafeWrite, AgentTrust:
		return nil
	default:
		return fmt.Errorf("invalid agent operation policy %q", p)
	}
}

// Allows is evaluated only after the deterministic blacklist check.
func (p AgentOperationPolicy) Allows(level string) bool {
	switch p {
	case AgentTrust:
		return true
	case AgentSafeRead:
		return level == "SAFE_READ"
	case AgentReadOnly:
		return level == "SAFE_READ" || level == "SENSITIVE_READ"
	case AgentSafeWrite:
		return level == "SAFE_READ" || level == "SENSITIVE_READ" || level == "SAFE_CHANGE"
	default:
		return false
	}
}
