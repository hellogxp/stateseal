package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hellogxp/stateseal/internal/buildinfo"
	"github.com/hellogxp/stateseal/internal/config"
	"github.com/hellogxp/stateseal/internal/i18n"
	"github.com/hellogxp/stateseal/internal/identity"
	"github.com/hellogxp/stateseal/internal/store"
	"github.com/hellogxp/stateseal/pkg/protocol"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

const stateSealMCPInstructions = "StateSeal is the verified-delivery authority for code-changing work in local Git projects. Use it for implementation, fixes, refactors, migrations, generated code, and tests; do not invoke it for explanation, search, planning, or other read-only work. For a code-changing request, call inspect_project before editing. If the project is enabled, call start_delivery and never edit the source workspace directly. If it is not enabled, present the concise verification contract and call enable_project; StateSeal itself elicits the user's native confirmation. After delivery, present changed files, checks, durations, exact code state, coverage, receipt, and residual risks. Then call apply_verified; StateSeal itself elicits a separate final acceptance. Never claim StateSeal verification without an admitted non-empty receipt, and never bypass a failed or rejected StateSeal delivery by editing the source workspace."

type mcpProjectInput struct {
	RepoPath string `json:"repo_path" jsonschema:"absolute path to the Git project currently open in the Agent desktop"`
}

type mcpEnableProjectInput struct {
	RepoPath   string `json:"repo_path" jsonschema:"absolute path to the Git project currently open in the Agent desktop"`
	SetupToken string `json:"setup_token" jsonschema:"exact setup token returned by inspect_project for the displayed verification contract"`
}

type mcpStartDeliveryInput struct {
	RepoPath string `json:"repo_path" jsonschema:"absolute path to the enabled Git project"`
	Goal     string `json:"goal" jsonschema:"the user's complete development goal in their original language"`
}

type mcpDeliveryInput struct {
	SessionID string `json:"session_id" jsonschema:"StateSeal Desktop session identifier returned by start_delivery"`
}

type mcpApplyInput struct {
	SessionID string `json:"session_id" jsonschema:"StateSeal Desktop session identifier returned by start_delivery"`
	ReceiptID string `json:"receipt_id" jsonschema:"exact admitted receipt identifier shown to the user"`
}

type mcpProjectInspection struct {
	Project              string            `json:"project"`
	RepoRoot             string            `json:"repo_root"`
	Enabled              bool              `json:"enabled"`
	ConfirmationRequired bool              `json:"confirmation_required"`
	Admission            []string          `json:"admission"`
	Completion           []string          `json:"completion"`
	Protected            []string          `json:"protected"`
	Execution            string            `json:"execution"`
	ResidualRisks        []string          `json:"residual_risks,omitempty"`
	VerifierProvenance   []mcpVerifierPlan `json:"verifier_provenance"`
	SetupToken           string            `json:"setup_token,omitempty"`
	PolicyDigest         string            `json:"policy_digest"`
	ConfigCommit         string            `json:"config_commit,omitempty"`
	NextAction           string            `json:"next_action"`
}

type mcpVerifierPlan struct {
	CheckID string `json:"check_id"`
	Phase   string `json:"phase"`
	Layer   string `json:"layer"`
	Origin  string `json:"origin"`
}

func mcpCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "mcp", Short: "Expose StateSeal verified delivery through Model Context Protocol"}
	var agent string
	serve := &cobra.Command{
		Use:   "serve",
		Short: "Run the local StateSeal MCP server over stdio",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !isSupportedAgent(agent) {
				return codedError{10, fmt.Errorf("unsupported host agent %q", agent)}
			}
			return newStateSealMCPServer(agent).Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
	serve.Flags().StringVar(&agent, "agent", "codex", "coding Agent used for isolated delivery")
	cmd.AddCommand(serve)
	return cmd
}

