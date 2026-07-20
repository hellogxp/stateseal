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

执行一次 `seal integrate codex-desktop` 并重启应用后，在 Codex Desktop 中选择
本地环境、打开 Git 项目，然后正常输入开发目标：

```text
增加输入校验，保持兼容，并补充完整测试
```

StateSeal MCP 会先只读识别项目。项目首次使用时，原对话展示自动发现的
Admission、Completion、受保护路径和剩余风险；随后 Codex 弹出原生工具授权，
用户确认一次 `enable_project`。StateSeal 将合同提交为 `seal.yaml`，后续普通
开发目标不再重复确认，除非策略发生变化。

编码与自测由受控 Codex 子进程在隔离候选区完成；用户源目录在此期间不变。
验证通过后，原对话展示 receipt、检查结果、覆盖范围和剩余风险，并通过第二个
原生授权询问是否执行 `apply_verified`。只有与当前会话完全匹配的已验证 receipt
才能应用到 feature 分支；拒绝则保持用户分支不变。普通用户无需输入任何
`seal desktop` 内部命令。
