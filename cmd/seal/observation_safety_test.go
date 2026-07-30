package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDefaultProductSurfaceIsObservationOnly(t *testing.T) {
	root := newRoot()
	forbidden := map[string]bool{
		"run": true, "verify": true, "submit": true, "apply": true,
		"restore": true, "desktop": true, "mcp": true, "integrate": true,
		"init": true, "setup": true, "install": true, "status": true,
		"timeline": true, "diff": true, "explain": true, "inspect": true,
		"doctor": true,
	}
	for _, command := range root.Commands() {
		if forbidden[command.Name()] {
			t.Fatalf("intervention command %q is registered", command.Name())
		}
	}
}

func TestLegacyHooksAreInertForEveryAgent(t *testing.T) {
	for _, agent := range supportedAgents {
		t.Run(agent, func(t *testing.T) {
			command := newRoot()
			var stdout, stderr bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			command.SetIn(strings.NewReader(`{"hook_event_name":"Stop","stop_hook_active":false}`))
			command.SetArgs([]string{"adapter", agent, "hook", "Stop"})
			if err := command.Execute(); err != nil {
				t.Fatalf("legacy hook returned an error: %v", err)
			}
			output := strings.ToLower(stdout.String() + stderr.String())
			for _, forbidden := range []string{"block", "deny", "followup", "additionalcontext", "systemmessage"} {
				if strings.Contains(output, forbidden) {
					t.Fatalf("legacy hook emitted intervention signal %q: %s", forbidden, output)
				}
			}
		})
	}
}
