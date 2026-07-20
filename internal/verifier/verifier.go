package verifier

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/identity"
	processctl "github.com/hellogxp/stateseal/internal/process"
	"github.com/hellogxp/stateseal/pkg/protocol"
)

const maxCapturedOutput = 64 * 1024

func Run(root, candidateID, treeHash, policyHash string, checks []config.Check) ([]protocol.EvidenceEnvelope, error) {
	return RunPhaseWithBudget(root, candidateID, treeHash, policyHash, checks, "verification", 0)
}

func RunWithBudget(root, candidateID, treeHash, policyHash string, checks []config.Check, totalTimeout time.Duration) ([]protocol.EvidenceEnvelope, error) {
	return RunPhaseWithBudget(root, candidateID, treeHash, policyHash, checks, "verification", totalTimeout)
}

func RunPhaseWithBudget(root, candidateID, treeHash, policyHash string, checks []config.Check, phase string, totalTimeout time.Duration) ([]protocol.EvidenceEnvelope, error) {
	result := make([]protocol.EvidenceEnvelope, 0, len(checks))
	started := time.Now()
	for _, check := range checks {
		timeout := check.Timeout()
		if totalTimeout > 0 {
			remaining := totalTimeout - time.Since(started)
			if remaining <= 0 {
				_, cwdIdentity, _ := resolveWorkingDir(root, check.CWD)
				evidence := envelope(check, candidateID, treeHash, policyHash, cwdIdentity, time.Now().UTC(), 124, "verification suite wall budget exhausted", true)
				evidence.VerificationPhase = phase
				result = append(result, evidence)
				return result, nil
			}
			if remaining < timeout {
				timeout = remaining
			}
		}
		evidence, err := runOne(root, candidateID, treeHash, policyHash, check, timeout)
		evidence.VerificationPhase = phase
		result = append(result, evidence)
		if err != nil {
			return result, err
		}
		if evidence.ExitCode != 0 || evidence.TimedOut {
			return result, nil
		}
	}
	return result, nil
}

func runOne(root, candidateID, treeHash, policyHash string, check config.Check, timeout time.Duration) (protocol.EvidenceEnvelope, error) {
	started := time.Now().UTC()
	working, cwdIdentity, cwdErr := resolveWorkingDir(root, check.CWD)
	if cwdErr != nil {
		return envelope(check, candidateID, treeHash, policyHash, cwdIdentity, started, 126, cwdErr.Error(), false), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, check.Command[0], check.Command[1:]...)
	cmd.Dir = working
	cmd.Env = cleanEnvironment()
	processctl.ConfigureGroup(cmd)
	output, err := os.CreateTemp("", "stateseal-verifier-*.log")
	if err != nil {
		return protocol.EvidenceEnvelope{}, err
	}
	outputPath := output.Name()
	defer os.Remove(outputPath)
	defer output.Close()
	cmd.Stdout, cmd.Stderr = output, output
	err = cmd.Run()
	_ = processctl.KillGroup(cmd)
	exit := 0
	if err != nil {
		var exitErr *exec.ExitError
		if ctx.Err() != nil {
			exit = 124
		} else if errors.As(err, &exitErr) {
			exit = exitErr.ExitCode()
		} else {
			exit = 127
		}
	}
	if err := output.Sync(); err != nil {
		return protocol.EvidenceEnvelope{}, err
	}
	raw, err := capturedOutput(output)
	if err != nil {
		return protocol.EvidenceEnvelope{}, err
	}
	return envelope(check, candidateID, treeHash, policyHash, cwdIdentity, started, exit, string(raw), ctx.Err() != nil), nil
}

