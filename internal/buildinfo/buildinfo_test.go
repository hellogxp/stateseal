package buildinfo

import "testing"

func TestCurrentAlwaysIncludesRuntimeIdentity(t *testing.T) {
	info := Current()
	if info.Version == "" || info.Commit == "" || info.BuildDate == "" {
		t.Fatalf("missing build identity: %+v", info)
	}
	if info.GoVersion == "" || info.Platform == "" {
		t.Fatalf("missing runtime identity: %+v", info)
	}
}

func TestCleanUsesFallbackForBlankValues(t *testing.T) {
	if got := clean("  ", "fallback"); got != "fallback" {
		t.Fatalf("clean returned %q", got)
	}
	if got := clean(" v0.1.0 ", "fallback"); got != "v0.1.0" {
		t.Fatalf("clean returned %q", got)
	}
}
