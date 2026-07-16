package verifier

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/pkg/protocol"
)

const maxCapturedOutput = 64 * 1024

func Run(root, candidateID, treeHash, policyHash string, checks []config.Check) ([]protocol.EvidenceEnvelope, error) {
	result := make([]protocol.EvidenceEnvelope, 0, len(checks))
	for _, check := range checks {
		evidence, err := runOne(root, candidateID, treeHash, policyHash, check)
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

func runOne(root, candidateID, treeHash, policyHash string, check config.Check) (protocol.EvidenceEnvelope, error) {
	started := time.Now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), check.Timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, check.Command[0], check.Command[1:]...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	exit := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exit = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			exit = 124
		} else {
			exit = 127
		}
	}
	raw := output.Bytes()
	if len(raw) > maxCapturedOutput {
		raw = raw[len(raw)-maxCapturedOutput:]
	}
	commandHash, _ := identity.JSONDigest(check.Command)
	envHash, _ := identity.JSONDigest(map[string]string{
		"goos": runtime.GOOS, "goarch": runtime.GOARCH,
		"path": os.Getenv("PATH"), "shell": os.Getenv("SHELL"),
	})
	suiteHash := identity.Digest([]byte(treeHash + "\x00" + commandHash))
	return protocol.EvidenceEnvelope{
		EvidenceID: identity.ID("ev"), CandidateID: candidateID, CodeTreeSHA256: treeHash,
		SuiteSHA256: suiteHash, CommandDigest: commandHash, CWDigest: identity.Digest([]byte(root)),
		EnvironmentDigest: envHash, PolicyDigest: policyHash,
		VerifierIdentity: "command/" + check.ID + "@v1", ExecutionID: identity.ID("exec"),
		StartedAt: started, FinishedAt: time.Now().UTC(), ExitCode: exit,
		ResultDigest: identity.Digest(raw), Output: strings.TrimSpace(string(raw)), TimedOut: ctx.Err() != nil,
	}, nil
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