func newStateSealMCPServer(agent string) *mcp.Server {
	info := buildinfo.Current()
	server := mcp.NewServer(&mcp.Implementation{
		Name: "stateseal", Title: "StateSeal Verified Delivery", Version: info.Version,
		WebsiteURL: "https://github.com/hellogxp/stateseal",
	}, &mcp.ServerOptions{Instructions: stateSealMCPInstructions})
	if os.Getenv("STATESEAL_DESKTOP_MCP_CHILD") == "1" {
		return server
	}

	mcp.AddTool(server, &mcp.Tool{
		Name: "inspect_project", Title: "Inspect StateSeal project policy",
		Description: "Inspect the current Git project without modifying it. Use this first for every code-changing request so StateSeal can determine whether one-time project enablement is required and show the exact verification contract.",
		Annotations: mcpAnnotations(true, false, true, false),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpProjectInput) (*mcp.CallToolResult, mcpProjectInspection, error) {
		inspection, err := inspectMCPProject(input.RepoPath, agent)
		return nil, inspection, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "enable_project", Title: "Enable StateSeal for this project",
		Description: "Enable the exact verification contract previously returned by inspect_project. This is a one-time project change and must be shown through the Agent's native approval UI before execution.",
		Annotations: mcpAnnotations(false, false, true, false),
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpEnableProjectInput) (*mcp.CallToolResult, mcpProjectInspection, error) {
		inspection, err := inspectMCPProject(input.RepoPath, agent)
		if err != nil {
			return nil, mcpProjectInspection{}, err
		}
		if !inspection.Enabled {
			locale := i18n.Detect()
			message := integrationText(locale,
				fmt.Sprintf("Enable StateSeal for %s? Admission: %s. Completion: %s. Policy: %s.", inspection.Project, strings.Join(inspection.Admission, "; "), strings.Join(inspection.Completion, "; "), inspection.PolicyDigest),
				fmt.Sprintf("是否为项目 %s 启用 StateSeal 受控交付？Admission：%s。Completion：%s。策略：%s。", inspection.Project, strings.Join(inspection.Admission, "；"), strings.Join(inspection.Completion, "；"), inspection.PolicyDigest))
			if err := elicitMCPConfirmation(ctx, req, message); err != nil {
				return nil, mcpProjectInspection{}, err
			}
		}
		inspection, err = enableMCPProject(input.RepoPath, input.SetupToken, agent)
		return nil, inspection, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "start_delivery", Title: "Develop and verify in isolation",
		Description: "Run the user's goal through an isolated coding Agent and independent StateSeal evaluators. The source workspace is not modified. Return the exact receipt, changed-file count, checks, coverage, and residual risks for review.",
		Annotations: mcpAnnotations(false, false, false, true),
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpStartDeliveryInput) (*mcp.CallToolResult, desktopCommandResult, error) {
		result, err := startMCPDelivery(ctx, req, input, agent)
		return nil, result, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_delivery_status", Title: "Get StateSeal delivery status",
		Description: "Read durable StateSeal authority state for an active or interrupted Desktop delivery. This tool never changes project files.",
		Annotations: mcpAnnotations(true, false, true, false),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpDeliveryInput) (*mcp.CallToolResult, desktopCommandResult, error) {
		result, err := desktopSessionResult(input.SessionID)
		return nil, result, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "apply_verified", Title: "Apply the verified checkpoint",
		Description: "Apply only the exact admitted checkpoint bound to the supplied Desktop session and receipt. This modifies the user's Git branch and must always use the Agent's native approval UI.",
		Annotations: mcpAnnotations(false, true, false, false),
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpApplyInput) (*mcp.CallToolResult, desktopCommandResult, error) {
		if err := validateDesktopApplyRequest(input.SessionID, input.ReceiptID); err != nil {
			return nil, desktopCommandResult{}, err
		}
		locale := i18n.Detect()
		message := integrationText(locale,
			fmt.Sprintf("Accept and apply StateSeal receipt %s to the project? Only the exact verified checkpoint will be applied.", input.ReceiptID),
			fmt.Sprintf("是否接受并应用 StateSeal Receipt %s？只会应用与该凭证绑定的确切验证代码。", input.ReceiptID))
		if err := elicitMCPConfirmation(ctx, req, message); err != nil {
			return nil, desktopCommandResult{}, err
		}
		result, err := applyDesktopSession(input.SessionID, input.ReceiptID)
		return nil, result, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "reject_delivery", Title: "Reject the verified candidate",
		Description: "Reject a pending candidate and leave the user's source branch unchanged.",
		Annotations: mcpAnnotations(false, false, true, false),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpDeliveryInput) (*mcp.CallToolResult, desktopCommandResult, error) {
		result, err := rejectDesktopSession(input.SessionID)
		return nil, result, err
	})

	return server
}

