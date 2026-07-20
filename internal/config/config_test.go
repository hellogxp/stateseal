package config

import "testing"

func TestVerifierCoverageDefaultsRemainBackwardCompatible(t *testing.T) {
	check := Check{ID: "tests", Command: []string{"go", "test", "./..."}}
	if check.CoverageLayer() != "L1" || check.Provenance() != "project-policy" {
		t.Fatalf("unexpected legacy defaults: layer=%s origin=%s", check.CoverageLayer(), check.Provenance())
	}
}

func TestPolicyRejectsUnknownVerifierMetadata(t *testing.T) {
	policy := Default("invalid-metadata", []Check{{
		ID: "tests", Command: []string{"go", "test", "./..."}, Layer: "L4",
	}})
	if err := policy.Validate(); err == nil {
		t.Fatal("policy accepted an unknown verifier layer")
	}
	policy = Default("invalid-origin", []Check{{
		ID: "tests", Command: []string{"go", "test", "./..."}, Origin: "agent-claimed",
	}})
	if err := policy.Validate(); err == nil {
		t.Fatal("policy accepted an unknown verifier origin")
	}
}
