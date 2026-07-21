package broker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/internal/store"
	"github.com/hellogxp/stateseal/internal/verifier"
	"github.com/hellogxp/stateseal/internal/worktree"
	"github.com/hellogxp/stateseal/pkg/protocol"
)

type Broker struct {
	Policy         config.Policy
	PolicyHash     string
	Root           string
	Store          *store.Store
	State          protocol.TaskState
	terminalReason string
}

// RecordAbstention records a protocol-level failure without changing the
// last verified checkpoint.
func (b *Broker) RecordAbstention(reason string) (protocol.CompletionReceipt, error) {
	return b.finish(protocol.VerdictAbstained, b.State.Checkpoint, nil, reason)
}

func (b *Broker) RecordEscalation(reason string) (protocol.CompletionReceipt, error) {
	return b.finish(protocol.VerdictEscalated, b.State.Checkpoint, nil, reason)
}

// RecordRejection converts a broker-level delivery obligation into a durable
// rejection. It is used for constraints such as a code-changing task that
// produced no deliverable change, which verifier success alone cannot prove.
func (b *Broker) RecordRejection(reason string) (protocol.CompletionReceipt, error) {
	b.State.CandidatesRejected++
	b.State.Checkpoint = nil
	return b.finish(protocol.VerdictRejected, nil, nil, reason)
}

func New(root, mode string) (*Broker, error) {
	p, _, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	return NewTask(root, mode, p.Task.ID, p.Task.Goal)
}

// NewTask starts a task using project policy from seal.yaml while keeping the
// task identity and goal in external authority state.
func NewTask(root, mode, taskID, goal string) (*Broker, error) {
	if mode != "shadow" && mode != "warn" && mode != "enforce" {
		return nil, fmt.Errorf("invalid mode %q", mode)
	}
	if err := identity.ValidateTaskID(taskID); err != nil {
		return nil, err
	}
	if err := identity.EnsureLocalExclude(root, ".stateseal/"); err != nil {
		return nil, err
	}
	p, raw, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	s, err := store.Open(root, taskID)
	if err != nil {
		return nil, err
	}
	base, _ := identity.Git(root, "rev-parse", "HEAD")
	b := &Broker{Policy: p, PolicyHash: identity.Digest(raw), Root: root, Store: s}
	b.State = protocol.TaskState{Version: protocol.Version, TaskID: taskID, Goal: goal, RepoRoot: root,
		BaseCommit: strings.TrimSpace(string(base)), Mode: mode, Status: "WORKING", Coverage: "terminal-only"}
	if err := b.Store.Save(b.State); err != nil {
		return nil, err
	}
	if err := store.SetActiveTask(root, taskID); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *Broker) event(kind string, data map[string]any) error {
	_, err := b.Store.Append(protocol.Event{Type: kind, TaskID: b.State.TaskID, Data: data})
	return err
}