func validateDesktopApplyRequest(sessionID, receiptID string) error {
	session, err := store.LoadDesktopSession(sessionID)
	if err != nil {
		return err
	}
	if session.Stage != store.DesktopStagePendingApply || session.Verdict != string(protocol.VerdictAdmitted) {
		return fmt.Errorf("Desktop session has no admitted checkpoint awaiting acceptance")
	}
	if receiptID == "" || receiptID != session.ReceiptID {
		return fmt.Errorf("receipt %q does not match the admitted Desktop session", receiptID)
	}
	return nil
}

func elicitMCPConfirmation(ctx context.Context, req *mcp.CallToolRequest, message string) error {
	result, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
		Mode: "form", Message: message,
		RequestedSchema: &jsonschema.Schema{Type: "object"},
	})
	if err != nil {
		return fmt.Errorf("native user confirmation is unavailable; no project state was changed: %w", err)
	}
	if result == nil || result.Action != "accept" {
		return fmt.Errorf("user did not confirm the StateSeal action; no project state was changed")
	}
	return nil
}

func mcpAnnotations(readOnly, destructive, idempotent, openWorld bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint: readOnly, DestructiveHint: boolPointer(destructive),
		IdempotentHint: idempotent, OpenWorldHint: boolPointer(openWorld),
	}
}

func boolPointer(value bool) *bool { return &value }

func inspectMCPProject(path, agent string) (mcpProjectInspection, error) {
	root, err := resolveMCPRepo(path)
	if err != nil {
		return mcpProjectInspection{}, err
	}
	policyPath := filepath.Join(root, "seal.yaml")
	missing := !fileExists(policyPath)
	policy, err := projectPolicyPreview(root, missing)
	if err != nil {
		return mcpProjectInspection{}, err
	}
	policyDigest, err := previewPolicyDigest(root, policy, missing)
	if err != nil {
		return mcpProjectInspection{}, err
	}
	settings, settingsErr := store.LoadProjectSettings(root)
	enabled := settingsErr == nil && settings.DesktopEnabled && settings.DesktopSurface == "mcp" &&
		desktopAgentEnabled(settings, agent) &&
		settings.DesktopPolicyDigest == policyDigest
	inspection := mcpProjectInspection{
		Project: filepath.Base(root), RepoRoot: root, Enabled: enabled,
		ConfirmationRequired: !enabled, Admission: checkCommandList(policy.Admission.Checks),
		Completion: checkCommandList(policy.Completion.Checks), Protected: append([]string(nil), policy.State.Protected...),
		Execution:     "isolated Git worktree plus fresh independent evaluator",
		ResidualRisks: append([]string(nil), policy.ResidualRisks...), PolicyDigest: policyDigest,
		VerifierProvenance: mcpVerifierPlans(policy),
	}
	if enabled {
		inspection.NextAction = "call start_delivery with the user's ordinary development goal; do not edit the source workspace directly"
		return inspection, nil
	}
	inspection.SetupToken = setupToken(root, policyDigest, agent)
	inspection.NextAction = "present this concise verification contract, then request native approval for enable_project using the exact setup_token"
	return inspection, nil
}

func mcpVerifierPlans(policy config.Policy) []mcpVerifierPlan {
	result := make([]mcpVerifierPlan, 0, len(policy.Admission.Checks)+len(policy.Completion.Checks))
	for _, phase := range []struct {
		name   string
		checks []config.Check
	}{{"admission", policy.Admission.Checks}, {"completion", policy.Completion.Checks}} {
		for _, check := range phase.checks {
			result = append(result, mcpVerifierPlan{
				CheckID: check.ID, Phase: phase.name, Layer: check.CoverageLayer(), Origin: check.Provenance(),
			})
		}
	}
	return result
}

