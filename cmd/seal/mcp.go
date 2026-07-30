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
	workspacepkg "github.com/hellogxp/stateseal/internal/workspace"
	"github.com/hellogxp/stateseal/pkg/protocol"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

const stateSealMCPInstructions = "StateSeal is the visible verified-delivery authority for code-changing work. Do not invoke it for explanation, search, planning, or other read-only work. For a code-changing request, call inspect_project with the user's complete goal before editing. StateSeal automatically routes a non-Git workspace to the relevant repository using the goal and read-only workspace evidence; ordinary users must not be asked to identify a Git repository or supply a path. Show the selected Workspace, Repository, routing evidence, Agent, execution mode, and delivery stages. If routing remains genuinely ambiguous, first use the Agent's existing context and read-only search to disambiguate. Only if product intent is still ambiguous, ask a natural-language project question and call inspect_project again with the inferred repository; never silently choose the first repository. If the project is excluded or StateSeal reports degraded or unavailable authority, continue with the Agent's normal workflow, clearly label the delivery UNVERIFIED, and never claim a StateSeal receipt. StateSeal infrastructure failures must not stop ordinary development. If the project is enabled, call start_delivery and do not edit the source workspace directly. If it is not enabled, present the verification contract and call enable_project. A healthy StateSeal verification rejection remains authoritative in enforce mode and must not be bypassed. After an admitted delivery, present changed files, checks, durations, exact code state, coverage, receipt, and residual risks, then call apply_verified. Final apply always requires explicit user approval; if approval is unavailable or declined, preserve the verified checkpoint and leave the source branch unchanged."

const (
	mcpConfirmationElicitation = "elicitation"
	mcpConfirmationHostTool    = "host-tool"
)

type mcpProjectInput struct {
	RepoPath   string `json:"repo_path" jsonschema:"absolute path to the workspace or Git project currently open in the Agent desktop"`
	Repository string `json:"repository,omitempty" jsonschema:"optional repository inferred by the Agent after read-only analysis when automatic routing remains ambiguous"`
	Goal       string `json:"goal,omitempty" jsonschema:"the user's complete development goal in their original language, used for automatic workspace routing"`
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
	Workspace            string                        `json:"workspace,omitempty"`
	Project              string                        `json:"project"`
	RepoRoot             string                        `json:"repo_root"`
	Repositories         []workspacepkg.Repository     `json:"repositories,omitempty"`
	RoutingCandidates    []workspacepkg.RouteCandidate `json:"routing_candidates,omitempty"`
	RoutingMethod        string                        `json:"routing_method,omitempty"`
	RoutingConfidence    string                        `json:"routing_confidence,omitempty"`
	RoutingEvidence      []string                      `json:"routing_evidence,omitempty"`
	Agent                string                        `json:"agent,omitempty"`
	Enabled              bool                          `json:"enabled"`
	Excluded             bool                          `json:"excluded"`
	ExclusionReason      string                        `json:"exclusion_reason,omitempty"`
	ExcludedAt           string                        `json:"excluded_at,omitempty"`
	ConfirmationRequired bool                          `json:"confirmation_required"`
	Admission            []string                      `json:"admission"`
	Completion           []string                      `json:"completion"`
	Protected            []string                      `json:"protected"`
	Execution            string                        `json:"execution"`
	ResidualRisks        []string                      `json:"residual_risks,omitempty"`
	VerifierProvenance   []mcpVerifierPlan             `json:"verifier_provenance"`
	SetupToken           string                        `json:"setup_token,omitempty"`
	PolicyDigest         string                        `json:"policy_digest"`
	ConfigCommit         string                        `json:"config_commit,omitempty"`
	AuthorityStatus      string                        `json:"authority_status,omitempty"`
	ReasonCode           string                        `json:"reason_code,omitempty"`
	Reason               string                        `json:"reason,omitempty"`
	Retryable            bool                          `json:"retryable,omitempty"`
	SafeState            *mcpSafeState                 `json:"safe_state,omitempty"`
	AllowedActions       []string                      `json:"allowed_actions,omitempty"`
	Unverified           bool                          `json:"unverified,omitempty"`
	NextAction           string                        `json:"next_action"`
}

