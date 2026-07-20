package protocol

// Rule IDs are stable machine-readable explanations for admission decisions.
// Human-readable reasons may evolve without breaking policy automation.
const (
	RuleCandidateMutated         = "ST001"
	RuleTrustedBaseChanged       = "ST002"
	RuleFilesystemEscape         = "ST003"
	RuleVerifierFailed           = "VR001"
	RuleVerifierTimedOut         = "VR002"
	RuleVerifierUnavailable      = "VR003"
	RuleProtectedPathChanged     = "PV001"
	RulePolicyChanged            = "PV002"
	RuleReceiptIntegrity         = "PV003"
	RuleTerminalRecovered        = "CP001"
	RuleCheckpointUnavailable    = "CP002"
	RuleCheckpointMismatch       = "CP003"
	RuleRecertificationFailed    = "CP004"
	RuleWallBudgetExhausted      = "LC001"
	RuleCandidateBudgetExhausted = "LC002"
	RuleNoProgress               = "LC003"
	RuleAgentExited              = "EX001"
)

var ruleSummaries = map[string]string{
	RuleCandidateMutated:         "candidate state changed during verification",
	RuleTrustedBaseChanged:       "trusted base changed during managed execution",
	RuleFilesystemEscape:         "candidate escaped the repository boundary",
	RuleVerifierFailed:           "a configured verifier failed",
	RuleVerifierTimedOut:         "a configured verifier timed out",
	RuleVerifierUnavailable:      "verifier execution could not complete",
	RuleProtectedPathChanged:     "a protected path changed without an active exception",
	RulePolicyChanged:            "policy changed after admission",
	RuleReceiptIntegrity:         "receipt integrity validation failed",
	RuleTerminalRecovered:        "terminal regression was replaced by a freshly recertified checkpoint",
	RuleCheckpointUnavailable:    "no usable verified checkpoint was available",
	RuleCheckpointMismatch:       "checkpoint state could not be reproduced",
	RuleRecertificationFailed:    "fresh checkpoint recertification failed",
	RuleWallBudgetExhausted:      "agent wall-time budget was exhausted",
	RuleCandidateBudgetExhausted: "candidate budget was exhausted",
	RuleNoProgress:               "agent repeated or oscillated without making progress",
	RuleAgentExited:              "agent process exited with an error",
}

func RuleSummary(id string) string { return ruleSummaries[id] }