func enableMCPProject(path, token, agent string) (mcpProjectInspection, error) {
	inspection, err := inspectMCPProject(path, agent)
	if err != nil {
		return mcpProjectInspection{}, err
	}
	if inspection.Enabled {
		inspection.NextAction = "project was already enabled; call start_delivery for the user's goal"
		return inspection, nil
	}
	if token == "" || token != inspection.SetupToken {
		return mcpProjectInspection{}, fmt.Errorf("verification contract changed or was not inspected; call inspect_project again")
	}
	if err := identity.CheckpointIdentity(inspection.RepoRoot); err != nil {
		return mcpProjectInspection{}, err
	}
	dirty, err := identity.Git(inspection.RepoRoot, "status", "--porcelain")
	if err != nil {
		return mcpProjectInspection{}, err
	}
	if len(bytes.TrimSpace(dirty)) > 0 {
		return mcpProjectInspection{}, fmt.Errorf("trusted base is dirty; commit or stash changes before enabling StateSeal")
	}

	policyPath := filepath.Join(inspection.RepoRoot, "seal.yaml")
	createdPolicy := !fileExists(policyPath)
	if createdPolicy {
		policy, _ := discoveredProjectPolicy(inspection.RepoRoot)
		policy.Task.Goal = "Runtime goals are supplied by StateSeal MCP."
		if err := config.Write(policyPath, policy); err != nil {
			return mcpProjectInspection{}, err
		}
		if err := identity.EnsureLocalExclude(inspection.RepoRoot, ".stateseal/"); err != nil {
			_ = os.Remove(policyPath)
			return mcpProjectInspection{}, err
		}
		if _, err := identity.Git(inspection.RepoRoot, "add", "--", "seal.yaml"); err != nil {
			_ = os.Remove(policyPath)
			return mcpProjectInspection{}, fmt.Errorf("stage StateSeal policy: %w", err)
		}
		if _, err := identity.Git(inspection.RepoRoot, "commit", "-m", "chore(stateseal): configure verified desktop delivery"); err != nil {
			_, _ = identity.Git(inspection.RepoRoot, "rm", "--cached", "--quiet", "--", "seal.yaml")
			_ = os.Remove(policyPath)
			return mcpProjectInspection{}, fmt.Errorf("commit StateSeal policy: %w", err)
		}
	}

	digest := rawPolicyDigest(inspection.RepoRoot)
	settings, _ := store.LoadProjectSettings(inspection.RepoRoot)
	settings.TrustedHookAutomation = false
	settings.DesktopEnabled = true
	settings.DesktopPolicyDigest = digest
	settings.DesktopSurface = "mcp"
	if !containsString(settings.DesktopAgents, agent) {
		settings.DesktopAgents = append(settings.DesktopAgents, agent)
	}
	if err := store.SaveProjectSettings(inspection.RepoRoot, settings); err != nil {
		return mcpProjectInspection{}, err
	}
	result, err := inspectMCPProject(inspection.RepoRoot, agent)
	if err != nil {
		return mcpProjectInspection{}, err
	}
	if head, headErr := identity.Git(inspection.RepoRoot, "rev-parse", "HEAD"); headErr == nil {
		result.ConfigCommit = strings.TrimSpace(string(head))
	}
	result.NextAction = "project enabled; call start_delivery for the user's original goal without requiring a StateSeal prefix"
	return result, nil
}

func startMCPDelivery(ctx context.Context, req *mcp.CallToolRequest, input mcpStartDeliveryInput, agent string) (desktopCommandResult, error) {
	root, err := resolveMCPRepo(input.RepoPath)
	if err != nil {
		return desktopCommandResult{}, err
	}
	if strings.TrimSpace(input.Goal) == "" {
		return desktopCommandResult{}, fmt.Errorf("development goal is required")
	}
	if err := ensureDesktopMCPSetup(root, agent); err != nil {
		return desktopCommandResult{}, err
	}
	sessionID := identity.ID("desktop")
	session := store.DesktopSession{
		SessionID: sessionID, Agent: agent, RepoRoot: root, Goal: strings.TrimSpace(input.Goal),
		TaskID: newTaskID(input.Goal, time.Now().UTC()), Stage: store.DesktopStageReady,
	}
	if err := store.SaveDesktopSession(session); err != nil {
		return desktopCommandResult{}, err
	}
	progress := newMCPProgressReporter(ctx, req)
	locale := i18n.Detect()
	progress(2, integrationText(locale,
		"StateSeal accepted the goal and bound it to durable Desktop authority state",
		"StateSeal 已接收目标，并绑定到持久化 Desktop 权威状态"))
	result, err := runDesktopSession(ctx, sessionID, func(event machineProgressEvent) {
		progress(0, event.Message)
	})
	if err != nil {
		return desktopCommandResult{}, err
	}
	progress(100, integrationText(locale,
		"StateSeal finished independent verification and bound the result to the exact code state",
		"StateSeal 已完成独立验证，并将结果绑定到确切代码状态"))
	return result, nil
}

