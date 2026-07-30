package ui

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hellogxp/stateseal/pkg/protocol"
)

// passiveSessionFile is intentionally small and schema-tolerant. StateSeal
// tails Agent-owned transcripts; it never installs a hook, proxies a request,
// or writes back to the Agent.
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
	Output    string          `json:"output"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
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
		if statErr != nil || info.IsDir() {
			continue
		}
		files = append(files, passiveSessionFile{Path: path, ModTime: info.ModTime(), Size: info.Size()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime.After(files[j].ModTime) })
	if len(files) > 40 {
		files = files[:40]
	}
	snapshots := make([]RunSnapshot, 0, len(files))
	for _, file := range files {
		snapshot, parseErr := cachedCodexTranscript(file)
		if parseErr != nil || snapshot.Summary.ID == "" {
			continue
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
	passiveSnapshotCache.entries[file.Path] = cachedPassiveSnapshot{
		ModTime: file.ModTime, Size: file.Size, Snapshot: snapshot,
	}
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
		Status:         "ACTIVE",
		Mode:           "read-only observation",
		Disposition:    "observed",
		UpdatedAt:      file.ModTime,
		Repository:     "workspace",
		IntegrityError: "",
	}
	nodes := []GraphNode{}
	edges := []GraphEdge{}
	events := []protocol.Event{}
	callNodes := map[string]int{}
	skills := map[string]string{}
	lastNode := "session"
	sequence := uint64(0)
	terminal := false
	failures := 0

	addEvent := func(kind string, at time.Time, data map[string]any) {
		sequence++
		events = append(events, protocol.Event{
			Sequence: sequence, Type: kind, TaskID: summary.TaskID, Timestamp: at, Data: data,
		})
	}
	addNode := func(node GraphNode, edgeLabel string) {
		nodes = append(nodes, node)
		if lastNode != "" && lastNode != node.ID {
			edges = append(edges, GraphEdge{
				ID: fmt.Sprintf("edge-%d", len(edges)+1), Source: lastNode, Target: node.ID,
				Label: edgeLabel, Status: node.Status,
			})
		}
		lastNode = node.ID
	}

	scanner := bufio.NewScanner(handle)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
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
			nodes = append(nodes, GraphNode{
				ID: "session", Kind: "session", Label: "Agent session",
				Subtitle: nonempty(meta.Originator, "Codex"), Status: "running", Timestamp: record.Timestamp,
				Details: map[string]any{
					"source": "Codex transcript", "workspace": summary.RepositoryRoot,
					"evidence_grade": "Observed", "intervention": "none",
				},
			})
		case "event_msg":
			var payload codexPayload
			if json.Unmarshal(record.Payload, &payload) != nil {
				continue
			}
			switch payload.Type {
			case "user_message":
				if summary.Goal == "" && isHumanPrompt(payload.Message) {
					summary.Goal = compact(redactPrompt(payload.Message), 96)
					if len(nodes) > 0 {
						nodes[0].Subtitle = compact(summary.Goal, 54)
					}
					addEvent("REQUEST_OBSERVED", record.Timestamp, map[string]any{"evidence_grade": "Observed"})
				}
			case "task_complete":
				terminal = true
				summary.Status = "COMPLETED"
				summary.Disposition = "reported complete by Agent"
				addEvent("SESSION_COMPLETED", record.Timestamp, map[string]any{"evidence_grade": "Observed"})
			}
		case "response_item":
			var payload codexPayload
			if json.Unmarshal(record.Payload, &payload) != nil {
				continue
			}
			switch payload.Type {
			case "function_call", "custom_tool_call":
				tool := nonempty(payload.Name, payload.Namespace)
				if tool == "" {
					tool = "tool"
				}
				for _, skill := range skillsFromArguments(payload.Arguments) {
					if _, exists := skills[skill]; exists {
						continue
					}
					nodeID := "skill-" + strconv.Itoa(len(skills)+1)
					skills[skill] = nodeID
					addNode(GraphNode{
						ID: nodeID, Kind: "skill", Label: skill, Subtitle: "Skill resource loaded",
						Status: "passed", Timestamp: record.Timestamp,
						Details: map[string]any{"evidence_grade": "Derived", "basis": "SKILL.md path observed in tool input"},
					}, "derived attribution")
					addEvent("SKILL_ATTRIBUTED", record.Timestamp, map[string]any{
						"skill": skill, "evidence_grade": "Derived",
					})
				}
				nodeID := "tool-" + strconv.Itoa(len(callNodes)+1)
				callNodes[payload.CallID] = len(nodes)
				addNode(GraphNode{
					ID: nodeID, Kind: "tool", Label: tool, Subtitle: safeToolSummary(payload.Arguments),
					Status: "running", Timestamp: record.Timestamp,
					Details: map[string]any{
						"call_id": shortID(payload.CallID), "evidence_grade": "Observed",
						"raw_arguments_retained": false,
					},
				}, "observed call")
				addEvent("TOOL_CALLED", record.Timestamp, map[string]any{
					"tool": tool, "call_id": shortID(payload.CallID), "evidence_grade": "Observed",
				})
				summary.CandidatesEvaluated++
			case "function_call_output", "custom_tool_call_output":
				index, ok := callNodes[payload.CallID]
				if !ok || index >= len(nodes) {
					continue
				}
				status := "passed"
				result := "completed"
				if outputFailed(payload.Output) {
					status = "failed"
					result = "reported failure"
					failures++
				}
				nodes[index].Status = status
				nodes[index].Details["result"] = result
				addEvent("TOOL_RESULT_OBSERVED", record.Timestamp, map[string]any{
					"call_id": shortID(payload.CallID), "result": result, "evidence_grade": "Observed",
				})
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
	if terminal {
		status := "passed"
		if failures > 0 {
			status = "warning"
			summary.Status = "ATTENTION"
			summary.Disposition = "completed with reported tool failures"
		}
		addNode(GraphNode{
			ID: "outcome", Kind: "outcome", Label: "Observed outcome",
			Subtitle: summary.Disposition, Status: status, Timestamp: summary.UpdatedAt,
			Details: map[string]any{
				"evidence_grade": "Observed", "semantic_correctness": "not asserted",
				"intervention": "none",
			},
		}, "reported result")
	}
	summary.EvidenceCount = len(events)
	summary.CheckpointsVerified = len(skills)
	summary.CandidatesRejected = failures
	snapshot.Summary = summary
	snapshot.Nodes = nodes
	snapshot.Edges = edges
	snapshot.Events = events
	return snapshot, nil
}

func isHumanPrompt(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.HasPrefix(value, "<") &&
		!strings.HasPrefix(value, "The following is the Codex agent history")
}

func redactPrompt(value string) string {
	fields := strings.Fields(value)
	for i, field := range fields {
		lower := strings.ToLower(field)
		if strings.Contains(lower, "github_pat_") || strings.Contains(lower, "sk-") ||
			strings.Contains(lower, "token=") || strings.Contains(lower, "password=") {
			fields[i] = "[REDACTED]"
		}
	}
	return strings.Join(fields, " ")
}

func skillsFromArguments(arguments string) []string {
	var found []string
	for _, token := range strings.FieldsFunc(arguments, func(r rune) bool {
		return r == '"' || r == '\'' || r == '\\' || r == ' ' || r == '\n'
	}) {
		if !strings.HasSuffix(token, "/SKILL.md") {
			continue
		}
		name := filepath.Base(filepath.Dir(token))
		if name != "" && name != "." {
			found = append(found, name)
		}
	}
	return found
}

func safeToolSummary(arguments string) string {
	var input map[string]any
	if json.Unmarshal([]byte(arguments), &input) == nil {
		if title, ok := input["title"].(string); ok && strings.TrimSpace(title) != "" {
			return compact(title, 54)
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
