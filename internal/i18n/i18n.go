package i18n

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/text/language"
)

type Locale string

const (
	English            Locale = "en"
	SimplifiedChinese  Locale = "zh-Hans"
	TraditionalChinese Locale = "zh-Hant"
)

type Key string

const (
	FirstRunIntro        Key = "first_run_intro"
	FirstRunConfirm      Key = "first_run_confirm"
	FirstRunCancelled    Key = "first_run_cancelled"
	FirstRunDirty        Key = "first_run_dirty"
	FirstRunInteractive  Key = "first_run_interactive"
	SetupComplete        Key = "setup_complete"
	SetupTitle           Key = "setup_title"
	PolicyCreated        Key = "policy_created"
	PolicyPreserved      Key = "policy_preserved"
	IntegrationReady     Key = "integration_ready"
	IdentityReady        Key = "identity_ready"
	BeforeFirstRun       Key = "before_first_run"
	StartTask            Key = "start_task"
	HookTrustConfirm     Key = "hook_trust_confirm"
	HookTrustInteractive Key = "hook_trust_interactive"
	ManagedTitle         Key = "managed_title"
	GoalLabel            Key = "goal_label"
	PlanTitle            Key = "plan_title"
	AgentLabel           Key = "agent_label"
	PermissionsLabel     Key = "permissions_label"
	IsolationLabel       Key = "isolation_label"
	AdmissionLabel       Key = "admission_label"
	CompletionLabel      Key = "completion_label"
	AttemptsLabel        Key = "attempts_label"
	PermissionsValue     Key = "permissions_value"
	IsolationValue       Key = "isolation_value"
	WorkspaceReady       Key = "workspace_ready"
	ProposalReady        Key = "proposal_ready"
	AgentStarted         Key = "agent_started"
	AgentHeartbeat       Key = "agent_heartbeat"
	CandidateReceived    Key = "candidate_received"
	AdmissionRunning     Key = "admission_running"
	AdmissionPassed      Key = "admission_passed"
	AdmissionFailed      Key = "admission_failed"
	FeedbackSent         Key = "feedback_sent"
	CheckpointSaved      Key = "checkpoint_saved"
	FinalRunning         Key = "final_running"
	FinalPassed          Key = "final_passed"
	FinalFailed          Key = "final_failed"
	StateBound           Key = "state_bound"
	ResultTitle          Key = "result_title"
	ResultPassed         Key = "result_passed"
	ResultRejected       Key = "result_rejected"
	ResultAbstained      Key = "result_abstained"
	ResultStatus         Key = "result_status"
	ChangedFilesLabel    Key = "changed_files_label"
	AgentAttemptsLabel   Key = "agent_attempts_label"
	ChecksLabel          Key = "checks_label"
	CoverageLabel        Key = "coverage_label"
	DurationLabel        Key = "duration_label"
	ReasonLabel          Key = "reason_label"
	DeliveryBasis        Key = "delivery_basis"
	BasisIsolated        Key = "basis_isolated"
	BasisExternal        Key = "basis_external"
	BasisExactState      Key = "basis_exact_state"
	BasisRecertified     Key = "basis_recertified"
	ResidualRiskLabel    Key = "residual_risk_label"
	ConfirmApply         Key = "confirm_apply"
	Applied              Key = "applied"
	NotApplied           Key = "not_applied"
	CoverageFull         Key = "coverage_full"
	CoverageTerminal     Key = "coverage_terminal"
	RiskConfiguredOnly   Key = "risk_configured_only"
	RiskHostUnattested   Key = "risk_host_unattested"
	RiskRemoteCI         Key = "risk_remote_ci"
	AutonomousNotice     Key = "autonomous_notice"
	GoalRequired         Key = "goal_required"
	AgentTimedOut        Key = "agent_timed_out"
	AgentExited          Key = "agent_exited"
)

var supported = []language.Tag{
	language.English, // The first language is the required fallback.
	language.SimplifiedChinese,
	language.TraditionalChinese,
}

