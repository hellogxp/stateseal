# StateSeal 快速上手

正常使用只有三层一次性配置，之后每次只需描述开发目标：

```text
电脑级    安装 StateSeal 一次
Agent 级  每个 Agent 集成一次
项目级    首次确认验证合同一次
任务级    输入开发目标
```

## 1. 安装与确认版本

安装发行版后确认二进制身份：

```bash
seal version
```

## 2. 集成 Agent

根据本机使用的 Agent 执行一次：

```bash
seal integrate codex-desktop
# 或：seal integrate claude-code
# 或：seal integrate qoder
```

StateSeal 会合并用户级生命周期 Hook，保留已有配置，同时创建一次安全备份。
Codex Desktop 使用 `SessionStart`、`UserPromptSubmit`、`PreToolUse`、
`PostToolUse` 和 `Stop` 来绑定会话、阻止受控任务绕过隔离候选区，并把验收
结果返回原对话。可以随时检查或撤销：

```bash
seal integrate status
seal integrate doctor codex-desktop
seal integrate uninstall codex-desktop
```

Codex 对非托管命令 Hook 要求原生信任检查。新建一个 Codex 会话，执行
`/hooks`，检查 StateSeal 的精确定义并信任一次。StateSeal 在普通 Desktop
使用中不会绕过这一步。

Codex Desktop 受控交付已通过确定性端到端测试，但在固定 Desktop 版本完成
实机 conformance 前仍标记为 experimental。Claude Code 与 Qoder 当前完成的
是用户级生命周期基础，不代表完整 Desktop 闭环。`seal run` 仍是已经完成
实机验证的通用入口。

## 3. 在项目中启动任务

进入 Git 项目根目录：

```bash
cd /path/to/your-project
seal run "增加输入校验，保持兼容，并补充完整测试"
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

## 4. 等待验收结果

StateSeal 自动创建隔离候选区、启动 Agent、独立复验候选、保存可信
checkpoint，并在结束前使用全新 Evaluator 复验。验证通过后，用户只需决定
是否应用代码：

```text
StateSeal · 验收结果
✓ 验证通过，可以交付

交付依据
  ✓ 候选代码在隔离工作区生成
  ✓ 检查由 StateSeal 独立执行
  ✓ 交付代码与被验证状态完全一致
  ✓ 结束前已在全新 Evaluator 中复验

是否接受并应用这份已验证的代码？[y/N]
```

`ADMITTED` 只表示确切代码状态通过了 `seal.yaml` 中配置的检查，不代表测试
覆盖完整、主机可信或远程 CI 已经通过；这些风险会继续显示在 receipt 中。

## Codex Desktop 使用路径

执行一次 `seal integrate codex-desktop` 后，在 Codex Desktop 中打开 Git 项目，
每个受控任务只需输入：

```text
StateSeal: 增加输入校验，保持兼容，并补充完整测试
```

StateSeal 会在仓库外绑定 Codex `session_id`、项目、目标和任务 ID。项目首次
使用时，原对话先展示自动识别的 Admission 与 Completion 命令，请用户确认
一次。确认后，Desktop 父会话只负责调用精确的 StateSeal 委托命令；直接编辑
工具会被拒绝。真正编码在隔离候选区内由受控 Codex 子进程完成。

验证通过后，原 Desktop 对话展示 receipt、检查结果和剩余风险，再询问是否
应用。输入明确的“确认”或“yes”才会把确切 checkpoint 应用到 feature 分支；
输入“拒绝”或“no”则保持用户分支不变。内部 `seal desktop` 命令主要供 adapter
和异常恢复使用，普通用户无需手动输入。