type mcpSafeState struct {
	SourceWorkspaceChanged bool `json:"source_workspace_changed"`
	CandidatePreserved     bool `json:"candidate_preserved"`
}

type mcpVerifierPlan struct {
	CheckID string `json:"check_id"`
	Phase   string `json:"phase"`
	Layer   string `json:"layer"`
	Origin  string `json:"origin"`
}

func mcpCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "mcp", Short: "Expose StateSeal verified delivery through Model Context Protocol"}
	var agent, confirmation string
	serve := &cobra.Command{
		Use:   "serve",
		Short: "Run the local StateSeal MCP server over stdio",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !isSupportedAgent(agent) {
				return codedError{10, fmt.Errorf("unsupported host agent %q", agent)}
			}
			if confirmation != mcpConfirmationElicitation && confirmation != mcpConfirmationHostTool {
				return codedError{10, fmt.Errorf("unsupported confirmation mode %q", confirmation)}
			}
			return newStateSealMCPServerWithConfirmation(agent, confirmation).Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
	serve.Flags().StringVar(&agent, "agent", "codex", "coding Agent used for isolated delivery")
	serve.Flags().StringVar(&confirmation, "confirmation", mcpConfirmationElicitation, "user confirmation source: elicitation or host-tool")
	cmd.AddCommand(serve)
	return cmd
}

func newStateSealMCPServer(agent string) *mcp.Server {
	return newStateSealMCPServerWithConfirmation(agent, mcpConfirmationElicitation)
}

