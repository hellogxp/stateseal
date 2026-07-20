package main

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/i18n"
	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/pkg/protocol"
)

var progressHeartbeatInterval = 15 * time.Second

type progressSnapshot struct {
	Attempts    int
	Changed     int
	Checks      int
	StartedAt   time.Time
	CompletedAt time.Time
}

type runProgress struct {
	mu          sync.Mutex
	w           io.Writer
	locale      i18n.Locale
	enabled     bool
	heartbeat   bool
	startedAt   time.Time
	completedAt time.Time
	attempts    int
	changed     int
	checks      int
	stop        chan struct{}
	done        chan struct{}
}

func newRunProgress(w io.Writer, locale i18n.Locale, enabled, heartbeat bool) *runProgress {
	return &runProgress{w: w, locale: locale, enabled: enabled, heartbeat: heartbeat, startedAt: time.Now()}
}

func (p *runProgress) Header(goal, agent string, policy config.Policy) {
	if !p.enabled {
		return
	}
	fmt.Fprintf(p.w, "%s\n\n%s\n  %s\n\n%s\n", p.locale.T(i18n.ManagedTitle), p.locale.T(i18n.GoalLabel), goal, p.locale.T(i18n.PlanTitle))
	p.field(p.locale.T(i18n.AgentLabel), agent)
	p.field(p.locale.T(i18n.PermissionsLabel), p.locale.T(i18n.PermissionsValue))
	p.field(p.locale.T(i18n.IsolationLabel), p.locale.T(i18n.IsolationValue))
	p.field(p.locale.T(i18n.AdmissionLabel), checkSetSummary(policy.Admission.Checks))
	p.field(p.locale.T(i18n.CompletionLabel), checkSetSummary(policy.Completion.Checks))
	p.field(p.locale.T(i18n.AttemptsLabel), fmt.Sprint(policy.Budget.MaxCandidates))
	fmt.Fprintln(p.w, "────────────────────────────────────────")
}

func (p *runProgress) field(label, value string) {
	separator := ":"
	if p.locale.IsChinese() {
		separator = "："
	}
	fmt.Fprintf(p.w, "  %s%s %s\n", label, separator, value)
}

func (p *runProgress) WorkspaceReady() { p.step("✓", p.locale.T(i18n.WorkspaceReady)) }
func (p *runProgress) ProposalReady()  { p.step("✓", p.locale.T(i18n.ProposalReady)) }

func (p *runProgress) AgentStarted(agent, proposal, base string) {
	p.step("●", p.locale.T(i18n.AgentStarted, agent))
	if !p.enabled || !p.heartbeat {
		return
	}
	p.mu.Lock()
	p.stop, p.done = make(chan struct{}), make(chan struct{})
	stop, done := p.stop, p.done
	p.mu.Unlock()
	go func() {
		defer close(done)
		ticker := time.NewTicker(progressHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				changed := changedFileCount(proposal, base)
				p.mu.Lock()
				p.changed = changed
				p.writeLocked("●", p.locale.T(i18n.AgentHeartbeat, agent, changed))
				p.mu.Unlock()
			}
		}
	}()
}

func (p *runProgress) StopAgentHeartbeat() {
	p.mu.Lock()
	stop, done := p.stop, p.done
	p.stop, p.done = nil, nil
	p.mu.Unlock()
	if stop != nil {
		close(stop)
		<-done
	}
}

func (p *runProgress) Candidate(number, changed int, admission, completion []config.Check) {
	p.mu.Lock()
	p.attempts = number
	p.changed = changed
	p.writeLocked("◆", p.locale.T(i18n.CandidateReceived, number, changed))
	p.writeLocked("⟳", p.locale.T(i18n.AdmissionRunning, checkSetSummary(admission), checkSetSummary(completion)))
	p.mu.Unlock()
}

func (p *runProgress) CandidateResult(receipt protocol.CompletionReceipt, checks []config.Check, evidence []protocol.EvidenceEnvelope, elapsed time.Duration, agent string) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if receipt.Verdict == protocol.VerdictAdmitted {
		p.writeLocked("✓", p.locale.T(i18n.AdmissionPassed, formatDuration(elapsed)))
		p.writeEvidenceLocked(receipt, checks, evidence)
		p.writeLocked("✓", p.locale.T(i18n.CheckpointSaved))
		return
	}
	p.writeLocked("✗", p.locale.T(i18n.AdmissionFailed, compactFailure(receipt.Reason)))
	if receipt.Verdict == protocol.VerdictRejected || receipt.Verdict == protocol.VerdictAbstained {
		p.writeLocked("↩", p.locale.T(i18n.FeedbackSent, agent))
	}
}

