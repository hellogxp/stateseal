package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hellogxp/stateseal/internal/store"
	"github.com/hellogxp/stateseal/pkg/protocol"
)

func TestRunsAPIProjectsTrustedStateAndGraph(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	taskStore, err := store.Open("/work/payments", "fix-callback")
	if err != nil {
		t.Fatal(err)
	}
	candidate := protocol.CandidateState{CandidateID: "cand_123", TaskID: "fix-callback", Source: "codex", CreatedAt: time.Now().UTC()}
	checkpoint := protocol.VerifiedCheckpoint{
		CheckpointID: "cp_123", CandidateID: candidate.CandidateID,
		AdmissionEvidence: []string{"ev_123"}, VerifiedAt: time.Now().UTC(),
	}
	evidence := protocol.EvidenceEnvelope{
		EvidenceID: "ev_123", CandidateID: candidate.CandidateID, VerifierIdentity: "command/unit@v1",
		VerificationPhase: "admission", StartedAt: time.Now().Add(-time.Second), FinishedAt: time.Now(), Output: "ok",
	}
	receipt := protocol.CompletionReceipt{ReceiptID: "rcpt_123", TaskID: "fix-callback", Verdict: protocol.VerdictAdmitted, Disposition: "ALLOWED", IssuedAt: time.Now().UTC()}
	if err := taskStore.Save(protocol.TaskState{
		Version: protocol.Version, TaskID: "fix-callback", Goal: "Fix duplicate callbacks", RepoRoot: "/work/payments",
		Mode: "enforce", Status: "ADMITTED", Candidate: &candidate, Checkpoint: &checkpoint, Receipt: &receipt,
		CandidatesEvaluated: 1, CheckpointsVerified: 1, Evidence: []protocol.EvidenceEnvelope{evidence},
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []protocol.Event{
		{Type: "TASK_CREATED", TaskID: "fix-callback"},
		{Type: "CANDIDATE_SUBMITTED", TaskID: "fix-callback", Data: map[string]any{"candidate_id": candidate.CandidateID}},
		{Type: "CHECKPOINT_VERIFIED", TaskID: "fix-callback", Data: map[string]any{"checkpoint_id": checkpoint.CheckpointID, "candidate_id": candidate.CandidateID}},
		{Type: "COMPLETION_ADMITTED", TaskID: "fix-callback", Data: map[string]any{"receipt_id": receipt.ReceiptID}},
	} {
		if _, err := taskStore.Append(event); err != nil {
			t.Fatal(err)
		}
	}
	server, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/runs", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/runs = %d: %s", recorder.Code, recorder.Body.String())
	}
	var summaries []RunSummary
	if err := json.Unmarshal(recorder.Body.Bytes(), &summaries); err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].Repository != "payments" || summaries[0].EvidenceCount != 1 {
		t.Fatalf("unexpected summaries: %+v", summaries)
	}

	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/runs/"+summaries[0].ID, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET run = %d: %s", recorder.Code, recorder.Body.String())
	}
	var snapshot RunSnapshot
	if err := json.Unmarshal(recorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Nodes) < 5 || len(snapshot.Edges) < 4 || snapshot.Receipt == nil {
		t.Fatalf("incomplete graph projection: nodes=%d edges=%d receipt=%+v", len(snapshot.Nodes), len(snapshot.Edges), snapshot.Receipt)
	}
	var evidenceToCheckpoint, candidateToCheckpoint bool
	for _, edge := range snapshot.Edges {
		if edge.Source == "evidence-ev_123" && edge.Target == "checkpoint-cp_123" {
			evidenceToCheckpoint = true
		}
		if edge.Source == "candidate-cand_123" && edge.Target == "checkpoint-cp_123" {
			candidateToCheckpoint = true
		}
	}
	if !evidenceToCheckpoint || candidateToCheckpoint {
		t.Fatalf("checkpoint provenance was not rewired through admission evidence: %+v", snapshot.Edges)
	}
	if snapshot.Evidence[0].Output != "ok" {
		t.Fatalf("evidence output changed unexpectedly: %q", snapshot.Evidence[0].Output)
	}
	if recorder.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("security headers were not applied")
	}
}

func TestRunsAPITruncatesLargeVerifierOutput(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	taskStore, err := store.Open("/work/repo", "large-output")
	if err != nil {
		t.Fatal(err)
	}
	output := strings.Repeat("x", 20*1024)
	if err := taskStore.Save(protocol.TaskState{
		TaskID: "large-output", RepoRoot: "/work/repo", Status: "REJECTED",
		Evidence: []protocol.EvidenceEnvelope{{EvidenceID: "ev", Output: output}},
	}); err != nil {
		t.Fatal(err)
	}
	snapshots, err := LoadSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	got := snapshots[0].Evidence[0].Output
	if len(got) >= len(output) || !strings.HasPrefix(got, "… output truncated …") {
		t.Fatalf("large output was not safely bounded: length=%d", len(got))
	}
}

func TestStaticConsoleIsEmbedded(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "StateSeal Runs") {
		t.Fatalf("embedded console unavailable: %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/locales/zh-CN.json", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"runs.title"`) {
		t.Fatalf("embedded locale catalog unavailable: %d %s", recorder.Code, recorder.Body.String())
	}
}