func newStateSealMCPServerWithConfirmation(agent, confirmation string) *mcp.Server {
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
		Description: "Inspect the current workspace without modifying it, automatically route the user's goal to the relevant repository using read-only evidence, and show the exact verification contract. Use this first for every code-changing request.",
		Annotations: mcpAnnotations(true, false, true, false),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpProjectInput) (*mcp.CallToolResult, mcpProjectInspection, error) {
		inspection, err := inspectMCPWorkspace(input.RepoPath, input.Repository, input.Goal, agent)
		if err != nil {
			return nil, degradedProjectInspectionForError(input.RepoPath, "PROJECT_INSPECTION_FAILED", err), nil
		}
		return nil, inspection, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "enable_project", Title: "Enable StateSeal for this project",
		Description: "Enable the exact verification contract previously returned by inspect_project. This is a one-time project change and must be shown through the Agent's native approval UI before execution.",
		Annotations: mcpAnnotations(false, true, true, false),
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpEnableProjectInput) (*mcp.CallToolResult, mcpProjectInspection, error) {
		inspection, err := inspectMCPProject(input.RepoPath, agent)
		if err != nil {
			return nil, degradedProjectInspectionForError(input.RepoPath, "PROJECT_INSPECTION_FAILED", err), nil
		}
		if inspection.Excluded {
			inspection.AuthorityStatus = "not_applicable"
			inspection.Unverified = true
			inspection.AllowedActions = []string{"continue_unverified", "include_project"}
			inspection.NextAction = "continue with the Agent's normal workflow and disclose that StateSeal is excluded; do not claim verified delivery"
			return nil, inspection, nil
		}
		if !inspection.Enabled && (input.SetupToken == "" || input.SetupToken != inspection.SetupToken) {
			return nil, mcpProjectInspection{}, fmt.Errorf("verification contract changed or was not inspected; call inspect_project again")
		}
		if !inspection.Enabled {
			locale := i18n.Detect()
			message := integrationText(locale,
				fmt.Sprintf("Enable StateSeal for %s? Admission: %s. Completion: %s. Policy: %s.", inspection.Project, strings.Join(inspection.Admission, "; "), strings.Join(inspection.Completion, "; "), inspection.PolicyDigest),
				fmt.Sprintf("是否为项目 %s 启用 StateSeal 受控交付？Admission：%s。Completion：%s。策略：%s。", inspection.Project, strings.Join(inspection.Admission, "；"), strings.Join(inspection.Completion, "；"), inspection.PolicyDigest))
			outcome := confirmMCPAction(ctx, req, confirmation, message)
			if outcome != "accepted" {
				return nil, degradedProjectInspection(inspection, outcome), nil
			}
		}
		inspection, err = enableMCPProject(input.RepoPath, input.SetupToken, agent)
		if err != nil {
			return nil, degradedProjectInspectionForError(input.RepoPath, "PROJECT_ENABLEMENT_FAILED", err), nil
		}
		return nil, inspection, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "start_delivery", Title: "Develop and verify in isolation",
		Description: "Run the user's goal through an isolated coding Agent and independent StateSeal evaluators. The source workspace is not modified. Return the exact receipt, changed-file count, checks, coverage, and residual risks for review.",
		Annotations: mcpAnnotations(false, false, false, true),
	}, func(ctx context.Context, req *mcp.CallToolRequest, input mcpStartDeliveryInput) (*mcp.CallToolResult, desktopCommandResult, error) {
		result, err := startMCPDelivery(ctx, req, input, agent)
		if err != nil {
			return nil, degradedDesktopResult(input.RepoPath, err), nil
		}
		return nil, result, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_delivery_status", Title: "Get StateSeal delivery status",
		Description: "Read durable StateSeal authority state for an active or interrupted Desktop delivery. This tool never changes project files.",
		Annotations: mcpAnnotations(true, false, true, false),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpDeliveryInput) (*mcp.CallToolResult, desktopCommandResult, error) {
		result, err := desktopSessionResult(input.SessionID)
		if err != nil {
			return nil, degradedDesktopResult("", err), nil
		}
		return nil, result, nil
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
		outcome := confirmMCPAction(ctx, req, confirmation, message)
		if outcome != "accepted" {
			result, err := desktopSessionResult(input.SessionID)
			if err != nil {
				return nil, desktopCommandResult{}, err
			}
			result.AuthorityStatus = "needs_user_action"
			result.ReasonCode = confirmationReasonCode(outcome)
			result.Retryable = outcome == "unavailable"
			result.SafeState = &mcpSafeState{SourceWorkspaceChanged: false, CandidatePreserved: true}
			result.AllowedActions = []string{"retry_apply", "reject_delivery", "continue_other_work"}
			result.NextAction = "keep the verified checkpoint pending and leave the source branch unchanged; the user may retry apply or reject it later"
			return nil, result, nil
		}
		result, err := applyDesktopSession(input.SessionID, input.ReceiptID)
		if err != nil {
			pending, statusErr := desktopSessionResult(input.SessionID)
			if statusErr != nil {
				return nil, degradedDesktopResult("", err), nil
			}
			pending.AuthorityStatus = "degraded"
			pending.ReasonCode = "APPLY_RUNTIME_FAILED"
			pending.Reason = err.Error()
			pending.Retryable = true
			pending.SafeState = &mcpSafeState{SourceWorkspaceChanged: false, CandidatePreserved: true}
			pending.AllowedActions = []string{"retry_apply", "reject_delivery", "diagnose"}
			pending.NextAction = "keep the verified checkpoint pending, leave the source branch unchanged, and offer retry or rejection"
			return nil, pending, nil
		}
		return nil, result, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "reject_delivery", Title: "Reject the verified candidate",
		Description: "Reject a pending candidate and leave the user's source branch unchanged.",
		Annotations: mcpAnnotations(false, false, true, false),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpDeliveryInput) (*mcp.CallToolResult, desktopCommandResult, error) {
		result, err := rejectDesktopSession(input.SessionID)
		if err != nil {
			return nil, degradedDesktopResult("", err), nil
		}
		return nil, result, nil
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

func confirmMCPAction(ctx context.Context, req *mcp.CallToolRequest, mode, message string) string {
	if mode == mcpConfirmationHostTool {
		return "accepted"
	}
	result, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
		Mode: "form", Message: message,
		RequestedSchema: &jsonschema.Schema{Type: "object"},
	})
	if err != nil {
		return "unavailable"
	}
	if result == nil || result.Action != "accept" {
		return "declined"
	}
	return "accepted"
}

func confirmationReasonCode(outcome string) string {
	if outcome == "declined" {
		return "USER_DECLINED_CONFIRMATION"
	}
	return "CLIENT_CONFIRMATION_UNAVAILABLE"
}

