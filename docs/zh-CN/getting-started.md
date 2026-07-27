[English](../getting-started.md) | 简体中文

# StateSeal 快速上手

正常使用只有三层一次性配置，之后每次只需描述开发目标：

```text
电脑级    安装 StateSeal 一次
Agent 级  每个 Agent 集成一次
项目级    首次确认验证合同一次
任务级    输入开发目标
```

## 1. 安装与确认版本

安装由 GitHub Actions 从确定 Git tag 构建的最新发行版。安装器会识别
macOS/Linux 与 amd64/arm64，下载匹配的归档、校验发布的 SHA-256，并安装到
`~/.local/bin`：

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal version
```

如需先检查脚本，把 `| sh` 替换为 `| less`。匿名评审或源码检出可以执行
`make install` 安装当前精确版本；源码安装要求 Git 与 Go 1.24 或更高版本。

## 2. 安装 Plugin 或集成 Agent

Codex 推荐使用 StateSeal Plugin。Plugin 已打包自动意图路由 Skill 与 MCP
注册；只需先安装 StateSeal Core，不需要单独安装 MCP：

```bash
codex plugin marketplace add /path/to/stateseal
codex plugin add stateseal@stateseal
```

`marketplace add` 只把当前源码目录登记为这台机器上的 Codex Plugin 目录。它不会
发布 Plugin、上传仓库、创建线上账号，也不会安装第二套 MCP server；正式
marketplace 发行可以隐藏这一步源码开发操作。

安装后新建 Codex 任务。普通代码需求会自动触发 StateSeal；`@stateseal` 和
`/seal status`、`/seal on`、`/seal off`、`/seal run`、`/seal exclude`
仅是可选控制面。

没有 Plugin 的宿主、CLI/headless 或修复场景使用一键安装命令：

```bash
seal install codex-desktop
# 或：seal integrate claude-code
# 或：seal integrate qoder
```

Codex Desktop 集成会在 `~/.codex/config.toml` 中注册本地 StateSeal MCP
服务，保留已有模型、项目和 MCP 配置，同时创建一次安全备份。安装过程会执行
MCP 初始化和工具握手；不需要输入 `/hooks`，也不需要单独信任命令 Hook。
可以随时检查或撤销：

```bash
seal integrate status
seal integrate doctor codex-desktop
seal integrate uninstall codex-desktop
```

重启 Codex Desktop 后集成生效。Codex Desktop 受控交付已通过确定性 MCP
端到端测试，但在固定 Desktop 版本完成实机 conformance 前仍标记为
experimental。Claude Code 与 Qoder 当前完成的是生命周期基础；其 Desktop
MCP 闭环仍在兼容性矩阵中单独跟踪。`seal run` 是已经完成实机验证的通用入口。

## 3. 通用 CLI 路径

Plugin 不可用、CLI/headless、CI 或需要显式边界时，可以直接指定仓库：

```bash
seal run --repo /path/to/your-project \
  "增加输入校验，保持兼容，并补充完整测试"
```

也可以在仓库根目录省略 `--repo`。如果当前目录不是 Git 仓库，StateSeal 会发现
其中的 Git 子仓库；只有一个时可以自动选择，多个时必须明确选择：

```bash
seal workspace list /path/to/workspace
seal run --repo /path/to/workspace/service-a "增加输入校验并补充测试"
```

项目首次使用时，StateSeal 会先展示最低交付合同：

```text
StateSeal · 首次项目配置

  项目: service
  Agent: Codex
  Admission: go test ./...
  Completion: go test ./...; go vet ./...
  受保护配置: seal.yaml, .codex/hooks.json

继续？[Y/n]
```

只有当这些命令适合作为项目最低交付门槛时才确认。需要私有测试、远程 CI
或业务验收命令时，应先检查并调整 `seal.yaml`。

自动发现的检查会标记为 `L1 / auto-discovered`，只代表最低工程门槛，不代表
完整需求。项目可以将人工确认的检查标记为 `project-policy`，补充 L2 业务验收，
或接入 L3 远程 CI / 受保护 Runner。

## 4. 等待验收结果

StateSeal 自动创建隔离候选区、启动 Agent、独立复验候选、保存可信
checkpoint，并在结束前使用全新 Evaluator 复验。验证通过后，用户只需决定
是否应用代码：

```text
StateSeal · 受控交付

