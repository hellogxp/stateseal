package ui

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hellogxp/stateseal/pkg/protocol"
)

// StateSeal only reads Agent-owned transcripts and workspace metadata. It does
// not install hooks, proxy requests, alter prompts, or write back to the Agent.
type passiveSessionFile struct {
	Path    string
	ModTime time.Time
	Size    int64
}

type cachedPassiveSnapshot struct {
	ModTime  time.Time
	Size     int64
	Snapshot RunSnapshot
}

var passiveSnapshotCache = struct {
	sync.RWMutex
	entries map[string]cachedPassiveSnapshot
}{entries: map[string]cachedPassiveSnapshot{}}

type codexRecord struct {
	Timestamp time.Time       `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexSessionMeta struct {
	ID         string `json:"id"`
	SessionID  string `json:"session_id"`
	CWD        string `json:"cwd"`
	Originator string `json:"originator"`
}

type codexPayload struct {
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Namespace string          `json:"namespace"`
	CallID    string          `json:"call_id"`
	Arguments string          `json:"arguments"`
	Input     json.RawMessage `json:"input"`
	Output    json.RawMessage `json:"output"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
}

type passiveCall struct {
	Tool             string
	Command          string
	WorkingDirectory string
	StartedAt        time.Time
	StateRevision    int
	CheckIndex       int
	Mutation         bool
	MutationGrade    string
	Paths            []patchPath
}

type patchPath struct {
	Path      string
	Operation string
}

type workspaceObservation struct {
	StateID     string
	Branch      string
	Dirty       bool
	Changed     int
	Additions   int
	Deletions   int
	StatusPaths []string
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]+`),
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]+`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{12,}`),
	regexp.MustCompile(`(?i)((?:api[_-]?key|token|password|secret)\s*[=:]\s*)[^\s"']+`),
}