func degradedProjectInspection(inspection mcpProjectInspection, outcome string) mcpProjectInspection {
	inspection.AuthorityStatus = "degraded"
	inspection.ReasonCode = confirmationReasonCode(outcome)
	inspection.Retryable = outcome == "unavailable"
	inspection.SafeState = &mcpSafeState{SourceWorkspaceChanged: false, CandidatePreserved: false}
	inspection.AllowedActions = []string{"continue_unverified", "retry", "exclude_project"}
	inspection.Unverified = true
	inspection.NextAction = "continue with the Agent's normal development workflow, visibly label the result UNVERIFIED, and do not retry StateSeal automatically more than once"
	return inspection
}

func degradedProjectInspectionForError(path, reasonCode string, err error) mcpProjectInspection {
	return mcpProjectInspection{
		Workspace: path, AuthorityStatus: "degraded", ReasonCode: reasonCode, Reason: err.Error(),
		Retryable: true, Unverified: true,
		SafeState:      &mcpSafeState{SourceWorkspaceChanged: false, CandidatePreserved: false},
		AllowedActions: []string{"continue_unverified", "retry", "diagnose", "exclude_project"},
		NextAction:     "StateSeal could not establish delivery authority; continue with the Agent's normal workflow and visibly label the result UNVERIFIED",
	}
}

func degradedDesktopResult(repoPath string, err error) desktopCommandResult {
	return desktopCommandResult{
		Stage: "degraded", AuthorityStatus: "degraded", ReasonCode: "STATESEAL_AUTHORITY_UNAVAILABLE",
		Reason: err.Error(), Retryable: true, Unverified: true,
		SafeState:      &mcpSafeState{SourceWorkspaceChanged: false, CandidatePreserved: false},
		AllowedActions: []string{"continue_unverified", "retry", "diagnose", "exclude_project"},
		NextAction:     "StateSeal infrastructure is unavailable for " + repoPath + "; continue with the Agent's normal workflow and visibly label the result UNVERIFIED",
	}
}

func mcpAnnotations(readOnly, destructive, idempotent, openWorld bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint: readOnly, DestructiveHint: boolPointer(destructive),
		IdempotentHint: idempotent, OpenWorldHint: boolPointer(openWorld),
	}
}

func boolPointer(value bool) *bool { return &value }

func inspectMCPWorkspace(path, repository, goal, agent string) (mcpProjectInspection, error) {
	if strings.TrimSpace(path) == "" {
		return mcpProjectInspection{}, fmt.Errorf("repo_path is required")
	}
	routing, err := workspacepkg.Route(path, repository, goal)
	if err != nil {
		return mcpProjectInspection{}, err
	}
	workspace := routing.Inspection
	if len(workspace.Repositories) == 0 {
		return mcpProjectInspection{
			Workspace: workspace.Root, Repositories: workspace.Repositories,
			AuthorityStatus: "not_applicable", ReasonCode: "WORKSPACE_NO_GIT_REPOSITORIES",
			Unverified: true, AllowedActions: []string{"continue_unverified"},
			SafeState:  &mcpSafeState{SourceWorkspaceChanged: false, CandidatePreserved: false},
			NextAction: "StateSeal requires a Git repository for state-bound delivery; continue with the Agent's normal workflow and disclose that the result is UNVERIFIED",
		}, nil
	}
	if routing.Selected == nil {
		return mcpProjectInspection{
			Workspace: workspace.Root, Repositories: workspace.Repositories,
			RoutingCandidates: routing.Candidates, RoutingMethod: "ambiguous",
			AuthorityStatus: "needs_user_action", ReasonCode: "REPOSITORY_ROUTING_AMBIGUOUS",
			AllowedActions: []string{"analyze_workspace_read_only", "clarify_project_intent", "exclude_project"},
			SafeState:      &mcpSafeState{SourceWorkspaceChanged: false, CandidatePreserved: false},
			NextAction:     "use the Agent's current context and read-only workspace search to infer the project without asking for a Git path; only if product intent remains ambiguous, ask a natural-language project question and call inspect_project again with repository",
		}, nil
	}
	root := routing.Selected.Root
	inspection, err := inspectMCPProject(root, agent)
	if err != nil {
		return mcpProjectInspection{}, err
	}
	inspection.Workspace = workspace.Root
	inspection.Repositories = workspace.Repositories
	inspection.RoutingCandidates = routing.Candidates
	inspection.RoutingMethod = routing.Method
	inspection.RoutingConfidence = routing.Confidence
	inspection.RoutingEvidence = routing.Evidence
	return inspection, nil
}