func (b *Broker) VerifyCurrent(checks []config.Check) (protocol.CompletionReceipt, error) {
	if err := b.event("TASK_CREATED", map[string]any{"mode": b.State.Mode}); err != nil {
		return protocol.CompletionReceipt{}, err
	}
	tree, err := identity.Tree(b.Root, b.Policy.State.Include)
	if err != nil {
		return protocol.CompletionReceipt{}, err
	}
	if unsafe, err := identity.UnsafeSymlinks(b.Root); err != nil || len(unsafe) > 0 {
		if err != nil {
			return b.finish(protocol.VerdictAbstained, nil, nil, err.Error())
		}
		return b.finish(protocol.VerdictRejected, nil, nil, "symlink escapes repository: "+unsafe[0])
	}
	baseTree, _ := identity.Git(b.Root, "rev-parse", "HEAD^{tree}")
	candidate := protocol.CandidateState{CandidateID: identity.ID("cand"), TaskID: b.State.TaskID,
		BaseTreeSHA256: identity.Digest(baseTree), PatchSHA256: patchDigest(b.Root), ResultTreeSHA256: tree,
		CreatedAt: time.Now().UTC(), Source: "seal-verify"}
	b.State.Candidate = &candidate
	b.State.CandidatesEvaluated++
	b.State.Status = "VERIFYING"
	b.event("CANDIDATE_SUBMITTED", map[string]any{"candidate_id": candidate.CandidateID, "tree_sha256": tree})
	evidence, err := verifier.Run(b.Root, candidate.CandidateID, tree, b.PolicyHash, checks)
	b.State.Evidence = append(b.State.Evidence, evidence...)
	if err != nil {
		return b.finish(protocol.VerdictAbstained, nil, evidence, err.Error())
	}
	postTree, err := identity.Tree(b.Root, b.Policy.State.Include)
	if err != nil {
		return b.finish(protocol.VerdictAbstained, nil, evidence, err.Error())
	}
	if postTree != tree {
		return b.finish(protocol.VerdictStale, nil, evidence, "code changed while verification was running")
	}
	if !verifier.Passed(evidence, len(checks)) {
		b.State.CandidatesRejected++
		return b.finish(protocol.VerdictRejected, nil, evidence, failureReason("one or more checks failed", evidence))
	}
	cp := &protocol.VerifiedCheckpoint{CheckpointID: identity.ID("cp"), CandidateID: candidate.CandidateID,
		TreeSHA256: tree, AdmissionEvidence: evidenceIDs(evidence), PolicyDigest: b.PolicyHash, VerifiedAt: time.Now().UTC()}
	b.State.Checkpoint = cp
	b.State.CheckpointsVerified++
	b.event("CHECKPOINT_VERIFIED", map[string]any{"checkpoint_id": cp.CheckpointID, "tree_sha256": tree})
	return b.finish(protocol.VerdictAdmitted, cp, evidence, "")
}

func (b *Broker) AdmitManaged(m *worktree.Manager, proposal, source string) (protocol.CompletionReceipt, error) {
	return b.AdmitManagedWithReason(m, proposal, source, "terminal_candidate_regressed")
}

// AdmitManagedWithReason evaluates a terminal candidate and records why an
// older checkpoint was selected when terminal admission fails.
func (b *Broker) AdmitManagedWithReason(m *worktree.Manager, proposal, source, recoveryReason string) (protocol.CompletionReceipt, error) {
	b.terminalReason = recoveryReason
	defer func() { b.terminalReason = "" }()
	return b.admitManaged(m, proposal, source, true, recoveryReason)
}

// AdmitIntermediate evaluates a candidate boundary without falling back to an
// older checkpoint. Recovery is reserved for terminal completion selection.
func (b *Broker) AdmitIntermediate(m *worktree.Manager, proposal, source string) (protocol.CompletionReceipt, error) {
	return b.admitManaged(m, proposal, source, false, "")
}