var matcher = language.NewMatcher(supported)

// Detect follows the standard message-locale precedence used by gettext.
// Unsupported, empty, C, and POSIX locales fall back to English.
func Detect() Locale {
	var candidates []string
	if enabled := localeEnabled(); enabled {
		for _, item := range strings.Split(os.Getenv("LANGUAGE"), ":") {
			if normalized := normalize(item); normalized != "" {
				candidates = append(candidates, normalized)
			}
		}
	}
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if normalized := normalize(os.Getenv(key)); normalized != "" {
			candidates = append(candidates, normalized)
			break
		}
	}
	if len(candidates) == 0 {
		return English
	}
	tags := make([]language.Tag, 0, len(candidates))
	for _, candidate := range candidates {
		tags = append(tags, language.Make(candidate))
	}
	_, index, confidence := matcher.Match(tags...)
	if confidence == language.No || index < 0 || index >= len(supported) {
		return English
	}
	switch index {
	case 1:
		return SimplifiedChinese
	case 2:
		return TraditionalChinese
	default:
		return English
	}
}

func localeEnabled() bool {
	for _, key := range []string{"LC_ALL", "LANG"} {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" && !isEnglishFallbackLocale(value) {
			return true
		}
	}
	return false
}

func normalize(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || isEnglishFallbackLocale(value) {
		return ""
	}
	if index := strings.IndexAny(value, ".@"); index >= 0 {
		value = value[:index]
	}
	return strings.ReplaceAll(value, "_", "-")
}

func isEnglishFallbackLocale(value string) bool {
	value = strings.ToUpper(strings.TrimSpace(value))
	return value == "C" || value == "POSIX" || strings.HasPrefix(value, "C.")
}

func (l Locale) T(key Key, args ...any) string {
	template := catalogs[l][key]
	if template == "" {
		template = catalogs[English][key]
	}
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}

func (l Locale) IsChinese() bool {
	return l == SimplifiedChinese || l == TraditionalChinese
}

