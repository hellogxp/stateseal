package verifier

import (
	"testing"

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