func (b *Broker) admitManaged(m *worktree.Manager, proposal, source string, recoverTerminal bool, recoveryReason string) (protocol.CompletionReceipt, error) {
	if err := b.event("TASK_CREATED", map[string]any{"mode": b.State.Mode}); err != nil {
		return protocol.CompletionReceipt{}, err
	}
	previousCheckpoint := cloneCheckpoint(b.State.Checkpoint)
	b.State.ProposalPath = proposal
	head, err := identity.Git(m.Root, "rev-parse", "HEAD")
	if err != nil {
		return b.finish(protocol.VerdictAbstained, b.State.Checkpoint, nil, err.Error())
	}
	dirty, err := identity.Git(m.Root, "status", "--porcelain")
	if err != nil {
		return b.finish(protocol.VerdictAbstained, b.State.Checkpoint, nil, err.Error())
	}
	if strings.TrimSpace(string(head)) != m.Base || len(strings.TrimSpace(string(dirty))) > 0 {
		return b.finish(protocol.VerdictStale, b.State.Checkpoint, nil, "trusted base changed during managed execution")
	}
	commit, err := m.CommitCandidate(proposal, deliveryCommitMessage(b.State.Goal))
	if err != nil {
		if recoverTerminal && previousCheckpoint != nil {
			return b.recertifyCheckpoint(m, previousCheckpoint, "", recoveryReason)
		}
		return b.finish(protocol.VerdictAbstained, nil, nil, err.Error())
	}
	eval, cleanup, err := m.Evaluator(commit)
	if err != nil {
		if recoverTerminal && previousCheckpoint != nil {
			return b.recertifyCheckpoint(m, previousCheckpoint, "", recoveryReason)
		}
		return b.finish(protocol.VerdictAbstained, nil, nil, err.Error())
	}
	defer cleanup()
	tree, err := identity.Tree(eval, b.Policy.State.Include)
	if err != nil {
		if recoverTerminal && previousCheckpoint != nil {
			return b.recertifyCheckpoint(m, previousCheckpoint, "", recoveryReason)
		}
		return b.finish(protocol.VerdictAbstained, nil, nil, err.Error())
	}
	baseTree, err := identity.Tree(m.Root, b.Policy.State.Include)
	if err != nil {
		if recoverTerminal && previousCheckpoint != nil {
			return b.recertifyCheckpoint(m, previousCheckpoint, "", recoveryReason)
		}
		return b.finish(protocol.VerdictAbstained, b.State.Checkpoint, nil, err.Error())
	}
	patch, _ := identity.Git(m.Root, "diff", "--binary", m.Base, commit)
	candidate := protocol.CandidateState{CandidateID: identity.ID("cand"), TaskID: b.State.TaskID,
		BaseTreeSHA256: baseTree, PatchSHA256: identity.Digest(patch), ResultTreeSHA256: tree,
		Commit: commit, CreatedAt: time.Now().UTC(), Source: source}
	b.State.Candidate = &candidate
	b.State.CandidatesEvaluated++
	b.State.Status = "VERIFYING"
	b.event("CANDIDATE_SUBMITTED", map[string]any{"candidate_id": candidate.CandidateID, "tree_sha256": tree, "commit": commit})
	if unsafe, err := identity.UnsafeSymlinks(eval); err != nil || len(unsafe) > 0 {
		if err != nil {
			if recoverTerminal && previousCheckpoint != nil {
				return b.recertifyCheckpoint(m, previousCheckpoint, candidate.CandidateID, recoveryReason)
			}
			return b.finish(protocol.VerdictAbstained, b.State.Checkpoint, nil, err.Error())
		}
		if recoverTerminal && previousCheckpoint != nil {
			return b.recertifyCheckpoint(m, previousCheckpoint, candidate.CandidateID, recoveryReason)
		}
		return b.finish(protocol.VerdictRejected, b.State.Checkpoint, nil, "symlink escapes repository: "+unsafe[0])
	}
	files, err := m.ChangedFiles(m.Base, commit)
	if err != nil {
		if recoverTerminal && previousCheckpoint != nil {
			return b.recertifyCheckpoint(m, previousCheckpoint, candidate.CandidateID, recoveryReason)
		}
		return b.finish(protocol.VerdictAbstained, nil, nil, err.Error())
	}
	for _, file := range files {
		protected := file == "seal.yaml" || identity.MatchesAny(file, b.Policy.State.Protected)
		if protected && !b.Policy.AllowsProtected(file, time.Now().UTC()) {
			b.event("PROTECTED_PATH_REJECTED", map[string]any{"path": file})
			if recoverTerminal && previousCheckpoint != nil {
				return b.recertifyCheckpoint(m, previousCheckpoint, candidate.CandidateID, recoveryReason)
			}
			return b.finish(protocol.VerdictRejected, nil, nil, "protected path changed: "+file)
		}
	}
	admission, err := verifier.RunPhaseWithBudget(eval, candidate.CandidateID, tree, b.PolicyHash, b.Policy.Admission.Checks, "admission", time.Duration(b.Policy.Admission.TimeoutSeconds)*time.Second)
	b.State.Evidence = append(b.State.Evidence, admission...)
	if err != nil {
		return b.finish(protocol.VerdictAbstained, nil, admission, err.Error())
	}
	postTree, _ := identity.Tree(eval, b.Policy.State.Include)
	if postTree != tree {
		return b.finish(protocol.VerdictStale, nil, admission, "candidate changed during admission")
	}
	if !verifier.Passed(admission, len(b.Policy.Admission.Checks)) {
		b.State.CandidatesRejected++
		b.event("CANDIDATE_REJECTED", map[string]any{"reason": "admission check failed"})
		if recoverTerminal && previousCheckpoint != nil {
			return b.recertifyCheckpoint(m, previousCheckpoint, candidate.CandidateID, recoveryReason)
		}
		return b.finish(protocol.VerdictRejected, b.State.Checkpoint, admission, failureReason("admission check failed; last verified checkpoint was preserved", admission))
	}
	cp := &protocol.VerifiedCheckpoint{CheckpointID: identity.ID("cp"), CandidateID: candidate.CandidateID,
		TreeSHA256: tree, Commit: commit, AdmissionEvidence: evidenceIDs(admission), PolicyDigest: b.PolicyHash, VerifiedAt: time.Now().UTC()}
	b.State.Checkpoint = cp
	b.State.CheckpointsVerified++
	b.State.Status = "VERIFIED"
	b.event("CHECKPOINT_VERIFIED", map[string]any{"checkpoint_id": cp.CheckpointID, "tree_sha256": tree, "commit": commit})

	// Completion is always a fresh execution against the immutable checkpoint.
	freshEval, freshCleanup, err := m.Evaluator(commit)
	if err != nil {
		return b.finish(protocol.VerdictAbstained, cp, admission, err.Error())
	}
	defer freshCleanup()
	freshTree, err := identity.Tree(freshEval, b.Policy.State.Include)
	if err != nil || freshTree != cp.TreeSHA256 {
		return b.finish(protocol.VerdictStale, cp, admission, "checkpoint tree could not be reproduced")
	}
	completion, err := verifier.RunPhaseWithBudget(freshEval, candidate.CandidateID, freshTree, b.PolicyHash, b.Policy.Completion.Checks, "completion", time.Duration(b.Policy.Completion.TimeoutSeconds)*time.Second)
	b.State.Evidence = append(b.State.Evidence, completion...)
	if err != nil {
		return b.finish(protocol.VerdictAbstained, cp, completion, err.Error())
	}
	if !verifier.Passed(completion, len(b.Policy.Completion.Checks)) {
		b.State.CandidatesRejected++
		if recoverTerminal && previousCheckpoint != nil && previousCheckpoint.CheckpointID != cp.CheckpointID {
			return b.recertifyCheckpoint(m, previousCheckpoint, candidate.CandidateID, "terminal_completion_failed")
		}
		return b.finish(protocol.VerdictRejected, cp, completion, failureReason("fresh completion recertification failed", completion))
	}
	b.event("COMPLETION_RECERTIFIED", map[string]any{"checkpoint_id": cp.CheckpointID})
	return b.finish(protocol.VerdictAdmitted, cp, completion, "")
}

