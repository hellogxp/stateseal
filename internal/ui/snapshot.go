package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hellogxp/stateseal/internal/store"
	"github.com/hellogxp/stateseal/pkg/protocol"
)

type RunSummary struct {
	ID                    string    `json:"id"`
	TaskID                string    `json:"task_id"`
	Goal                  string    `json:"goal"`
	Repository            string    `json:"repository"`
	RepositoryRoot        string    `json:"repository_root"`
	Branch                string    `json:"branch,omitempty"`
	Agent                 string    `json:"agent"`
	Status                string    `json:"status"`
	EvidencePosture       string    `json:"evidence_posture"`
	Mode                  string    `json:"mode"`
	UpdatedAt             time.Time `json:"updated_at"`
	StartedAt             time.Time `json:"started_at,omitempty"`
	AgentReportedComplete bool      `json:"agent_reported_complete"`
	ToolCalls             int       `json:"tool_calls"`
	ToolFailures          int       `json:"tool_failures"`
	MutationEvents        int       `json:"mutation_events"`
	ChangedFiles          int       `json:"changed_files"`
	Additions             int       `json:"additions"`
	Deletions             int       `json:"deletions"`
	ChecksTotal           int       `json:"checks_total"`
	ChecksPassed          int       `json:"checks_passed"`
	ChecksFailed          int       `json:"checks_failed"`
	ChecksStale           int       `json:"checks_stale"`
	ArtifactsCount        int       `json:"artifacts_count"`
	FindingsCount         int       `json:"findings_count"`
	CurrentState          string    `json:"current_state,omitempty"`
	WorkspaceDirty        bool      `json:"workspace_dirty"`
	IntegrityError        string    `json:"integrity_error,omitempty"`
}

type ChangeRecord struct {
	ID            string    `json:"id"`
	Path          string    `json:"path"`
	Operation     string    `json:"operation"`
	StateRevision int       `json:"state_revision"`
	Timestamp     time.Time `json:"timestamp"`
	Source        string    `json:"source"`
	EvidenceGrade string    `json:"evidence_grade"`
}

type CheckRecord struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Category         string    `json:"category"`
	Command          string    `json:"command"`
	WorkingDirectory string    `json:"working_directory,omitempty"`
	Status           string    `json:"status"`
	Freshness        string    `json:"freshness"`
	StateRevision    int       `json:"state_revision"`
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at,omitempty"`
	DurationMS       int64     `json:"duration_ms,omitempty"`
	ExitCode         int       `json:"exit_code,omitempty"`
	ExitCodeObserved bool      `json:"exit_code_observed"`
	Output           string    `json:"output,omitempty"`
	Source           string    `json:"source"`
	EvidenceGrade    string    `json:"evidence_grade"`
}

type ArtifactRecord struct {
	ID            string    `json:"id"`
	Path          string    `json:"path"`
	Kind          string    `json:"kind"`
	Timestamp     time.Time `json:"timestamp"`
	Source        string    `json:"source"`
	EvidenceGrade string    `json:"evidence_grade"`
}

type Finding struct {
	ID            string    `json:"id"`
	Code          string    `json:"code"`
	Severity      string    `json:"severity"`
	Title         string    `json:"title"`
	Detail        string    `json:"detail"`
	Basis         string    `json:"basis"`
	EvidenceGrade string    `json:"evidence_grade"`
	Timestamp     time.Time `json:"timestamp,omitempty"`
}

