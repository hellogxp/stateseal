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

StateSeal 会合并用户级 `PostToolUse` 和 `Stop` Hook，保留已有配置，
同时创建一次安全备份。可以随时检查或撤销：

```bash
seal integrate status
seal integrate doctor codex-desktop
seal integrate uninstall codex-desktop
```

当前 Desktop/IDE 集成属于 experimental：安装成功仅证明生命周期配置结构
正确，不代表该 Agent 版本已经通过完整实机验证。现阶段权威交付路径仍是
`seal run`。

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