func deliveryCommitMessage(goal string) string {
	lower := strings.ToLower(goal)
	kind := "feat"
	switch {
	case strings.Contains(lower, "fix") || strings.Contains(goal, "修复"):
		kind = "fix"
	case strings.Contains(lower, "refactor") || strings.Contains(goal, "重构"):
		kind = "refactor"
	case strings.Contains(lower, "document") || strings.Contains(lower, "docs") || strings.Contains(goal, "文档"):
		kind = "docs"
	case strings.Contains(lower, "test") || strings.Contains(goal, "测试"):
		kind = "test"
	}
	message := kind + ": implement verified change"
	if strings.TrimSpace(goal) != "" {
		message += "\n\nGoal: " + strings.TrimSpace(goal)
	}
	return message
}

func (b *Broker) recertifyCheckpoint(m *worktree.Manager, cp *protocol.VerifiedCheckpoint, terminalCandidate, selectionReason string) (protocol.CompletionReceipt, error) {
	b.State.Checkpoint = cp
	_ = b.event("REGRESSION_DETECTED", map[string]any{
		"terminal_candidate": terminalCandidate,
		"checkpoint_id":      cp.CheckpointID,
		"reason":             selectionReason,
	})
	_ = b.event("CHECKPOINT_SELECTED", map[string]any{
		"checkpoint_id": cp.CheckpointID,
		"reason":        selectionReason,
	})
	eval, cleanup, err := m.Evaluator(cp.Commit)
	if err != nil {
		return b.finishSelected(protocol.VerdictAbstained, cp, nil, err.Error(), terminalCandidate, selectionReason, false)
	}
	defer cleanup()
	tree, err := identity.Tree(eval, b.Policy.State.Include)
	if err != nil {
		return b.finishSelected(protocol.VerdictAbstained, cp, nil, err.Error(), terminalCandidate, selectionReason, false)
	}
	if tree != cp.TreeSHA256 {
		return b.finishSelected(protocol.VerdictStale, cp, nil, "selected checkpoint tree could not be reproduced", terminalCandidate, selectionReason, false)
	}
	evidence, err := verifier.RunPhaseWithBudget(eval, cp.CandidateID, tree, b.PolicyHash, b.Policy.Completion.Checks, "recertification", time.Duration(b.Policy.Completion.TimeoutSeconds)*time.Second)
	b.State.Evidence = append(b.State.Evidence, evidence...)
	if err != nil {
		return b.finishSelected(protocol.VerdictAbstained, cp, evidence, err.Error(), terminalCandidate, selectionReason, false)
	}
	if !verifier.Passed(evidence, len(b.Policy.Completion.Checks)) {
		return b.finishSelected(protocol.VerdictRejected, cp, evidence, failureReason("selected checkpoint failed fresh completion recertification", evidence), terminalCandidate, selectionReason, false)
	}
	_ = b.event("CHECKPOINT_RESTORED", map[string]any{"checkpoint_id": cp.CheckpointID, "tree_sha256": tree})
	_ = b.event("COMPLETION_RECERTIFIED", map[string]any{"checkpoint_id": cp.CheckpointID, "recovered": true})
	return b.finishSelected(protocol.VerdictAdmitted, cp, evidence, "", terminalCandidate, selectionReason, true)
}

