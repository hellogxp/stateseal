package verifier

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hellogxp/stateseal/internal/config"
)

func TestTimeoutUsesStableExitCode(t *testing.T) {
	evidence, err := Run(t.TempDir(), "candidate", "tree", "policy", []config.Check{{
		ID: "timeout", Command: []string{"sh", "-c", "sleep 2"}, TimeoutSeconds: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 || !evidence[0].TimedOut || evidence[0].ExitCode != 124 {
		t.Fatalf("unexpected timeout evidence: %+v", evidence)
	}
}

func TestInvalidWorkingDirectoryIsDeterministicallyRejected(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	evidence, err := Run(root, "candidate", "tree", "policy", []config.Check{{
		ID: "cwd", Command: []string{"sh", "-c", "exit 0"}, CWD: "file.txt", TimeoutSeconds: 10,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 || evidence[0].ExitCode != 126 || !strings.Contains(evidence[0].Output, "not a directory") {
		t.Fatalf("unexpected cwd evidence: %+v", evidence)
	}
}

func TestWorkingDirectorySymlinkCannotEscapeRepository(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	evidence, err := Run(root, "candidate", "tree", "policy", []config.Check{{
		ID: "cwd-escape", Command: []string{"sh", "-c", "exit 0"}, CWD: "outside", TimeoutSeconds: 10,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 || evidence[0].ExitCode != 126 || !strings.Contains(evidence[0].Output, "outside repository") {
		t.Fatalf("cwd symlink escape was not rejected: %+v", evidence)
	}
}

func TestVerifierDoesNotExposeSecrets(t *testing.T) {
	t.Setenv("STATESEAL_TEST_SECRET", "must-not-leak")
	evidence, err := Run(t.TempDir(), "candidate", "tree", "policy", []config.Check{{
		ID: "secret", Command: []string{"sh", "-c", `test -z "$STATESEAL_TEST_SECRET"`}, TimeoutSeconds: 10,
	}})
	if err != nil || !Passed(evidence, 1) {
		t.Fatalf("secret was exposed to verifier: evidence=%+v err=%v", evidence, err)
	}
}

func TestVerifierKillsBackgroundProcessGroup(t *testing.T) {
	root := t.TempDir()
	evidence, err := Run(root, "candidate", "tree", "policy", []config.Check{{
		ID: "process-group", Command: []string{"sh", "-c", `sleep 30 & echo $! > child.pid`}, TimeoutSeconds: 10,
	}})
	if err != nil || !Passed(evidence, 1) {
		t.Fatalf("verifier command failed: evidence=%+v err=%v", evidence, err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "child.pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if err := syscall.Kill(pid, 0); err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("background verifier process %d survived", pid)
}

func TestCapturedOutputPreservesHeadAndTail(t *testing.T) {
	evidence, err := Run(t.TempDir(), "candidate", "tree", "policy", []config.Check{{
		ID: "output", Command: []string{"sh", "-c", `printf HEAD; yes x | head -c 70000; printf TAIL`}, TimeoutSeconds: 10,
	}})
	if err != nil || len(evidence) != 1 {
		t.Fatalf("run failed: evidence=%+v err=%v", evidence, err)
	}
	if !strings.HasPrefix(evidence[0].Output, "HEAD") || !strings.HasSuffix(evidence[0].Output, "TAIL") || !strings.Contains(evidence[0].Output, "bytes elided") {
		t.Fatalf("captured output did not preserve boundaries")
	}
}

func TestSuiteWallBudgetCapsChecks(t *testing.T) {
	started := time.Now()
	evidence, err := RunWithBudget(t.TempDir(), "candidate", "tree", "policy", []config.Check{
		{ID: "first", Command: []string{"sh", "-c", "true"}, TimeoutSeconds: 10},
		{ID: "second", Command: []string{"sh", "-c", "sleep 2"}, TimeoutSeconds: 10},
	}, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > time.Second || len(evidence) != 2 || evidence[1].ExitCode != 124 || !evidence[1].TimedOut {
		t.Fatalf("suite budget was not enforced: elapsed=%s evidence=%+v", time.Since(started), evidence)
	}
}
