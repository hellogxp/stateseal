package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hellogxp/stateseal/internal/store"
	"github.com/hellogxp/stateseal/pkg/protocol"
)

type RunSummary struct {
	ID                  string    `json:"id"`
	TaskID              string    `json:"task_id"`
	Goal                string    `json:"goal"`
	Repository          string    `json:"repository"`
	RepositoryRoot      string    `json:"repository_root"`
	Status              string    `json:"status"`
	Mode                string    `json:"mode"`
	Disposition         string    `json:"disposition,omitempty"`
	RuleID              string    `json:"rule_id,omitempty"`
	UpdatedAt           time.Time `json:"updated_at"`
	StartedAt           time.Time `json:"started_at,omitempty"`
	CandidatesEvaluated int       `json:"candidates_evaluated"`
	CandidatesRejected  int       `json:"candidates_rejected"`
	CheckpointsVerified int       `json:"checkpoints_verified"`
	EvidenceCount       int       `json:"evidence_count"`
	Applied             bool      `json:"applied"`
	Recovered           bool      `json:"recovered"`
	IntegrityError      string    `json:"integrity_error,omitempty"`
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
	Summary  RunSummary                  `json:"summary"`
	Nodes    []GraphNode                 `json:"nodes"`
	Edges    []GraphEdge                 `json:"edges"`
	Events   []protocol.Event            `json:"events"`
	Evidence []protocol.EvidenceEnvelope `json:"evidence"`
	Receipt  *protocol.CompletionReceipt `json:"receipt,omitempty"`
}

func LoadSnapshots() ([]RunSnapshot, error) {
	// Passive Agent transcripts are the primary runtime source. Reading them is
	// side-effect free and does not require hooks, an MCP server, or an Agent
	// wrapper.
	passive, err := loadPassiveSnapshots()
	if err != nil {
		return nil, err
	}
	tasks, err := store.ListTasks()
	if err != nil {
		if len(passive) > 0 {
			return passive, nil
		}
		return nil, err
	}
	snapshots := make([]RunSnapshot, 0, len(passive)+len(tasks))
	snapshots = append(snapshots, passive...)
	for _, task := range tasks {
		legacy := project(task)
		legacy.Summary.Mode = "legacy evidence archive"
		legacy.Summary.Disposition = "historical record"
		switch legacy.Summary.Status {
		case "WORKING", "VERIFYING", "VERIFIED":
			legacy.Summary.Status = "ACTIVE"
		case "REJECTED", "ABSTAINED", "STALE", "ESCALATED":
			legacy.Summary.Status = "ATTENTION"
		default:
			legacy.Summary.Status = "COMPLETED"
		}
		snapshots = append(snapshots, legacy)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Summary.UpdatedAt.After(snapshots[j].Summary.UpdatedAt)
	})
	return snapshots, nil
}

func project(task store.StoredTask) RunSnapshot {
	state := task.State
	summary := RunSummary{
		ID:                  task.RepositoryID + "." + state.TaskID,
		TaskID:              state.TaskID,
		Goal:                state.Goal,
		Repository:          filepath.Base(state.RepoRoot),
		RepositoryRoot:      state.RepoRoot,
		Status:              state.Status,
		Mode:                state.Mode,
		Disposition:         state.Disposition,
		RuleID:              state.RuleID,
		UpdatedAt:           state.UpdatedAt,
		CandidatesEvaluated: state.CandidatesEvaluated,
		CandidatesRejected:  state.CandidatesRejected,
		CheckpointsVerified: state.CheckpointsVerified,
		EvidenceCount:       len(state.Evidence),
		Applied:             !state.AppliedAt.IsZero(),
		IntegrityError:      task.IntegrityError,
	}
	if summary.Goal == "" {
		summary.Goal = state.TaskID
	}
	if summary.Repository == "." || summary.Repository == string(filepath.Separator) || summary.Repository == "" {
		summary.Repository = "repository"
	}
	if summary.Applied {
		summary.Status = "APPLIED"
	}
	if state.Receipt != nil {
		summary.Recovered = state.Receipt.Recovered
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