func inspectMCPProject(path, agent string) (mcpProjectInspection, error) {
	root, err := resolveMCPRepo(path)
	if err != nil {
		return mcpProjectInspection{}, err
	}
	settings, settingsErr := store.LoadProjectSettings(root)
	if settingsErr != nil && !os.IsNotExist(settingsErr) {
		return mcpProjectInspection{}, settingsErr
	}
	if settings.IntegrationExcluded {
		return mcpProjectInspection{
			Project: filepath.Base(root), RepoRoot: root, Excluded: true,
			ExclusionReason: settings.IntegrationExclusionReason, ExcludedAt: settings.IntegrationExcludedAt,
			AuthorityStatus: "not_applicable", ReasonCode: "PROJECT_EXCLUDED", Unverified: true,
			SafeState:      &mcpSafeState{SourceWorkspaceChanged: false, CandidatePreserved: false},
			AllowedActions: []string{"continue_unverified", "include_project"},
			NextAction:     "automatic StateSeal routing is disabled for this project; do not call enable_project or start_delivery, and continue with the Agent's normal workflow",
		}, nil
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
	enabled := settingsErr == nil && settings.DesktopEnabled && settings.DesktopSurface == "mcp" &&
		desktopAgentEnabled(settings, agent) &&
		settings.DesktopPolicyDigest == policyDigest
	inspection := mcpProjectInspection{
		Project: filepath.Base(root), RepoRoot: root, Agent: agentDisplayName(agent), Enabled: enabled,
		ConfirmationRequired: !enabled, Admission: checkCommandList(policy.Admission.Checks),
		Completion: checkCommandList(policy.Completion.Checks), Protected: append([]string(nil), policy.State.Protected...),
		Execution:     "isolated Git worktree plus same-agent worker and fresh independent evaluator",
		ResidualRisks: append([]string(nil), policy.ResidualRisks...), PolicyDigest: policyDigest,
		VerifierProvenance: mcpVerifierPlans(policy),
	}
	if enabled {
		inspection.AuthorityStatus = "healthy"
		inspection.NextAction = "call start_delivery with the user's ordinary development goal; do not edit the source workspace directly"
		return inspection, nil
	}
	inspection.AuthorityStatus = "needs_enablement"
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
	if inspection.Excluded {
		return mcpProjectInspection{}, fmt.Errorf("project is excluded from automatic StateSeal integration: %s; run `seal integrate include %s` to allow fresh enablement", inspection.ExclusionReason, inspection.RepoRoot)
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
	changedPaths := make([]string, 0, 2)
	if createdPolicy {
		policy, _ := discoveredProjectPolicy(inspection.RepoRoot)
		policy.Task.Goal = "Runtime goals are supplied by StateSeal MCP."
		if err := config.Write(policyPath, policy); err != nil {
			return mcpProjectInspection{}, err
		}
		changedPaths = append(changedPaths, "seal.yaml")
	}
	if err := identity.EnsureLocalExclude(inspection.RepoRoot, ".stateseal/"); err != nil {
		if createdPolicy {
			_ = os.Remove(policyPath)
		}
		return mcpProjectInspection{}, err
	}

	adapterPath := agentAdapterPath(inspection.RepoRoot, agent)
	adapterRaw, adapterReadErr := os.ReadFile(adapterPath)
	adapterExisted := adapterReadErr == nil
	if adapterReadErr != nil && !os.IsNotExist(adapterReadErr) {
		if createdPolicy {
			_ = os.Remove(policyPath)
		}
		return mcpProjectInspection{}, adapterReadErr
	}
	if !adapterConfigured(inspection.RepoRoot, agent) {
		binary, binaryErr := sealExecutable()
		if binaryErr != nil {
			if createdPolicy {
				_ = os.Remove(policyPath)
			}
			return mcpProjectInspection{}, binaryErr
		}
		if _, adapterErr := installAgentAdapter(inspection.RepoRoot, agent, binary, false); adapterErr != nil {
			if createdPolicy {
				_ = os.Remove(policyPath)
			}
			return mcpProjectInspection{}, adapterErr
		}
		changedPaths = append(changedPaths, relativeDisplay(inspection.RepoRoot, adapterPath))
	}
	if len(changedPaths) > 0 {
		if _, err := identity.Git(inspection.RepoRoot, append([]string{"add", "--"}, changedPaths...)...); err != nil {
			rollbackMCPEnablement(policyPath, createdPolicy, adapterPath, adapterExisted, adapterRaw)
			return mcpProjectInspection{}, fmt.Errorf("stage StateSeal project integration: %w", err)
		}
		if _, err := identity.Git(inspection.RepoRoot, "commit", "-m", "chore(stateseal): configure verified desktop delivery"); err != nil {
			rollbackMCPEnablement(policyPath, createdPolicy, adapterPath, adapterExisted, adapterRaw)
			if createdPolicy {
				_, _ = identity.Git(inspection.RepoRoot, "rm", "--cached", "--quiet", "--ignore-unmatch", "--", "seal.yaml")
			}
			adapterRelative := relativeDisplay(inspection.RepoRoot, adapterPath)
			if adapterExisted {
				_, _ = identity.Git(inspection.RepoRoot, "add", "--", adapterRelative)
			} else {
				_, _ = identity.Git(inspection.RepoRoot, "rm", "--cached", "--quiet", "--ignore-unmatch", "--", adapterRelative)
			}
			return mcpProjectInspection{}, fmt.Errorf("commit StateSeal project integration: %w", err)
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

func rollbackMCPEnablement(policyPath string, createdPolicy bool, adapterPath string, adapterExisted bool, adapterRaw []byte) {
	if createdPolicy {
		_ = os.Remove(policyPath)
	}
	if adapterExisted {
		_ = os.WriteFile(adapterPath, adapterRaw, 0o644)
	} else {
		_ = os.Remove(adapterPath)
	}
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
	workspaceRoot := root
	if workspace, workspaceErr := workspacepkg.Inspect(input.RepoPath); workspaceErr == nil {
		workspaceRoot = workspace.Root
	}
	progress := newMCPProgressReporter(ctx, req)
	locale := i18n.Detect()
	progress(2, integrationText(locale,
		fmt.Sprintf("StateSeal engaged · Workspace: %s · Repository: %s · Agent: %s · Mode: isolated verified delivery", workspaceRoot, root, agentDisplayName(agent)),
		fmt.Sprintf("StateSeal 已介入 · Workspace：%s · Repository：%s · Agent：%s · Mode：隔离可信交付", workspaceRoot, root, agentDisplayName(agent))))
	result, err := runDesktopSession(ctx, sessionID, func(event machineProgressEvent) {
		progress(0, event.Message)
	})
	if err != nil {
		return desktopCommandResult{}, err
	}
	if result.Stage == store.DesktopStageFailed && result.Verdict == "" {
		result.Workspace = workspaceRoot
		result.Repository = root
		result.Agent = agentDisplayName(agent)
		result.ExecutionMode = "isolated verified delivery"
		result.AuthorityStatus = "degraded"
		result.ReasonCode = "STATESEAL_DELIVERY_RUNTIME_FAILED"
		result.Retryable = true
		result.Unverified = true
		result.SafeState = &mcpSafeState{SourceWorkspaceChanged: false, CandidatePreserved: false}
		result.AllowedActions = []string{"continue_unverified", "retry", "diagnose", "exclude_project"}
		result.NextAction = "StateSeal itself failed before producing a verification verdict; continue with the Agent's normal workflow and visibly label the result UNVERIFIED"
		return result, nil
	}
	result.Workspace = workspaceRoot
	result.Repository = root
	result.Agent = agentDisplayName(agent)
	result.ExecutionMode = "isolated verified delivery"
	result.DeliveryStages = []string{"create isolated candidate", "agent development", "independent verification", "receipt", "user-confirmed apply"}
	result.AuthorityStatus = "healthy"
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
		AuthorityStatus: "healthy",
		NextAction:      desktopSessionNextAction(session),
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
	root, err := workspacepkg.Resolve(path, "")
	if err != nil {
		return "", fmt.Errorf("select a Git repository for StateSeal: %w", err)
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
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command(binary, "mcp", "serve", "--agent", agent, "--confirmation", "host-tool")}, nil)
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