func newMCPProgressReporter(ctx context.Context, req *mcp.CallToolRequest) func(float64, string) {
	token := req.Params.GetProgressToken()
	last := float64(0)
	return func(value float64, message string) {
		if token == nil {
			return
		}
		if value <= 0 {
			value = last + 5
		}
		if value > 100 {
			value = 100
		}
		if value <= last {
			value = last + 1
		}
		if value > 100 {
			value = 100
		}
		last = value
		_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
			ProgressToken: token, Progress: value, Total: 100, Message: message,
		})
	}
}

func desktopSessionResult(sessionID string) (desktopCommandResult, error) {
	session, err := store.LoadDesktopSession(sessionID)
	if err != nil {
		return desktopCommandResult{}, err
	}
	result := desktopCommandResult{
		SessionID: session.SessionID, Stage: session.Stage, Verdict: protocolVerdict(session.Verdict),
		TaskID: session.TaskID, ReceiptID: session.ReceiptID, Reason: session.LastError,
		NextAction: desktopSessionNextAction(session),
	}
	if state, stateErr := loadDesktopTaskState(session); stateErr == nil {
		result.Files = verifiedChangedFileNames(state)
		result.ChangedFiles = len(result.Files)
		result.Checks = desktopCheckResults(session.RepoRoot, state)
		result.ChecksPassed = len(result.Checks)
		result.CodeState = resultCodeState(state)
		result.RuleID = resultRuleID(state)
		result.Coverage = state.Coverage
		result.Branch = state.AppliedBranch
		if state.Receipt != nil {
			result.ResidualRisks = append([]string(nil), state.Receipt.ResidualRisks...)
			result.VerificationCoverage = state.Receipt.VerificationCoverage
			result.LivenessImpact = state.Receipt.LivenessImpact
		}
	}
	return result, nil
}

func protocolVerdict(value string) protocol.Verdict { return protocol.Verdict(value) }

func desktopSessionNextAction(session store.DesktopSession) string {
	switch session.Stage {
	case store.DesktopStagePendingApply:
		return "present the admitted evidence and request native approval for apply_verified"
	case store.DesktopStageApplied:
		return "the exact verified checkpoint is applied"
	case store.DesktopStageRejected:
		return "the candidate was rejected and the user branch is unchanged"
	case store.DesktopStageRunning:
		return "wait or call get_delivery_status again; do not edit the source workspace"
	default:
		return "report the current StateSeal state without claiming verified delivery"
	}
}

func resolveMCPRepo(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("repo_path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	root, err := identity.GitRoot(abs)
	if err != nil {
		return "", fmt.Errorf("open a Git project directory before using StateSeal: %w", err)
	}
	return filepath.Clean(root), nil
}

func previewPolicyDigest(root string, policy config.Policy, missing bool) (string, error) {
	if !missing {
		return rawPolicyDigest(root), nil
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}
	return identity.Digest(raw), nil
}

func setupToken(root, policyDigest, agent string) string {
	return identity.Digest([]byte("stateseal-desktop-enable\x00" + root + "\x00" + policyDigest + "\x00" + agent))
}

func checkCommandList(checks []config.Check) []string {
	commands := make([]string, 0, len(checks))
	for _, check := range checks {
		command := shellJoin(check.Command)
		if check.CWD != "" && check.CWD != "." {
			command = "cd " + shellQuote(check.CWD) + " && " + command
		}
		commands = append(commands, command)
	}
	return commands
}

func validateMCPServer(binary, agent string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "stateseal-doctor", Version: buildinfo.Current().Version}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command(binary, "mcp", "serve", "--agent", agent)}, nil)
	if err != nil {
		return fmt.Errorf("MCP initialize handshake: %w", err)
	}
	defer session.Close()
	if result := session.InitializeResult(); result == nil || !strings.Contains(result.Instructions, "code-changing") {
		return fmt.Errorf("MCP server did not advertise StateSeal routing instructions")
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("MCP tools handshake: %w", err)
	}
	required := map[string]bool{
		"inspect_project": false, "enable_project": false, "start_delivery": false,
		"get_delivery_status": false, "apply_verified": false, "reject_delivery": false,
	}
	for _, tool := range tools.Tools {
		if _, ok := required[tool.Name]; ok {
			required[tool.Name] = true
		}
	}
	for name, present := range required {
		if !present {
			return fmt.Errorf("MCP server is missing tool %s", name)
		}
	}
	return nil
}