func loadPassiveSnapshots() ([]RunSnapshot, error) {
	if os.Getenv("STATESEAL_UI_SOURCE") == "legacy" {
		return nil, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil
	}
	paths, err := filepath.Glob(filepath.Join(home, ".codex", "sessions", "*", "*", "*", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	files := make([]passiveSessionFile, 0, len(paths))
	for _, path := range paths {
		info, statErr := os.Stat(path)
		if statErr == nil && !info.IsDir() {
			files = append(files, passiveSessionFile{Path: path, ModTime: info.ModTime(), Size: info.Size()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime.After(files[j].ModTime) })
	if len(files) > 40 {
		files = files[:40]
	}

	snapshots := make([]RunSnapshot, 0, len(files))
	observedWorkspaces := map[string]bool{}
	for _, file := range files {
		snapshot, parseErr := cachedCodexTranscript(file)
		if parseErr != nil || snapshot.Summary.ID == "" {
			continue
		}
		root := snapshot.Summary.RepositoryRoot
		if root != "" && !observedWorkspaces[root] {
			observedWorkspaces[root] = true
			enrichWithWorkspace(&snapshot, observeWorkspace(root), file.ModTime)
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

func cachedCodexTranscript(file passiveSessionFile) (RunSnapshot, error) {
	passiveSnapshotCache.RLock()
	entry, ok := passiveSnapshotCache.entries[file.Path]
	passiveSnapshotCache.RUnlock()
	if ok && entry.Size == file.Size && entry.ModTime.Equal(file.ModTime) {
		return entry.Snapshot, nil
	}
	snapshot, err := projectCodexTranscript(file)
	if err != nil {
		return RunSnapshot{}, err
	}
	passiveSnapshotCache.Lock()
	passiveSnapshotCache.entries[file.Path] = cachedPassiveSnapshot{ModTime: file.ModTime, Size: file.Size, Snapshot: snapshot}
	passiveSnapshotCache.Unlock()
	return snapshot, nil
}

func projectCodexTranscript(file passiveSessionFile) (RunSnapshot, error) {
	handle, err := os.Open(file.Path)
	if err != nil {
		return RunSnapshot{}, err
	}
	defer handle.Close()

	snapshot := RunSnapshot{}
	summary := RunSummary{
		Status:          "OBSERVING",
		EvidencePosture: "OBSERVING",
		Mode:            "read-only observation",
		UpdatedAt:       file.ModTime,
		Repository:      "workspace",
		Agent:           "Codex",
	}
	var events []protocol.Event
	var changes []ChangeRecord
	var checks []CheckRecord
	var artifacts []ArtifactRecord
	calls := map[string]passiveCall{}
	sequence := uint64(0)
	stateRevision := 0
	terminal := false
	unknownToolFailures := 0
	lastWorkingDirectory := ""

	addEvent := func(kind string, at time.Time, data map[string]any) {
		sequence++
		events = append(events, protocol.Event{Sequence: sequence, Type: kind, TaskID: summary.TaskID, Timestamp: at, Data: data})
	}

	scanner := bufio.NewScanner(handle)
	// Long-lived desktop sessions may contain a large compaction record on one
	// JSONL line. Accept it so later delivery events are not silently lost.
	scanner.Buffer(make([]byte, 64*1024), 128*1024*1024)
	for scanner.Scan() {
		var record codexRecord
		if json.Unmarshal(scanner.Bytes(), &record) != nil {
			continue
		}
		if record.Timestamp.After(summary.UpdatedAt) {
			summary.UpdatedAt = record.Timestamp
		}
		switch record.Type {
		case "session_meta":
			var meta codexSessionMeta
			if json.Unmarshal(record.Payload, &meta) != nil {
				continue
			}
			baseID := nonempty(meta.ID, meta.SessionID)
			pathDigest := sha256.Sum256([]byte(file.Path))
			summary.ID = fmt.Sprintf("%s.%x", baseID, pathDigest[:4])
			summary.TaskID = shortID(summary.ID)
			summary.RepositoryRoot = meta.CWD
			if base := filepath.Base(meta.CWD); base != "" && base != "." && base != string(filepath.Separator) {
				summary.Repository = base
			}
			summary.StartedAt = record.Timestamp
			summary.Agent = nonempty(meta.Originator, "Codex")
			addEvent("SESSION_OBSERVED", record.Timestamp, map[string]any{"source": "Codex transcript", "evidence_grade": "Observed"})
		case "event_msg":
			var payload codexPayload
			if json.Unmarshal(record.Payload, &payload) != nil {
				continue
			}
			switch payload.Type {
			case "task_started":
				terminal = false
				summary.AgentReportedComplete = false
			case "user_message":
				if isHumanPrompt(payload.Message) {
					summary.Goal = compact(redactSensitive(requestSummary(payload.Message)), 120)
					addEvent("REQUEST_OBSERVED", record.Timestamp, map[string]any{"evidence_grade": "Observed"})
				}
			case "task_complete":
				terminal = true
				summary.AgentReportedComplete = true
				addEvent("AGENT_REPORTED_COMPLETE", record.Timestamp, map[string]any{"evidence_grade": "Observed", "semantic_correctness": "not asserted"})
			}
		case "response_item":
			var payload codexPayload
			if json.Unmarshal(record.Payload, &payload) != nil {
				continue
			}
			switch payload.Type {
			case "function_call", "custom_tool_call":
				input := toolInput(payload)
				tool := nonempty(payload.Name, payload.Namespace)
				if tool == "" {
					tool = "tool"
				}
				callID := payload.CallID
				if callID == "" {
					callID = fmt.Sprintf("call-%d", summary.ToolCalls+1)
				}
				command, workdir := extractCommand(input)
				if workdir != "" {
					lastWorkingDirectory = workdir
				}
				category, checkName := classifyCheck(command)
				paths := patchPaths(input)
				mutation, mutationGrade := classifyMutation(tool, command, paths)
				call := passiveCall{
					Tool: tool, Command: command, WorkingDirectory: workdir, StartedAt: record.Timestamp,
					StateRevision: stateRevision, CheckIndex: -1, Mutation: mutation,
					MutationGrade: mutationGrade, Paths: paths,
				}
				if category != "" {
					call.CheckIndex = len(checks)
					checks = append(checks, CheckRecord{
						ID: "check-" + strconv.Itoa(len(checks)+1), Name: checkName, Category: category,
						Command: compact(redactSensitive(command), 220), WorkingDirectory: workdir,
						Status: "RUNNING", Freshness: "UNKNOWN", StateRevision: stateRevision,
						StartedAt: record.Timestamp, Source: "Agent transcript", EvidenceGrade: "Observed",
					})
				}
				calls[callID] = call
				summary.ToolCalls++
				addEvent("TOOL_CALLED", record.Timestamp, map[string]any{
					"tool": tool, "category": nonempty(category, "tool"), "call_id": shortID(callID),
					"summary": safeToolSummary(input), "evidence_grade": "Observed",
				})
			case "function_call_output", "custom_tool_call_output":
				output := toolOutput(payload.Output)
				call, ok := calls[payload.CallID]
				if !ok {
					continue
				}
				exitCode, exitObserved := observedExitCode(output)
				failed := (exitObserved && exitCode != 0) || outputFailed(output)
				result := "completed"
				if failed {
					result = "reported failure"
				}
				if call.CheckIndex >= 0 && call.CheckIndex < len(checks) {
					check := &checks[call.CheckIndex]
					check.FinishedAt = record.Timestamp
					check.DurationMS = maxInt64(0, record.Timestamp.Sub(check.StartedAt).Milliseconds())
					check.ExitCode, check.ExitCodeObserved = exitCode, exitObserved
					check.Output = boundedOutput(redactSensitive(output))
					if failed {
						check.Status = "FAILED"
					} else if exitObserved {
						check.Status = "PASSED"
					} else {
						check.Status = "COMPLETED"
						check.EvidenceGrade = "Derived"
					}
				} else if failed {
					unknownToolFailures++
				}
				if call.Mutation && !failed {
					stateRevision++
					summary.MutationEvents++
					paths := call.Paths
					if len(paths) == 0 {
						paths = []patchPath{{Path: "workspace", Operation: "possibly modified"}}
					}
					for _, item := range paths {
						change := ChangeRecord{
							ID: "change-" + strconv.Itoa(len(changes)+1), Path: item.Path, Operation: item.Operation,
							StateRevision: stateRevision, Timestamp: record.Timestamp, Source: call.Tool,
							EvidenceGrade: call.MutationGrade,
						}
						changes = append(changes, change)
						if kind := artifactKind(item.Path); item.Operation == "added" && kind != "" {
							artifacts = append(artifacts, ArtifactRecord{
								ID: "artifact-" + strconv.Itoa(len(artifacts)+1), Path: item.Path, Kind: kind,
								Timestamp: record.Timestamp, Source: call.Tool, EvidenceGrade: call.MutationGrade,
							})
						}
					}
					addEvent("WORKSPACE_CHANGE_OBSERVED", record.Timestamp, map[string]any{
						"state_revision": stateRevision, "files": len(paths), "evidence_grade": call.MutationGrade,
					})
				}
				addEvent("TOOL_RESULT_OBSERVED", record.Timestamp, map[string]any{
					"tool": call.Tool, "call_id": shortID(payload.CallID), "result": result, "evidence_grade": "Observed",
				})
				delete(calls, payload.CallID)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return RunSnapshot{}, err
	}
	if summary.ID == "" {
		return RunSnapshot{}, nil
	}
	if summary.Goal == "" {
		summary.Goal = "Agent session " + summary.TaskID
	}
	if root := gitRootFor(lastWorkingDirectory); root != "" {
		summary.RepositoryRoot = root
		summary.Repository = filepath.Base(root)
	}
	normalizeRecordPaths(changes, artifacts, summary.RepositoryRoot)

	findings := analyzeDelivery(&summary, checks, stateRevision, terminal, unknownToolFailures)
	summary.ChangedFiles = uniqueChangedFiles(changes)
	summary.ChecksTotal = len(checks)
	summary.ArtifactsCount = len(artifacts)
	summary.FindingsCount = len(findings)
	summary.ToolFailures = unknownToolFailures
	for i := range checks {
		if checks[i].StateRevision < stateRevision {
			checks[i].Freshness = "STALE"
			summary.ChecksStale++
		} else {
			checks[i].Freshness = "CURRENT_BY_SEQUENCE"
		}
		switch checks[i].Status {
		case "PASSED":
			summary.ChecksPassed++
		case "FAILED":
			summary.ChecksFailed++
		}
	}

	snapshot.Summary = summary
	snapshot.Events = events
	snapshot.Changes = changes
	snapshot.Checks = checks
	snapshot.Artifacts = artifacts
	snapshot.Findings = findings
	snapshot.Nodes, snapshot.Edges = buildDeliveryGraph(summary, changes, checks, artifacts)
	return snapshot, nil
}

func analyzeDelivery(summary *RunSummary, checks []CheckRecord, finalRevision int, terminal bool, toolFailures int) []Finding {
	var findings []Finding
	add := func(code, severity, title, detail, basis, grade string, at time.Time) {
		findings = append(findings, Finding{
			ID: "finding-" + strconv.Itoa(len(findings)+1), Code: code, Severity: severity,
			Title: title, Detail: detail, Basis: basis, EvidenceGrade: grade, Timestamp: at,
		})
	}
	currentPass, currentFailure, stale := 0, 0, 0
	for _, check := range checks {
		if check.StateRevision < finalRevision {
			stale++
			continue
		}
		if check.Status == "PASSED" {
			currentPass++
		}
		if check.Status == "FAILED" {
			currentFailure++
			add("check_failed", "high", "Current-state check failed", check.Name+" reported a failure on the latest observed state.", check.ID, "Observed", check.FinishedAt)
		}
	}
	if stale > 0 {
		add("stale_evidence", "medium", "Checks precede later edits", fmt.Sprintf("%d check result(s) belong to an earlier observed state.", stale), "check state revision < final state revision", "Derived", summary.UpdatedAt)
	}
	if terminal && len(checks) == 0 {
		add("no_checks_observed", "medium", "No delivery checks observed", "The Agent reported completion, but the supported transcript contains no recognized test, build, lint, typecheck, or static-analysis run.", "completion event + zero recognized checks", "Derived", summary.UpdatedAt)
	}
	if terminal && finalRevision > 0 && currentPass == 0 && currentFailure == 0 && len(checks) > 0 {
		add("final_state_unchecked", "high", "Final observed state has no current check", "The workspace changed after the last recognized check and no later check was observed.", "latest change follows every recognized check", "Derived", summary.UpdatedAt)
	}
	if toolFailures > 0 {
		add("tool_failures", "low", "Tool failures were observed", fmt.Sprintf("%d non-check tool call(s) reported failure; inspect the timeline to see whether the Agent recovered.", toolFailures), "tool result records", "Observed", summary.UpdatedAt)
	}

	summary.AgentReportedComplete = terminal
	switch {
	case !terminal:
		summary.Status, summary.EvidencePosture = "OBSERVING", "OBSERVING"
	case currentFailure > 0:
		summary.Status, summary.EvidencePosture = "NEEDS_ATTENTION", "FAILURE"
	case finalRevision > 0 && currentPass == 0 && stale > 0:
		summary.Status, summary.EvidencePosture = "NEEDS_ATTENTION", "STALE"
	case len(checks) == 0 || (finalRevision > 0 && currentPass == 0):
		summary.Status, summary.EvidencePosture = "NEEDS_ATTENTION", "GAP"
	case currentPass > 0:
		summary.Status, summary.EvidencePosture = "AGENT_ENDED", "CURRENT"
	default:
		summary.Status, summary.EvidencePosture = "AGENT_ENDED", "UNKNOWN"
	}
	return findings
}

func buildDeliveryGraph(summary RunSummary, changes []ChangeRecord, checks []CheckRecord, artifacts []ArtifactRecord) ([]GraphNode, []GraphEdge) {
	nodes := []GraphNode{{
		ID: "request", Kind: "request", Label: "Delivery request", Subtitle: compact(summary.Goal, 54),
		Status: "observed", Timestamp: summary.StartedAt,
		Details: map[string]any{"evidence_grade": "Observed", "intervention": "none"},
	}, {
		ID: "state-0", Kind: "state", Label: "Workspace baseline", Subtitle: "Observed session start",
		Status: "observed", Timestamp: summary.StartedAt,
		Details: map[string]any{"state_revision": 0, "evidence_grade": "Derived"},
	}}
	edges := []GraphEdge{{ID: "edge-1", Source: "request", Target: "state-0", Label: "session started", Status: "observed"}}
	stateTimes := map[int]time.Time{}
	stateFiles := map[int]map[string]bool{}
	maxRevision := 0
	for _, change := range changes {
		maxRevision = maxInt(maxRevision, change.StateRevision)
		if stateTimes[change.StateRevision].IsZero() {
			stateTimes[change.StateRevision] = change.Timestamp
		}
		if stateFiles[change.StateRevision] == nil {
			stateFiles[change.StateRevision] = map[string]bool{}
		}
		stateFiles[change.StateRevision][change.Path] = true
	}
	previous := "state-0"
	visibleStart := 1
	if maxRevision > 3 {
		visibleStart = maxRevision - 2
		nodes = append(nodes, GraphNode{
			ID: "state-omitted", Kind: "state", Label: "Earlier observed activity",
			Subtitle: fmt.Sprintf("%d earlier code revision(s)", visibleStart-1), Status: "observed",
			Details: map[string]any{"evidence_grade": "Derived", "reason": "graph condensed; complete records remain below"},
		})
		edges = append(edges, GraphEdge{ID: fmt.Sprintf("edge-%d", len(edges)+1), Source: previous, Target: "state-omitted", Label: "condensed", Status: "observed"})
		previous = "state-omitted"
	}
	for revision := visibleStart; revision <= maxRevision; revision++ {
		id := fmt.Sprintf("state-%d", revision)
		nodes = append(nodes, GraphNode{
			ID: id, Kind: "state", Label: fmt.Sprintf("Code state S%d", revision),
			Subtitle: fmt.Sprintf("%d observed file change(s)", len(stateFiles[revision])), Status: "observed",
			Timestamp: stateTimes[revision], Details: map[string]any{
				"state_revision": revision, "files": sortedKeys(stateFiles[revision]),
				"evidence_grade": "Derived", "basis": "ordered mutation events in Agent transcript",
			},
		})
		edges = append(edges, GraphEdge{ID: fmt.Sprintf("edge-%d", len(edges)+1), Source: previous, Target: id, Label: "observed change", Status: "observed"})
		previous = id
	}
	visibleChecks := checks
	if len(visibleChecks) > 6 {
		visibleChecks = visibleChecks[len(visibleChecks)-6:]
	}
	for i, check := range visibleChecks {
		id := check.ID
		status := "observed"
		if check.Status == "FAILED" {
			status = "failed"
		} else if check.Freshness == "STALE" {
			status = "warning"
		} else if check.Status == "RUNNING" {
			status = "running"
		}
		nodes = append(nodes, GraphNode{
			ID: id, Kind: "check", Label: check.Name, Subtitle: check.Status + " · " + check.Freshness,
			Status: status, Timestamp: check.StartedAt, Details: map[string]any{
				"category": check.Category, "command": check.Command, "state_revision": check.StateRevision,
				"freshness": check.Freshness, "evidence_grade": check.EvidenceGrade,
			},
		})
		source := fmt.Sprintf("state-%d", check.StateRevision)
		if check.StateRevision > 0 && check.StateRevision < visibleStart {
			source = "state-omitted"
		}
		edges = append(edges, GraphEdge{ID: fmt.Sprintf("edge-check-%d", i+1), Source: source, Target: id, Label: "check executed", Status: status})
	}
	visibleArtifacts := artifacts
	if len(visibleArtifacts) > 4 {
		visibleArtifacts = visibleArtifacts[len(visibleArtifacts)-4:]
	}
	for i, artifact := range visibleArtifacts {
		nodes = append(nodes, GraphNode{
			ID: artifact.ID, Kind: "artifact", Label: filepath.Base(artifact.Path), Subtitle: artifact.Kind,
			Status: "observed", Timestamp: artifact.Timestamp,
			Details: map[string]any{"path": artifact.Path, "source": artifact.Source, "evidence_grade": artifact.EvidenceGrade},
		})
		edges = append(edges, GraphEdge{ID: fmt.Sprintf("edge-artifact-%d", i+1), Source: previous, Target: artifact.ID, Label: "produced", Status: "observed"})
	}
	if summary.AgentReportedComplete {
		claimStatus := "observed"
		if summary.EvidencePosture == "FAILURE" || summary.EvidencePosture == "GAP" || summary.EvidencePosture == "STALE" {
			claimStatus = "warning"
		}
		nodes = append(nodes, GraphNode{
			ID: "claim", Kind: "claim", Label: "Agent reported complete", Subtitle: "A claim, not an approval",
			Status: claimStatus, Timestamp: summary.UpdatedAt,
			Details: map[string]any{"evidence_grade": "Observed", "semantic_correctness": "not asserted", "intervention": "none"},
		})
		edges = append(edges, GraphEdge{ID: fmt.Sprintf("edge-%d", len(edges)+1), Source: previous, Target: "claim", Label: "reported outcome", Status: claimStatus})
	}
	return nodes, edges
}

func enrichWithWorkspace(snapshot *RunSnapshot, observation workspaceObservation, at time.Time) {
	if observation.StateID == "" {
		return
	}
	snapshot.Summary.CurrentState = observation.StateID
	snapshot.Summary.Branch = observation.Branch
	snapshot.Summary.WorkspaceDirty = observation.Dirty
	if observation.Changed > snapshot.Summary.ChangedFiles {
		snapshot.Summary.ChangedFiles = observation.Changed
	}
	snapshot.Summary.Additions = observation.Additions
	snapshot.Summary.Deletions = observation.Deletions
	if observation.Dirty && snapshot.Summary.MutationEvents == 0 {
		snapshot.Findings = append(snapshot.Findings, Finding{
			ID: "finding-workspace-gap", Code: "workspace_attribution_gap", Severity: "low",
			Title: "Workspace changes are not fully attributable", Detail: "Git reports current changes, but no supported mutation event was observed in this session.",
			Basis: "current Git status versus Agent transcript", EvidenceGrade: "Inferred", Timestamp: at,
		})
		snapshot.Summary.FindingsCount = len(snapshot.Findings)
	}
}

func observeWorkspace(root string) workspaceObservation {
	if root == "" {
		return workspaceObservation{}
	}
	git := func(args ...string) ([]byte, error) {
		return exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	}
	top, err := git("rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(string(top)) == "" {
		return workspaceObservation{}
	}
	head, err := git("rev-parse", "--short=12", "HEAD")
	if err != nil {
		return workspaceObservation{}
	}
	branch, _ := git("branch", "--show-current")
	status, _ := git("status", "--porcelain=v1", "-z", "--untracked-files=normal")
	diff, _ := git("diff", "--binary", "--no-ext-diff", "HEAD", "--")
	digest := sha256.Sum256(append(append([]byte(nil), status...), diff...))
	state := strings.TrimSpace(string(head))
	if len(status) > 0 {
		state += "+dirty:" + hex.EncodeToString(digest[:4])
	}
	paths := parseStatusPaths(status)
	additions, deletions := 0, 0
	if numstat, numErr := git("diff", "--numstat", "HEAD", "--"); numErr == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(numstat)), "\n") {
			parts := strings.Fields(line)
			if len(parts) < 3 {
				continue
			}
			if value, convErr := strconv.Atoi(parts[0]); convErr == nil {
				additions += value
			}
			if value, convErr := strconv.Atoi(parts[1]); convErr == nil {
				deletions += value
			}
		}
	}
	return workspaceObservation{
		StateID: state, Branch: strings.TrimSpace(string(branch)), Dirty: len(status) > 0,
		Changed: len(paths), Additions: additions, Deletions: deletions, StatusPaths: paths,
	}
}

func gitRootFor(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	output, err := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func normalizeRecordPaths(changes []ChangeRecord, artifacts []ArtifactRecord, root string) {
	normalize := func(path string) string {
		path = filepath.Clean(path)
		if root != "" && filepath.IsAbs(path) {
			if relative, err := filepath.Rel(root, path); err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return filepath.ToSlash(relative)
			}
		}
		if filepath.IsAbs(path) {
			return filepath.Base(path)
		}
		return filepath.ToSlash(path)
	}
	for i := range changes {
		changes[i].Path = normalize(changes[i].Path)
	}
	for i := range artifacts {
		artifacts[i].Path = normalize(artifacts[i].Path)
	}
}

func extractCommand(arguments string) (string, string) {
	var input map[string]any
	if json.Unmarshal([]byte(arguments), &input) == nil {
		command, _ := input["cmd"].(string)
		if command == "" {
			command, _ = input["command"].(string)
		}
		workdir, _ := input["workdir"].(string)
		return strings.TrimSpace(command), workdir
	}
	// Recent Codex builds wrap tool calls in a small JavaScript orchestration
	// program. Read only the explicit string literals; never evaluate it.
	command := jsStringProperty(arguments, "cmd")
	if command == "" {
		command = jsStringProperty(arguments, "command")
	}
	return strings.TrimSpace(command), jsStringProperty(arguments, "workdir")
}

func toolInput(payload codexPayload) string {
	if payload.Arguments != "" {
		return payload.Arguments
	}
	if len(payload.Input) == 0 {
		return ""
	}
	var value string
	if json.Unmarshal(payload.Input, &value) == nil {
		return value
	}
	return string(payload.Input)
}

func toolOutput(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var textValue string
	if json.Unmarshal(raw, &textValue) == nil {
		return textValue
	}
	var blocks []map[string]any
	if json.Unmarshal(raw, &blocks) == nil {
		var values []string
		for _, block := range blocks {
			if value, ok := block["text"].(string); ok {
				values = append(values, value)
			}
		}
		return strings.Join(values, "\n")
	}
	return string(raw)
}

func jsStringProperty(source, property string) string {
	pattern := regexp.MustCompile(`(?s)\b` + regexp.QuoteMeta(property) + `\s*:\s*("(?:\\.|[^"\\])*")`)
	match := pattern.FindStringSubmatch(source)
	if len(match) != 2 {
		return ""
	}
	value, err := strconv.Unquote(match[1])
	if err != nil {
		return ""
	}
	return value
}

func classifyCheck(command string) (string, string) {
	lower := strings.ToLower(command)
	patterns := []struct {
		category, name string
		needles        []string
	}{
		{"test", "Test suite", []string{"go test", "pytest", "python -m pytest", "npm test", "npm run test", "pnpm test", "yarn test", "cargo test", "mvn test", "gradle test", "ctest"}},
		{"build", "Build", []string{"go build", "npm run build", "pnpm build", "yarn build", "cargo build", "mvn package", "gradle build"}},
		{"lint", "Lint", []string{"golangci-lint", "eslint", "ruff check", "pylint", "shellcheck", "markdownlint"}},
		{"typecheck", "Type check", []string{"mypy", "pyright", "tsc --noemit", "tsc --noEmit"}},
		{"static-analysis", "Static analysis", []string{"go vet", "staticcheck", "semgrep", "codeql"}},
	}
	for _, item := range patterns {
		for _, needle := range item.needles {
			if strings.Contains(lower, strings.ToLower(needle)) {
				return item.category, item.name
			}
		}
	}
	return "", ""
}

func classifyMutation(tool, command string, paths []patchPath) (bool, string) {
	lowerTool, lowerCommand := strings.ToLower(tool), strings.ToLower(command)
	if len(paths) > 0 || strings.Contains(lowerTool, "apply_patch") || strings.Contains(lowerTool, "write_file") || strings.Contains(lowerTool, "edit_file") {
		return true, "Observed"
	}
	for _, marker := range []string{"gofmt -w", "prettier --write", "eslint --fix", "ruff format", "git commit", "git mv ", " mv ", " cp ", "touch "} {
		if strings.Contains(" "+lowerCommand, marker) {
			return true, "Derived"
		}
	}
	return false, ""
}

func patchPaths(arguments string) []patchPath {
	patch := arguments
	var input map[string]any
	if json.Unmarshal([]byte(arguments), &input) == nil {
		for _, key := range []string{"patch", "input"} {
			if value, ok := input[key].(string); ok && strings.TrimSpace(value) != "" {
				patch = value
				break
			}
		}
	}
	if embedded := jsAssignedString(arguments, "patch"); embedded != "" {
		patch = embedded
	}
	var paths []patchPath
	for _, line := range strings.Split(patch, "\n") {
		line = strings.TrimSpace(line)
		for prefix, operation := range map[string]string{
			"*** Add File:": "added", "*** Update File:": "modified", "*** Delete File:": "deleted",
		} {
			if strings.HasPrefix(line, prefix) {
				path := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, prefix)), `"`)
				if path != "" {
					paths = append(paths, patchPath{Path: path, Operation: operation})
				}
			}
		}
	}
	return paths
}

func jsAssignedString(source, variable string) string {
	pattern := regexp.MustCompile(`(?s)\b(?:const|let|var)\s+` + regexp.QuoteMeta(variable) + `\s*=\s*("(?:\\.|[^"\\])*")`)
	match := pattern.FindStringSubmatch(source)
	if len(match) != 2 {
		return ""
	}
	value, err := strconv.Unquote(match[1])
	if err != nil {
		return ""
	}
	return value
}

func observedExitCode(output string) (int, bool) {
	var value map[string]any
	if json.Unmarshal([]byte(output), &value) == nil {
		if number, ok := value["exit_code"].(float64); ok {
			return int(number), true
		}
	}
	match := regexp.MustCompile(`(?i)(?:exit[_ ]code)["']?\s*[:=]\s*(-?\d+)`).FindStringSubmatch(output)
	if len(match) == 2 {
		if code, err := strconv.Atoi(match[1]); err == nil {
			return code, true
		}
	}
	if strings.Contains(output, "Script completed") || strings.Contains(output, "Process exited with code 0") {
		return 0, true
	}
	return 0, false
}

func isHumanPrompt(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.HasPrefix(value, "<") && !strings.HasPrefix(value, "The following is the Codex agent history")
}

func requestSummary(value string) string {
	const marker = "## My request for Codex:"
	if index := strings.LastIndex(value, marker); index >= 0 {
		value = value[index+len(marker):]
	}
	return strings.TrimSpace(value)
}

func redactSensitive(value string) string {
	for _, pattern := range secretPatterns {
		value = pattern.ReplaceAllString(value, "[REDACTED]")
	}
	return value
}

func safeToolSummary(arguments string) string {
	var input map[string]any
	if json.Unmarshal([]byte(arguments), &input) == nil {
		if title, ok := input["title"].(string); ok && strings.TrimSpace(title) != "" {
			return compact(redactSensitive(title), 72)
		}
		if command, ok := input["cmd"].(string); ok && strings.TrimSpace(command) != "" {
			first := strings.Split(strings.TrimSpace(command), "\n")[0]
			return compact(redactSensitive(first), 72)
		}
		if path, ok := input["path"].(string); ok && path != "" {
			return "Path: " + filepath.Base(path)
		}
	}
	return "Arguments hidden by default"
}

func outputFailed(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "process exited with code 1") ||
		strings.Contains(lower, "\"exit_code\":1") ||
		strings.Contains(lower, "exit code: 1") ||
		strings.Contains(lower, "script failed")
}

func artifactKind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp":
		return "image"
	case ".pdf", ".docx", ".pptx", ".xlsx", ".md", ".tex", ".html":
		return "document"
	case ".json", ".xml", ".sarif", ".junit":
		return "report"
	case ".zip", ".tar", ".gz":
		return "archive"
	}
	return ""
}

func parseStatusPaths(raw []byte) []string {
	var paths []string
	for _, record := range strings.Split(string(raw), "\x00") {
		if len(record) < 4 {
			continue
		}
		paths = append(paths, strings.TrimSpace(record[3:]))
	}
	return paths
}

func uniqueChangedFiles(changes []ChangeRecord) int {
	paths := map[string]bool{}
	for _, change := range changes {
		paths[change.Path] = true
	}
	return len(paths)
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