type GraphNode struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Label     string         `json:"label"`
	Subtitle  string         `json:"subtitle,omitempty"`
	Status    string         `json:"status"`
	Timestamp time.Time      `json:"timestamp,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

type GraphEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
	Status string `json:"status,omitempty"`
}

type RunSnapshot struct {
	Summary   RunSummary                  `json:"summary"`
	Nodes     []GraphNode                 `json:"nodes"`
	Edges     []GraphEdge                 `json:"edges"`
	Events    []protocol.Event            `json:"events"`
	Changes   []ChangeRecord              `json:"changes"`
	Checks    []CheckRecord               `json:"checks"`
	Artifacts []ArtifactRecord            `json:"artifacts"`
	Findings  []Finding                   `json:"findings"`
	Evidence  []protocol.EvidenceEnvelope `json:"evidence"`
	Receipt   *protocol.CompletionReceipt `json:"receipt,omitempty"`
}

func LoadSnapshots() ([]RunSnapshot, error) {
	// Passive Agent transcripts are the primary runtime source. Reading them is
	// side-effect free and does not require hooks, an MCP server, or an Agent
	// wrapper.
	passive, err := loadPassiveSnapshots()
	if err != nil {
		return nil, err
	}
	snapshots := append([]RunSnapshot(nil), passive...)
	// Legacy admission records are deliberately excluded from the supported
	// runtime view. They can be inspected explicitly for research archaeology,
	// but are never mixed with passive delivery observations.
	if os.Getenv("STATESEAL_INCLUDE_LEGACY") == "1" || os.Getenv("STATESEAL_UI_SOURCE") == "legacy" {
		tasks, listErr := store.ListTasks()
		if listErr != nil {
			if len(snapshots) > 0 {
				return snapshots, nil
			}
			return nil, listErr
		}
		for _, task := range tasks {
			legacy := project(task)
			legacy.Summary.Mode = "historical research archive"
			legacy.Summary.Agent = "legacy StateSeal prototype"
			legacy.Summary.Status = "ARCHIVED"
			legacy.Summary.EvidencePosture = "HISTORICAL"
			snapshots = append(snapshots, legacy)
		}
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Summary.UpdatedAt.After(snapshots[j].Summary.UpdatedAt)
	})
	return snapshots, nil
}

func project(task store.StoredTask) RunSnapshot {
	state := task.State
	summary := RunSummary{
		ID:              task.RepositoryID + "." + state.TaskID,
		TaskID:          state.TaskID,
		Goal:            state.Goal,
		Repository:      filepath.Base(state.RepoRoot),
		RepositoryRoot:  state.RepoRoot,
		Agent:           "legacy StateSeal prototype",
		Status:          "ARCHIVED",
		EvidencePosture: "HISTORICAL",
		Mode:            "historical research archive",
		UpdatedAt:       state.UpdatedAt,
		ToolCalls:       state.CandidatesEvaluated,
		ChecksTotal:     len(state.Evidence),
		ChecksPassed:    state.CheckpointsVerified,
		FindingsCount:   state.CandidatesRejected,
		IntegrityError:  task.IntegrityError,
	}
	if summary.Goal == "" {
		summary.Goal = state.TaskID
	}
	if summary.Repository == "." || summary.Repository == string(filepath.Separator) || summary.Repository == "" {
		summary.Repository = "repository"
	}
	if len(task.Events) > 0 {
		summary.StartedAt = task.Events[0].Timestamp
	}

	evidence := append([]protocol.EvidenceEnvelope(nil), state.Evidence...)
	for i := range evidence {
		evidence[i].Output = boundedOutput(evidence[i].Output)
	}
	snapshot := RunSnapshot{
		Summary:  summary,
		Events:   task.Events,
		Evidence: evidence,
		Receipt:  state.Receipt,
	}
	snapshot.Nodes, snapshot.Edges = projectGraph(state, task.Events, evidence, task.IntegrityError)
	return snapshot
}

func projectGraph(state protocol.TaskState, events []protocol.Event, evidence []protocol.EvidenceEnvelope, integrityError string) ([]GraphNode, []GraphEdge) {
	goalStatus := "running"
	if state.Receipt != nil {
		goalStatus = verdictStatus(string(state.Receipt.Verdict))
	}
	nodes := []GraphNode{{
		ID: "goal", Kind: "goal", Label: "Task intent", Subtitle: compact(state.Goal, 54), Status: goalStatus,
		Details: map[string]any{"task_id": state.TaskID, "goal": state.Goal, "repository": state.RepoRoot, "mode": state.Mode},
	}}
	var edges []GraphEdge
	nodeIDs := map[string]bool{"goal": true}
	candidateNodes := map[string]string{}
	checkpointNodes := map[string]string{}
	evidenceNodes := map[string]string{}
	lastCandidate := ""
	firstTimestamp := time.Time{}
	if len(events) > 0 {
		firstTimestamp = events[0].Timestamp
		nodes[0].Timestamp = firstTimestamp
	}
	addNode := func(node GraphNode) {
		if node.ID == "" || nodeIDs[node.ID] {
			return
		}
		nodeIDs[node.ID] = true
		nodes = append(nodes, node)
	}
	addEdge := func(source, target, label, status string) {
		if source == "" || target == "" {
			return
		}
		edges = append(edges, GraphEdge{
			ID: fmt.Sprintf("edge-%d", len(edges)+1), Source: source, Target: target, Label: label, Status: status,
		})
	}

	for _, event := range events {
		switch event.Type {
		case "CANDIDATE_SUBMITTED":
			candidateID := dataString(event.Data, "candidate_id")
			if candidateID == "" {
				candidateID = fmt.Sprintf("candidate-%d", event.Sequence)
			}
			nodeID := "candidate-" + candidateID
			status := "running"
			if state.Candidate != nil && state.Candidate.CandidateID == candidateID && state.Receipt != nil {
				status = verdictStatus(string(state.Receipt.Verdict))
			}
			addNode(GraphNode{ID: nodeID, Kind: "candidate", Label: "Candidate", Subtitle: shortID(candidateID), Status: status, Timestamp: event.Timestamp, Details: event.Data})
			addEdge("goal", nodeID, "proposed", status)
			candidateNodes[candidateID] = nodeID
			lastCandidate = nodeID
		case "PROTECTED_PATH_REJECTED":
			nodeID := fmt.Sprintf("guard-%d", event.Sequence)
			addNode(GraphNode{ID: nodeID, Kind: "guard", Label: "Policy guard", Subtitle: dataString(event.Data, "path"), Status: "failed", Timestamp: event.Timestamp, Details: event.Data})
			addEdge(nonempty(lastCandidate, "goal"), nodeID, "blocked", "failed")
		case "CANDIDATE_REJECTED":
			nodeID := fmt.Sprintf("rejection-%d", event.Sequence)
			source := candidateNodes[dataString(event.Data, "candidate_id")]
			addNode(GraphNode{ID: nodeID, Kind: "rejection", Label: "Candidate rejected", Subtitle: humanize(dataString(event.Data, "reason")), Status: "failed", Timestamp: event.Timestamp, Details: event.Data})
			addEdge(nonempty(source, nonempty(lastCandidate, "goal")), nodeID, "rejected", "failed")
		case "CHECKPOINT_VERIFIED":
			checkpointID := dataString(event.Data, "checkpoint_id")
			nodeID := "checkpoint-" + nonempty(checkpointID, fmt.Sprintf("%d", event.Sequence))
			candidateID := dataString(event.Data, "candidate_id")
			source := lastCandidate
			if candidateNode := candidateNodes[candidateID]; candidateNode != "" {
				source = candidateNode
			} else if state.Checkpoint != nil {
				source = nonempty(candidateNodes[state.Checkpoint.CandidateID], source)
			}
			addNode(GraphNode{ID: nodeID, Kind: "checkpoint", Label: "Verified checkpoint", Subtitle: shortID(checkpointID), Status: "passed", Timestamp: event.Timestamp, Details: event.Data})
			addEdge(nonempty(source, "goal"), nodeID, "admitted", "passed")
			checkpointNodes[checkpointID] = nodeID
		case "REGRESSION_DETECTED":
			nodeID := fmt.Sprintf("regression-%d", event.Sequence)
			addNode(GraphNode{ID: nodeID, Kind: "regression", Label: "Regression detected", Subtitle: humanize(dataString(event.Data, "reason")), Status: "failed", Timestamp: event.Timestamp, Details: event.Data})
			source := candidateNodes[dataString(event.Data, "terminal_candidate")]
			addEdge(nonempty(source, nonempty(lastCandidate, "goal")), nodeID, "regressed", "failed")
		case "CHECKPOINT_SELECTED":
			nodeID := fmt.Sprintf("selection-%d", event.Sequence)
			addNode(GraphNode{ID: nodeID, Kind: "selection", Label: "Checkpoint selected", Subtitle: humanize(dataString(event.Data, "reason")), Status: "warning", Timestamp: event.Timestamp, Details: event.Data})
			addEdge(findLatest(nodes, "regression", "checkpoint"), nodeID, "recover", "warning")
		case "CHECKPOINT_RESTORED":
			nodeID := fmt.Sprintf("restore-%d", event.Sequence)
			addNode(GraphNode{ID: nodeID, Kind: "restore", Label: "Checkpoint restored", Subtitle: shortID(dataString(event.Data, "checkpoint_id")), Status: "warning", Timestamp: event.Timestamp, Details: event.Data})
			addEdge(findLatest(nodes, "selection", "checkpoint"), nodeID, "materialize", "warning")
		case "COMPLETION_RECERTIFIED":
			nodeID := fmt.Sprintf("recertify-%d", event.Sequence)
			status := "passed"
			subtitle := "fresh completion evidence"
			if dataBool(event.Data, "recovered") {
				subtitle = "fresh evidence after recovery"
			}
			addNode(GraphNode{ID: nodeID, Kind: "recertification", Label: "Recertified", Subtitle: subtitle, Status: status, Timestamp: event.Timestamp, Details: event.Data})
			addEdge(findLatest(nodes, "restore", "checkpoint", "candidate"), nodeID, "fresh verify", status)
		}
	}

	for i, item := range evidence {
		nodeID := "evidence-" + nonempty(item.EvidenceID, fmt.Sprintf("%d", i))
		status := "passed"
		if item.TimedOut {
			status = "warning"
		} else if item.ExitCode != 0 {
			status = "failed"
		}
		label := strings.TrimSuffix(strings.TrimPrefix(item.VerifierIdentity, "command/"), "@v1")
		if label == "" {
			label = "Verifier"
		}
		addNode(GraphNode{
			ID: nodeID, Kind: "verifier", Label: label, Subtitle: humanize(item.VerificationPhase), Status: status, Timestamp: item.FinishedAt,
			Details: map[string]any{"evidence_id": item.EvidenceID, "phase": item.VerificationPhase, "exit_code": item.ExitCode, "timed_out": item.TimedOut, "duration_ms": item.FinishedAt.Sub(item.StartedAt).Milliseconds(), "result_digest": item.ResultDigest},
		})
		source := candidateNodes[item.CandidateID]
		addEdge(nonempty(source, nonempty(lastCandidate, "goal")), nodeID, humanize(item.VerificationPhase), status)
		evidenceNodes[item.EvidenceID] = nodeID
	}

	// A verified checkpoint is downstream of the admission evidence that
	// certified it, not parallel to that evidence. Rewire the latest checkpoint
	// using its typed evidence references while retaining backward-compatible
	// candidate edges for older state without those references.
	if state.Checkpoint != nil {
		checkpointNode := checkpointNodes[state.Checkpoint.CheckpointID]
		if checkpointNode != "" && len(state.Checkpoint.AdmissionEvidence) > 0 {
			var bound []string
			for _, evidenceID := range state.Checkpoint.AdmissionEvidence {
				if nodeID := evidenceNodes[evidenceID]; nodeID != "" {
					bound = append(bound, nodeID)
				}
			}
			if len(bound) > 0 {
				edges = removeEdgesTo(edges, checkpointNode)
				for _, source := range bound {
					addEdge(source, checkpointNode, "checkpoint", "passed")
				}
			}
		}
	}

	if state.Receipt != nil {
		status := verdictStatus(string(state.Receipt.Verdict))
		addNode(GraphNode{
			ID: "receipt", Kind: "receipt", Label: string(state.Receipt.Verdict), Subtitle: humanize(state.Receipt.Disposition), Status: status, Timestamp: state.Receipt.IssuedAt,
			Details: map[string]any{"receipt_id": state.Receipt.ReceiptID, "rule_id": state.Receipt.RuleID, "disposition": state.Receipt.Disposition, "reason": state.Receipt.Reason, "digest": state.Receipt.ReceiptDigest},
		})
		source := findLatest(nodes, "recertification", "checkpoint", "candidate", "goal")
		addEdge(source, "receipt", "decision", status)
		for _, evidenceID := range state.Receipt.CompletionEvidence {
			if nodeID := evidenceNodes[evidenceID]; nodeID != "" && nodeID != source {
				addEdge(nodeID, "receipt", "completion evidence", status)
			}
		}
	}
	if !state.AppliedAt.IsZero() {
		addNode(GraphNode{ID: "apply", Kind: "apply", Label: "Applied", Subtitle: shortID(state.AppliedCommit), Status: "passed", Timestamp: state.AppliedAt, Details: map[string]any{"commit": state.AppliedCommit, "branch": state.AppliedBranch}})
		addEdge(nonempty(findNode(nodes, "receipt"), findLatest(nodes, "checkpoint", "candidate", "goal")), "apply", "explicit apply", "passed")
	}
	if integrityError != "" {
		addNode(GraphNode{ID: "integrity", Kind: "integrity", Label: "Ledger integrity", Subtitle: "timeline unavailable", Status: "failed", Details: map[string]any{"error": integrityError}})
		addEdge("goal", "integrity", "audit", "failed")
	}
	return nodes, edges
}

func removeEdgesTo(edges []GraphEdge, target string) []GraphEdge {
	filtered := edges[:0]
	for _, edge := range edges {
		if edge.Target != target {
			filtered = append(filtered, edge)
		}
	}
	return filtered
}

func boundedOutput(output string) string {
	const limit = 16 * 1024
	if len(output) <= limit {
		return output
	}
	return "… output truncated …\n" + output[len(output)-limit:]
}

func compact(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	return value[:limit-1] + "…"
}

func shortID(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:8] + "…" + value[len(value)-4:]
}

func dataString(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return value
}

func dataBool(data map[string]any, key string) bool {
	value, _ := data[key].(bool)
	return value
}

func verdictStatus(value string) string {
	switch value {
	case "ADMITTED", "APPLIED":
		return "passed"
	case "REJECTED", "ESCALATED":
		return "failed"
	case "ABSTAINED", "STALE":
		return "warning"
	default:
		return "running"
	}
}

func humanize(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "_", " "))
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func nonempty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func findNode(nodes []GraphNode, id string) string {
	for _, node := range nodes {
		if node.ID == id {
			return id
		}
	}
	return ""
}

func findLatest(nodes []GraphNode, kinds ...string) string {
	for i := len(nodes) - 1; i >= 0; i-- {
		for _, kind := range kinds {
			if nodes[i].Kind == kind {
				return nodes[i].ID
			}
		}
	}
	return "goal"
}