func failureReason(prefix string, evidence []protocol.EvidenceEnvelope) string {
	for i := len(evidence) - 1; i >= 0; i-- {
		item := evidence[i]
		if item.ExitCode == 0 && !item.TimedOut {
			continue
		}
		output := strings.TrimSpace(item.Output)
		const limit = 2400
		if len(output) > limit {
			output = "…" + output[len(output)-limit:]
		}
		if output == "" {
			return fmt.Sprintf("%s\nFailed verifier: %s (exit %d)", prefix, item.VerifierIdentity, item.ExitCode)
		}
		return fmt.Sprintf("%s\nFailed verifier: %s (exit %d)\n%s", prefix, item.VerifierIdentity, item.ExitCode, output)
	}
	return prefix
}

func (b *Broker) finish(verdict protocol.Verdict, cp *protocol.VerifiedCheckpoint, evidence []protocol.EvidenceEnvelope, reason string) (protocol.CompletionReceipt, error) {
	return b.finishSelected(verdict, cp, evidence, reason, "", "", false)
}

func (b *Broker) finishSelected(verdict protocol.Verdict, cp *protocol.VerifiedCheckpoint, evidence []protocol.EvidenceEnvelope, reason, terminalCandidate, selectionReason string, recovered bool) (protocol.CompletionReceipt, error) {
	ruleID := classifyRule(verdict, reason, selectionReason, b.terminalReason, recovered, evidence)
	disposition := modeDisposition(b.State.Mode, verdict)
	receipt := protocol.CompletionReceipt{ReceiptVersion: protocol.Version, ReceiptID: identity.ID("rcpt"),
		TaskID: b.State.TaskID, Verdict: verdict, RuleID: ruleID, EnforcementMode: b.State.Mode, Disposition: disposition,
		PolicyDigest: b.PolicyHash, IssuedBy: "local-broker",
		IssuedAt: time.Now().UTC(), ResidualRisks: b.Policy.ResidualRisks, Reason: reason,
		TerminalCandidate: terminalCandidate, SelectionReason: selectionReason, Recovered: recovered}
	if len(receipt.ResidualRisks) == 0 {
		receipt.ResidualRisks = []string{"Only configured checks were evaluated."}
	}
	if cp != nil {
		receipt.CheckpointID, receipt.TreeSHA256 = cp.CheckpointID, cp.TreeSHA256
	}
	receipt.CompletionEvidence = evidenceIDs(evidence)
	receipt.VerificationCoverage = b.verificationCoverage(evidence)
	receipt.VerificationCoverage.Uncovered = append([]string(nil), receipt.ResidualRisks...)
	receipt.LivenessImpact = &protocol.LivenessImpact{
		CandidatesEvaluated: b.State.CandidatesEvaluated,
		CandidatesRejected:  b.State.CandidatesRejected,
		CheckpointsVerified: b.State.CheckpointsVerified,
		Recovered:           recovered, SelectionReason: selectionReason,
	}
	receipt.ReceiptDigest = ""
	receipt.ReceiptDigest, _ = identity.JSONDigest(receipt)
	b.State.Receipt = &receipt
	b.State.Status = string(verdict)
	b.State.RuleID = ruleID
	b.State.Disposition = disposition
	b.State.LastError = reason
	if err := b.Store.Save(b.State); err != nil {
		return receipt, err
	}
	kind := "COMPLETION_" + string(verdict)
	if _, err := b.Store.Append(protocol.Event{Type: kind, TaskID: b.State.TaskID,
		Data: map[string]any{"receipt_id": receipt.ReceiptID, "receipt_digest": receipt.ReceiptDigest, "reason": reason, "rule_id": ruleID}}); err != nil {
		return receipt, err
	}
	if _, err := b.Store.Append(protocol.Event{Type: "MODE_DECISION", TaskID: b.State.TaskID,
		Data: map[string]any{"mode": b.State.Mode, "verdict": verdict, "disposition": disposition, "rule_id": ruleID}}); err != nil {
		return receipt, err
	}
	path, err := b.Store.ExportReceipt(receipt)
	if err == nil {
		exportWorkspaceReceipt(b.Root, receipt)
	}
	_ = path
	return receipt, err
}