func (p *runProgress) FinalStarted(checks []config.Check, changed int) {
	p.mu.Lock()
	if p.attempts == 0 {
		p.attempts = 1
	}
	p.changed = changed
	p.mu.Unlock()
	p.step("⟳", p.locale.T(i18n.FinalRunning, checkSetSummary(checks)))
}

func (p *runProgress) FinalResult(receipt protocol.CompletionReceipt, checks []config.Check, evidence []protocol.EvidenceEnvelope, elapsed time.Duration) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	if receipt.Verdict != protocol.VerdictAdmitted {
		p.writeLocked("✗", p.locale.T(i18n.FinalFailed, compactFailure(receipt.Reason)))
		p.mu.Unlock()
		return
	}
	p.checks = len(receipt.CompletionEvidence)
	p.writeLocked("✓", p.locale.T(i18n.FinalPassed, formatDuration(elapsed)))
	p.writeEvidenceLocked(receipt, checks, evidence)
	p.writeLocked("✓", p.locale.T(i18n.StateBound))
	p.mu.Unlock()
}

func (p *runProgress) Complete() progressSnapshot {
	p.StopAgentHeartbeat()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.completedAt.IsZero() {
		p.completedAt = time.Now()
	}
	return progressSnapshot{Attempts: p.attempts, Changed: p.changed, Checks: p.checks, StartedAt: p.startedAt, CompletedAt: p.completedAt}
}

func (p *runProgress) Snapshot() progressSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	completed := p.completedAt
	if completed.IsZero() {
		completed = time.Now()
	}
	return progressSnapshot{Attempts: p.attempts, Changed: p.changed, Checks: p.checks, StartedAt: p.startedAt, CompletedAt: completed}
}

func (p *runProgress) step(symbol, message string) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	p.writeLocked(symbol, message)
	p.mu.Unlock()
}

func (p *runProgress) writeLocked(symbol, message string) {
	if !p.enabled {
		return
	}
	fmt.Fprintf(p.w, "[%s] %s %s\n", formatElapsed(time.Since(p.startedAt)), symbol, message)
}

func (p *runProgress) writeEvidenceLocked(receipt protocol.CompletionReceipt, checks []config.Check, evidence []protocol.EvidenceEnvelope) {
	byID := make(map[string]protocol.EvidenceEnvelope, len(evidence))
	for _, item := range evidence {
		byID[item.EvidenceID] = item
	}
	for index, id := range receipt.CompletionEvidence {
		item, ok := byID[id]
		if !ok || item.ExitCode != 0 {
			continue
		}
		name := item.VerifierIdentity
		if index < len(checks) {
			name = shellJoin(checks[index].Command)
		}
		fmt.Fprintf(p.w, "         ✓ %s · %s\n", name, formatDuration(item.FinishedAt.Sub(item.StartedAt)))
	}
}

func changedFileCount(root, base string) int {
	files := map[string]bool{}
	if out, err := identity.Git(root, "diff", "--name-only", base); err == nil {
		for _, file := range strings.Fields(string(out)) {
			files[file] = true
		}
	}
	if out, err := identity.Git(root, "ls-files", "--others", "--exclude-standard"); err == nil {
		for _, file := range strings.Fields(string(out)) {
			files[file] = true
		}
	}
	return len(files)
}

func compactFailure(reason string) string {
	lines := strings.Split(strings.TrimSpace(reason), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "Failed verifier:") {
			continue
		}
		if len(line) > 160 {
			line = line[:157] + "..."
		}
		return line
	}
	return strings.TrimSpace(reason)
}

func formatElapsed(duration time.Duration) string {
	total := int(duration.Round(time.Second).Seconds())
	if total < 0 {
		total = 0
	}
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

func formatDuration(duration time.Duration) string {
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Round(time.Millisecond).Milliseconds())
	}
	if duration < time.Minute {
		return fmt.Sprintf("%.1fs", duration.Seconds())
	}
	return fmt.Sprintf("%dm%02ds", int(duration.Minutes()), int(duration.Seconds())%60)
}