func envelope(check config.Check, candidateID, treeHash, policyHash, cwdIdentity string, started time.Time, exit int, output string, timedOut bool) protocol.EvidenceEnvelope {
	commandHash, _ := identity.JSONDigest(check.Command)
	cwdHash := identity.Digest([]byte(cwdIdentity))
	envHash, _ := identity.JSONDigest(map[string]string{
		"goos": runtime.GOOS, "goarch": runtime.GOARCH,
		"path": filteredEnvironmentValue("PATH"), "shell": filteredEnvironmentValue("SHELL"),
		"local_dependency_metadata": localDependencyDigest(),
	})
	suiteHash := identity.Digest([]byte(treeHash + "\x00" + commandHash + "\x00" + cwdHash))
	raw := []byte(output)
	return protocol.EvidenceEnvelope{
		EvidenceID: identity.ID("ev"), CandidateID: candidateID, CodeTreeSHA256: treeHash,
		SuiteSHA256: suiteHash, CommandDigest: commandHash, CWDigest: cwdHash,
		EnvironmentDigest: envHash, PolicyDigest: policyHash,
		VerifierIdentity: "command/" + check.ID + "@v1", ExecutionID: identity.ID("exec"),
		VerifierLayer: check.CoverageLayer(), VerifierOrigin: check.Provenance(),
		StartedAt: started, FinishedAt: time.Now().UTC(), ExitCode: exit,
		ResultDigest: identity.Digest(raw), Output: strings.TrimSpace(output), TimedOut: timedOut,
	}
}

func resolveWorkingDir(root, configured string) (string, string, error) {
	if configured == "" {
		configured = "."
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", filepath.ToSlash(configured), err
	}
	working, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(configured)))
	if err != nil {
		return "", filepath.ToSlash(configured), err
	}
	rel, err := filepath.Rel(root, working)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", filepath.ToSlash(configured), fmt.Errorf("verifier cwd escapes repository")
	}
	info, err := os.Stat(working)
	if err != nil {
		return "", filepath.ToSlash(configured), fmt.Errorf("verifier cwd is unavailable: %w", err)
	}
	if !info.IsDir() {
		return "", filepath.ToSlash(configured), fmt.Errorf("verifier cwd is not a directory")
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", filepath.ToSlash(configured), err
	}
	realWorking, err := filepath.EvalSymlinks(working)
	if err != nil {
		return "", filepath.ToSlash(configured), err
	}
	realRel, err := filepath.Rel(realRoot, realWorking)
	if err != nil || realRel == ".." || strings.HasPrefix(realRel, ".."+string(filepath.Separator)) {
		return "", filepath.ToSlash(configured), fmt.Errorf("verifier cwd resolves outside repository")
	}
	return working, filepath.ToSlash(rel), nil
}

func cleanEnvironment() []string {
	result := make([]string, 0, len(os.Environ()))
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		upper := strings.ToUpper(key)
		if strings.Contains(upper, "TOKEN") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "CREDENTIAL") || strings.Contains(upper, "API_KEY") {
			continue
		}
		result = append(result, item)
	}
	return append(result, "GIT_TERMINAL_PROMPT=0", "PYTHONDONTWRITEBYTECODE=1", "PYTEST_ADDOPTS=-p no:cacheprovider")
}

func filteredEnvironmentValue(key string) string {
	for _, item := range cleanEnvironment() {
		name, value, ok := strings.Cut(item, "=")
		if ok && name == key {
			return value
		}
	}
	return ""
}

func capturedOutput(file *os.File) ([]byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() <= maxCapturedOutput {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		return io.ReadAll(file)
	}
	half := int64(maxCapturedOutput / 2)
	head := make([]byte, half)
	if _, err := file.ReadAt(head, 0); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	tail := make([]byte, half)
	if _, err := file.ReadAt(tail, info.Size()-half); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	marker := fmt.Sprintf("\n...[%d bytes elided]...\n", info.Size()-2*half)
	return append(append(head, marker...), tail...), nil
}

func localDependencyDigest() string {
	var metadata []byte
	seen := map[string]bool{}
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Base(entry) != ".bin" || filepath.Base(filepath.Dir(entry)) != "node_modules" {
			continue
		}
		lock := filepath.Join(filepath.Dir(entry), ".package-lock.json")
		if seen[lock] {
			continue
		}
		seen[lock] = true
		if raw, err := os.ReadFile(lock); err == nil {
			metadata = append(metadata, []byte(lock)...)
			metadata = append(metadata, 0)
			metadata = append(metadata, raw...)
		}
	}
	return identity.Digest(metadata)
}

func Passed(evidence []protocol.EvidenceEnvelope, expected int) bool {
	if len(evidence) != expected {
		return false
	}
	for _, e := range evidence {
		if e.ExitCode != 0 || e.TimedOut {
			return false
		}
	}
	return true
}