func (b *Broker) verificationCoverage(evidence []protocol.EvidenceEnvelope) *protocol.VerificationCoverage {
	coverage := &protocol.VerificationCoverage{
		Observation: b.State.Coverage,
		Verifiers:   []protocol.VerifierCoverage{},
		IntegrityControls: []string{
			"exact-code-state binding",
			"policy and verifier-suite binding",
			"receipt integrity",
		},
		Uncovered: []string{},
	}
	for _, item := range evidence {
		if item.VerificationPhase == "completion" || item.VerificationPhase == "recertification" {
			if !contains(coverage.IntegrityControls, "fresh evaluator recertification") {
				coverage.IntegrityControls = append(coverage.IntegrityControls, "fresh evaluator recertification")
			}
		}
		status := "passed"
		if item.TimedOut {
			status = "timed_out"
		} else if item.ExitCode != 0 {
			status = "failed"
		}
		checkID := strings.TrimPrefix(item.VerifierIdentity, "command/")
		checkID = strings.TrimSuffix(checkID, "@v1")
		coverage.Verifiers = append(coverage.Verifiers, protocol.VerifierCoverage{
			CheckID: checkID, Phase: item.VerificationPhase, Layer: item.VerifierLayer,
			Origin: item.VerifierOrigin, EvidenceID: item.EvidenceID, Status: status,
		})
	}
	return coverage
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func modeDisposition(mode string, verdict protocol.Verdict) string {
	if verdict == protocol.VerdictAdmitted {
		return "ALLOWED"
	}
	switch mode {
	case "shadow":
		return "OBSERVED"
	case "warn":
		return "OVERRIDDEN"
	default:
		return "BLOCKED"
	}
}

func classifyRule(verdict protocol.Verdict, reason, selectionReason, terminalReason string, recovered bool, evidence []protocol.EvidenceEnvelope) string {
	if recovered {
		switch selectionReason {
		case "wall_budget_exhausted":
			return protocol.RuleWallBudgetExhausted
		case "agent_exit_error":
			return protocol.RuleAgentExited
		default:
			return protocol.RuleTerminalRecovered
		}
	}
	switch terminalReason {
	case "wall_budget_exhausted":
		return protocol.RuleWallBudgetExhausted
	case "agent_exit_error":
		return protocol.RuleAgentExited
	}
	for _, item := range evidence {
		if item.TimedOut {
			return protocol.RuleVerifierTimedOut
		}
	}
	lower := strings.ToLower(reason)
	switch {
	case strings.Contains(lower, "protected path"):
		return protocol.RuleProtectedPathChanged
	case strings.Contains(lower, "symlink escapes"):
		return protocol.RuleFilesystemEscape
	case strings.Contains(lower, "trusted base changed"):
		return protocol.RuleTrustedBaseChanged
	case strings.Contains(lower, "changed during"):
		return protocol.RuleCandidateMutated
	case strings.Contains(lower, "tree could not be reproduced") || strings.Contains(lower, "tree does not match"):
		return protocol.RuleCheckpointMismatch
	case strings.Contains(lower, "recertification failed"):
		return protocol.RuleRecertificationFailed
	case strings.Contains(lower, "candidate budget exhausted"):
		return protocol.RuleCandidateBudgetExhausted
	case strings.Contains(lower, "no progress") || strings.Contains(lower, "oscillat"):
		return protocol.RuleNoProgress
	case strings.Contains(lower, "no deliverable change") || strings.Contains(lower, "matches the trusted base"):
		return protocol.RuleNoDeliverableChange
	case strings.Contains(lower, "check failed") || strings.Contains(lower, "checks failed"):
		return protocol.RuleVerifierFailed
	case verdict == protocol.VerdictAbstained && reason != "":
		return protocol.RuleVerifierUnavailable
	default:
		return ""
	}
}

func cloneCheckpoint(cp *protocol.VerifiedCheckpoint) *protocol.VerifiedCheckpoint {
	if cp == nil {
		return nil
	}
	copy := *cp
	copy.AdmissionEvidence = append([]string(nil), cp.AdmissionEvidence...)
	return &copy
}

func evidenceIDs(es []protocol.EvidenceEnvelope) []string {
	ids := make([]string, len(es))
	for i := range es {
		ids[i] = es[i].EvidenceID
	}
	return ids
}
func patchDigest(root string) string {
	out, _ := identity.Git(root, "diff", "--binary", "HEAD")
	return identity.Digest(out)
}
func exportWorkspaceReceipt(root string, r protocol.CompletionReceipt) {
	dir := filepath.Join(root, ".stateseal", "receipts")
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, r.ReceiptID+".json"), append(b, '\n'), 0o644)
}

func InspectReceipt(path string) (protocol.CompletionReceipt, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return protocol.CompletionReceipt{}, err
	}
	var r protocol.CompletionReceipt
	if err := json.Unmarshal(b, &r); err != nil {
		return r, err
	}
	want := r.ReceiptDigest
	r.ReceiptDigest = ""
	got, _ := identity.JSONDigest(r)
	r.ReceiptDigest = want
	if want == "" || got != want {
		return r, fmt.Errorf("%s: receipt integrity check failed", protocol.RuleReceiptIntegrity)
	}
	return r, nil
}