var catalogs = map[Locale]map[Key]string{
	English: {
		FirstRunIntro:        "First run will configure StateSeal:\n  • Detect project verification commands\n  • Merge the %s lifecycle integration\n  • Authorize the generated and validated StateSeal hook\n  • Create an auditable configuration commit",
		FirstRunConfirm:      "Continue? [Y/n] ",
		FirstRunCancelled:    "first-run setup was cancelled",
		FirstRunDirty:        "first-run setup requires a clean Git worktree; commit or stash existing changes",
		FirstRunInteractive:  "first-run setup needs confirmation; run in an interactive terminal or add --yes",
		SetupComplete:        "✓ StateSeal configured · Agent: %s · Verification: %s",
		SetupTitle:           "StateSeal project setup",
		PolicyCreated:        "project policy created: %s",
		PolicyPreserved:      "existing project policy preserved: %s",
		IntegrationReady:     "%s lifecycle integration ready: %s",
		IdentityReady:        "Git checkpoint identity ready",
		BeforeFirstRun:       "Before the first run, review and commit %s and %s.",
		StartTask:            "Start a task with: seal run \"Describe the intended outcome\"",
		HookTrustConfirm:     "Codex requires explicit project-hook trust. Allow StateSeal to load its validated hook during managed runs? [Y/n] ",
		HookTrustInteractive: "Codex project-hook trust requires confirmation; run interactively or add --yes",
		ManagedTitle:         "StateSeal · Verified development",
		GoalLabel:            "Goal",
		PlanTitle:            "Execution plan",
		AgentLabel:           "Agent",
		PermissionsLabel:     "Permissions",
		IsolationLabel:       "Candidate",
		AdmissionLabel:       "Admission",
		CompletionLabel:      "Completion",
		AttemptsLabel:        "Max attempts",
		PermissionsValue:     "workspace write",
		IsolationValue:       "isolated Git worktree",
		WorkspaceReady:       "workspace and Git identity checks passed",
		ProposalReady:        "isolated candidate workspace created",
		AgentStarted:         "%s started",
		AgentHeartbeat:       "%s working · %d changed files · waiting for next candidate",
		CandidateReceived:    "candidate #%d received · %d changed files",
		AdmissionRunning:     "candidate verification · admission: %s · recertification: %s",
		AdmissionPassed:      "candidate admitted · %s",
		AdmissionFailed:      "candidate not admitted · %s",
		FeedbackSent:         "verification feedback returned to %s; development continues",
		CheckpointSaved:      "trusted candidate preserved",
		FinalRunning:         "final independent recertification · %s",
		FinalPassed:          "final recertification passed · %s",
		FinalFailed:          "final recertification did not pass · %s",
		StateBound:           "final code state matches verified evidence",
		ResultTitle:          "StateSeal · Delivery result",
		ResultPassed:         "✓ Ready to deliver",
		ResultRejected:       "✗ Delivery requirements were not met",
		ResultAbstained:      "! Trusted verification could not complete",
		ResultStatus:         "! Delivery status: %s",
		ChangedFilesLabel:    "Changed files",
		AgentAttemptsLabel:   "Agent attempts",
		ChecksLabel:          "Checks",
		CoverageLabel:        "Coverage",
		DurationLabel:        "Duration",
		ReasonLabel:          "Reason",
		DeliveryBasis:        "Delivery evidence",
		BasisIsolated:        "candidate produced in an isolated workspace",
		BasisExternal:        "checks executed independently by StateSeal",
		BasisExactState:      "delivered code exactly matches verified state",
		BasisRecertified:     "final state recertified in a fresh evaluator",
		ResidualRiskLabel:    "Residual risks",
		ConfirmApply:         "\nAccept and apply this verified change? [y/N] ",
		Applied:              "✓ Verified change applied to branch %s.",
		NotApplied:           "Change is verified but not applied. Run `seal apply` when ready.",
		CoverageFull:         "intermediate verification + final recertification",
		CoverageTerminal:     "final recertification only (Agent hook not observed)",
		RiskConfiguredOnly:   "Only configured checks were evaluated",
		RiskHostUnattested:   "The execution host was not independently attested",
		RiskRemoteCI:         "Remote CI workflows were not executed locally",
		AutonomousNotice:     "Autonomous Agent permissions enabled; StateSeal verification remains external.",
		GoalRequired:         "provide a development goal, for example: seal run \"Add input validation while preserving compatibility\"",
		AgentTimedOut:        "Agent wall-time budget was exhausted; evaluating the latest candidate.",
		AgentExited:          "Agent exited with an error; the terminal candidate will still be evaluated: %v",
	},
	SimplifiedChinese: {
		FirstRunIntro:        "首次运行将自动配置：\n  • 检测项目验证命令\n  • 安装 %s 生命周期集成（合并现有配置）\n  • 授权运行由 StateSeal 生成并校验的项目 Hook\n  • 创建一个可审计的配置提交",
		FirstRunConfirm:      "继续？[Y/n] ",
		FirstRunCancelled:    "已取消首次配置",
		FirstRunDirty:        "首次配置需要干净的 Git 工作区；请先提交或暂存现有修改",
		FirstRunInteractive:  "首次运行需要确认 StateSeal 配置；请在交互终端运行，或添加 --yes",
		SetupComplete:        "✓ StateSeal 已配置 · Agent：%s · 验证：%s",
		SetupTitle:           "StateSeal 项目配置",
		PolicyCreated:        "已创建项目策略：%s",
		PolicyPreserved:      "已保留现有项目策略：%s",
		IntegrationReady:     "%s 生命周期集成已就绪：%s",
		IdentityReady:        "Git checkpoint 身份已就绪",
		BeforeFirstRun:       "首次运行前，请检查并提交 %s 和 %s。",
		StartTask:            "启动任务：seal run \"描述期望的开发结果\"",
		HookTrustConfirm:     "Codex 要求显式信任项目 Hook。是否授权 StateSeal 在受控运行中加载已校验的 Hook？[Y/n] ",
		HookTrustInteractive: "Codex 项目 Hook 信任需要确认；请在交互终端运行，或添加 --yes",
		ManagedTitle:         "StateSeal · 受控开发",
		GoalLabel:            "目标",
		PlanTitle:            "执行计划",
		AgentLabel:           "Agent",
		PermissionsLabel:     "权限",
		IsolationLabel:       "候选区",
		AdmissionLabel:       "Admission",
		CompletionLabel:      "Completion",
		AttemptsLabel:        "最大尝试",
		PermissionsValue:     "工作区写入",
		IsolationValue:       "隔离的 Git worktree",
		WorkspaceReady:       "工作区与 Git 身份检查通过",
		ProposalReady:        "已创建隔离候选区",
		AgentStarted:         "%s 已启动",
		AgentHeartbeat:       "%s 工作中 · 已变更 %d 个文件 · 等待下一个候选",
		CandidateReceived:    "收到候选 #%d · %d 个文件变更",
		AdmissionRunning:     "候选验证 · Admission：%s · 独立复验：%s",
		AdmissionPassed:      "候选已验证 · %s",
		AdmissionFailed:      "候选未通过 · %s",
		FeedbackSent:         "已将验证错误反馈给 %s，继续开发",
		CheckpointSaved:      "已保存可信候选",
		FinalRunning:         "结束前独立复验 · %s",
		FinalPassed:          "结束前复验通过 · %s",
		FinalFailed:          "结束前复验未通过 · %s",
		StateBound:           "最终代码状态与验证证据一致",
		ResultTitle:          "StateSeal · 验收结果",
		ResultPassed:         "✓ 验证通过，可以交付",
		ResultRejected:       "✗ 未达到交付标准",
		ResultAbstained:      "! 无法完成可信验证，需要处理前置条件",
		ResultStatus:         "! 验收状态：%s",
		ChangedFilesLabel:    "变更文件",
		AgentAttemptsLabel:   "Agent 尝试",
		ChecksLabel:          "验证检查",
		CoverageLabel:        "验证覆盖",
		DurationLabel:        "总耗时",
		ReasonLabel:          "原因",
		DeliveryBasis:        "交付依据",
		BasisIsolated:        "候选代码在隔离工作区生成",
		BasisExternal:        "检查由 StateSeal 独立执行",
		BasisExactState:      "交付代码与被验证状态完全一致",
		BasisRecertified:     "结束前已在全新 Evaluator 中复验",
		ResidualRiskLabel:    "剩余风险",
		ConfirmApply:         "\n是否接受并应用这份已验证的代码？[y/N] ",
		Applied:              "✓ 已将验证通过的代码应用到分支 %s。",
		NotApplied:           "代码已验证，尚未应用。确认后执行 `seal apply`。",
		CoverageFull:         "开发中验证 + 结束前复验",
		CoverageTerminal:     "结束前独立复验（Agent Hook 未观测）",
		RiskConfiguredOnly:   "仅评估了已配置的检查",
		RiskHostUnattested:   "执行主机未经独立证明",
		RiskRemoteCI:         "本地 Evaluator 未执行远程 CI 工作流",
		AutonomousNotice:     "已启用 Agent 免交互权限；StateSeal 验证边界保持独立。",
		GoalRequired:         "请指定开发目标，例如：seal run \"新增输入校验并保持兼容\"",
		AgentTimedOut:        "Agent 已用尽最大运行时间；将验证最新候选代码。",
		AgentExited:          "Agent 异常退出；仍将验证终止时的候选代码：%v",
	},
}

func init() {
	// English is the explicit fallback until a reviewed Traditional Chinese
	// catalog is available; never present Simplified Chinese as zh-Hant.
	catalogs[TraditionalChinese] = catalogs[English]
}