[00:00] ✓ 已加载项目验证策略
[00:01] ✓ 已创建隔离候选区
[00:15] ● Agent 正在分析项目 · 尚未产生代码变更
[00:45] ● Agent 正在实现 · 已变更 2 个文件
[01:14] ◆ 收到候选 #1 · 2 个文件变更
[01:18] ✓ go test ./... · 通过 · 2.1s
[01:19] ✓ go vet ./... · 通过 · 0.6s
[01:23] ✓ 最终代码状态与验证证据一致

StateSeal · 验收结果
✓ 验证通过，可以交付

  变更文件：2
  文件：ulid.go, ulid_test.go
  Receipt：rcpt_…
  代码状态：230fe20ef22f

交付依据
  ✓ 候选代码在隔离工作区生成
  ✓ 检查由 StateSeal 独立执行
  ✓ 交付代码与被验证状态完全一致
  ✓ 结束前已在全新 Evaluator 中复验

验证覆盖
  L0 · 状态、策略、新鲜度、Checkpoint 与 Receipt 完整性
  L1 · auto-discovered · 2 项检查
  交付影响：评估 2 个候选，拒绝 0 个，验证 2 个 Checkpoint

是否接受并应用这份已验证的代码？[y/N]
```

`ADMITTED` 只表示确切代码状态通过了 `seal.yaml` 中配置的检查，不代表测试
覆盖完整、主机可信或远程 CI 已经通过；Verifier 层级、来源、未覆盖项、候选
拒绝和恢复情况都会继续显示在 Receipt 中。

## Codex Desktop 使用路径

安装 Plugin 并新建任务后，在 Codex Desktop 中选择本地环境，打开 Git 项目或包含
多个 Git 子仓库的 Workspace，然后正常输入开发目标。仅在 Plugin 不可用或修复旧
安装时才需要 `seal install codex-desktop`：

```text
增加输入校验，保持兼容，并补充完整测试
```

StateSeal MCP 会先只读识别项目。只读解释、搜索和规划不进入受控交付。项目
首次出现代码变更任务时，原对话展示 Workspace、Repository、Worker Agent、
隔离执行方式，以及自动发现的 Admission、Completion、受保护
路径和剩余风险；随后 StateSeal 直接发起原生确认，用户确认一次
`enable_project`。宿主的重要工具授权就是这一次确认，不再嵌套第二次
elicitation。StateSeal 将合同和 repo-local Hook 提交为受保护项目配置，后续普通
开发目标不再重复确认，除非策略发生变化。

编码与自测由受控 Codex 子进程在隔离候选区完成；用户源目录在此期间不变。
验证通过后，原对话展示阶段时间线、变更文件、检查与耗时、receipt、确切代码
状态、覆盖范围和剩余风险；StateSeal 随后发起第二个原生确认。只有与当前会话
完全匹配、非空且已验证的 receipt
才能应用到 feature 分支；拒绝则保持用户分支不变。普通用户无需输入任何
`seal desktop` 内部命令。

如果打开的是非 Git 父目录，StateSeal 会发现其中的 Git 子仓库。只有一个时可
直接选择；存在多个时必须展示列表并由用户明确选择，绝不默认取第一个。CLI
可使用 `seal workspace list` 查看，并通过 `seal run --repo <path> "…"` 选择。

StateSeal 自身的确认、MCP、Store 或隔离 Worker 故障不会伪装成验证失败，也
不会终止普通开发。Agent 会回到原生工作流，并将结果醒目标记为
`UNVERIFIED`，且不生成 StateSeal Receipt。健康状态下 enforce 策略的真实
验证拒绝仍然有效。最终 Apply 永不自动放行；确认不可用或被拒绝时保留
Checkpoint，用户源分支保持不变。
