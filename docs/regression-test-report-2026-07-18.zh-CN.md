# StateSeal 产品级回归测试报告

> 测试日期：2026-07-18  
> 测试版本：`abcd9d4`（当前工作树仅包含测试前已经存在的文档修改）  
> 平台：macOS / Apple Silicon / Go 1.26.3  
> 范围：CLI、受控 Agent Loop、准入状态机、checkpoint、receipt、恢复、适配器和多技术栈

## 1. 测试结论

StateSeal 的核心机制已经成立：Agent 可以在隔离候选区开发，失败候选会被拒绝，正确候选会被独立验证并固化为 checkpoint；即使 Agent 后续把代码改坏，StateSeal 仍能恢复并重新认证最后一个正确状态。

| 维度 | 结果 | 结论 |
| --- | ---: | --- |
| Go 单元与集成测试 | 44 个测试函数；普通模式全部通过 | 通过 |
| Race 检查 | 首轮出现 1 次时间断言抖动；随后完整套件 3 轮通过，专项 20 轮通过 | 有测试稳定性风险 |
| SealBench 故障注入 | 24/24 | 通过 |
| Go / Python / Node 兼容矩阵 | 3/3 | 通过 |
| Agent 失败后修复 | 第 1 个候选被拒绝，第 2 个候选通过 | 通过 |
| Agent 终态回归恢复 | 正确 checkpoint 被恢复并重新认证 | 通过 |
| 主流 Agent 生命周期协议 | 6 个适配器通过确定性矩阵 | 通过 |
| Codex CLI 真实开发 | 隔离开发、验证、receipt 和 apply 跑通 | 通过 |
| apply 后状态 | 错误显示 `STALE` | 失败，发布阻断 |
| 重复初始化错误提示 | 退出码正确，但没有错误文字 | 失败，体验阻断 |

发布判断：**核心机制可进入 design partner 试用；修复 apply 状态错误和 CLI 静默错误前，不建议作为面向普通用户的公开 Alpha 发布。**

## 2. 产品是什么

StateSeal 不是新的 Agent，也不是新的测试框架。它是 coding agent 与用户分支之间的本地准入层。

```text
用户定义目标和验收规则
          │
          ▼
StateSeal 创建隔离候选区（Git worktree）
          │
          ▼
Agent 分析 → 编码 → 自测 → 修复
          │  尝试交付候选
          ▼
StateSeal 独立验证精确代码状态
     ┌────┴────┐
     │         │
   失败       通过
     │         │
退回证据   checkpoint + receipt
     │         │
Agent 修复     ▼
     └───→ 用户审阅并执行 apply
```

Agent 负责产生代码；StateSeal 负责回答：**这份精确代码是否满足已经约定的交付条件，是否可以进入用户分支。**

## 3. 普通用户如何使用

### 3.1 进入目标项目

所有命令都在准备开发的 Git 项目目录中执行：

```bash
cd /path/to/your-project
```

### 3.2 定义本次开发目标

```bash
seal init --goal "Repeated callbacks must create exactly one charge."
```

作用：生成 `seal.yaml`，记录目标、受保护路径、准入检查和完成检查。

用户需要检查两件事：

1. `task.goal` 是否准确表达需求；
2. verifier 命令是否真正覆盖验收条件。

### 3.3 安装所用 Agent 的生命周期适配器

以 Codex 为例：

```bash
seal adapter codex install
seal doctor
git add seal.yaml .codex/hooks.json
git commit -m "Configure StateSeal admission policy"
```

作用：让 StateSeal 在 Agent 自测完成和准备退出时自动观察候选边界。策略必须先提交，防止 Agent 在开发过程中修改验收规则。

当前支持：

```text
Codex · Claude Code · Gemini CLI · Cursor
GitHub Copilot CLI · OpenCode
```

没有原生适配器的终端 Agent 仍可通过 `seal run -- <agent>` 使用，但只能获得 terminal-only 覆盖。

### 3.4 启动受控开发

```bash
seal run -- codex exec \
  "Implement the requirement. Preserve compatibility and run all tests."
```

此时：

- 用户分支保持不变；
- Codex 在 StateSeal 管理的独立 worktree 中开发；
- Codex 可以自行多轮分析、编码和自测；
- 每次候选交付都由外部 broker 决定是否准入。

第一次候选失败时，典型输出为：

```text
StateSeal — REJECTED
Reason:  admission check failed; last verified checkpoint was preserved
Rule:    VR001 — a configured verifier failed
Mode:    enforce (BLOCKED)
```

修复后通过时，典型输出为：

```text
StateSeal — ADMITTED
Receipt: rcpt_...
Tree:    ...
Mode:    enforce (ALLOWED)
```

### 3.5 查看交付证据

```bash
seal status
seal diff
seal timeline
```

| 命令 | 用户获得的信息 |
| --- | --- |
| `seal status` | 当前是否准入、证据是否新鲜、checkpoint 和 receipt |
| `seal diff` | 已验证 checkpoint 相对可信基线的真实改动 |
| `seal timeline` | 候选提交、拒绝、验证、恢复和准入全过程 |

### 3.6 应用已验证代码

```bash
seal apply --branch fix/payment-idempotency
```

只有仍然新鲜的 `ADMITTED` checkpoint 才能应用。应用前，Agent 的修改不会进入用户分支。

## 4. 实际回归结果

### 4.1 基础质量

执行：

```bash
go vet ./...
go test ./... -count=1
go test -race ./... -count=3
```

结果：静态检查通过；普通测试全部通过；完整 race 套件连续 3 轮通过。

首轮并行执行完整 race 测试时，`TestSuiteWallBudgetCapsChecks` 出现一次失败。失败原因是 100ms 总预算在 race 开销下被第一个命令耗尽，测试仍假设一定产生两条 evidence。随后进行交叉复测：

```text
普通专项测试：50/50 通过
Race 专项测试：20/20 通过
完整 Race 套件：3/3 通过
```

判断：当前更接近测试时间断言抖动，而不是预算控制失效；仍应修改测试，使其不依赖第一个进程必须在 100ms 内完成。

### 4.2 SealBench 故障注入

执行：

```bash
./sealbench/run.sh ./bin/seal
```

结果：`SealBench passed 24/24 deterministic failure-injection cases.`

| Case | 场景 | 结果 |
| --- | --- | --- |
| SB001 | 旧 receipt 跨仓库重放 | 阻断 |
| SB002 | 验证通过后继续修改 | 标记过期 |
| SB003 | 可信分支或代码树变化 | 阻断 |
| SB004 | verifier 命令或 cwd 不同 | 证据身份不同 |
| SB005 | 测试套件通过后变化 | 旧证据失效 |
| SB006 | 策略通过后变化 | 旧准入失效 |
| SB007 | receipt 被编辑 | 完整性校验失败 |
| SB008 | 正确后回归 | 恢复正确 checkpoint |
| SB009 | 失败候选覆盖 checkpoint | 阻断 |
| SB010 | 候选预算耗尽 | 恢复已有 checkpoint |
| SB011 | verifier 超时 | 拒绝 |
| SB012 | 完成复验失败 | 不准入 |
| SB013 | 修改受保护路径或逃逸 symlink | 拒绝 |
| SB014 | 非法提交请求 | 记录 abstain，保留 checkpoint |
| SB015 | 无中间边界 | 明确报告 terminal-only |
| SB016 | 恢复时存在未跟踪残留 | 精确恢复 checkpoint |
| SB017 | verifier cwd 是文件 | 稳定退出码 126 |
| SB018 | Agent 环境中存在 secret | verifier 不继承 secret |
| SB019 | verifier 启动后台进程 | 进程组被清理 |
| SB020 | Agent 超过总时间预算 | 终止进程并恢复 checkpoint |
| SB021 | Codex 生命周期边界 | 中间 checkpoint 可恢复 |
| SB022 | shadow / warn 模式 | 正确记录 disposition |
| SB023 | 六种 Agent 生命周期映射 | 通过 |
| SB024 | Agent 构建缓存 | 可写且不污染候选身份 |

### 4.3 多技术栈兼容

执行：

```bash
./sealbench/stacks.sh ./bin/seal
```

结果：

```text
PASS STACK-GO      managed go test
PASS STACK-PYTHON  managed pytest policy
PASS STACK-NODE    managed npm test with ignored dependencies
Stack matrix passed 3/3 ecosystems.
```

这证明 StateSeal 核心不依赖 Go 项目；它管理的是代码状态、验证命令和准入证据。

### 4.4 Agent 第一次失败、第二次修复

测试 Agent 在同一次受控运行中执行：

```text
候选 1：把 app.txt 写成 bad
候选 2：收到拒绝后修复为 good
```

实际结果：

```text
AGENT_ATTEMPT=1
StateSeal — REJECTED
Rule: VR001
FIRST_SUBMIT_EXIT=1

AGENT_ATTEMPT=2
StateSeal — ADMITTED
```

最终状态：

```text
Status:     ADMITTED
Freshness:  CURRENT
Coverage:   intermediate + terminal
Checkpoint: cp_3d255bf05c8e71988c77ae6c
```

在执行 `seal apply` 之前，用户目录中的 `app.txt` 仍然是 `original`；应用后才变成 `good`。这验证了隔离候选区的实际价值。

### 4.5 正确 checkpoint 后发生终态回归

测试步骤：

```text
Agent 生成正确实现
  → seal submit 保存 checkpoint
  → Agent 又把代码改坏
  → Agent 准备结束
```

实际结果：

```text
CANDIDATE_REJECTED
REGRESSION_DETECTED        terminal_candidate_regressed
CHECKPOINT_SELECTED        terminal_candidate_regressed
CHECKPOINT_RESTORED
COMPLETION_RECERTIFIED
COMPLETION_ADMITTED        CP001
```

最终 receipt：

```text
Receipt:          rcpt_8ca3758ecfa57a031e165d0e
Recovered:        yes
Selection reason: terminal_candidate_regressed
```

这验证了 StateSeal 的关键差异：它不相信 Agent 最后的文字声明，而是重新认证最后一个正确代码状态。

### 4.6 真实 Codex CLI 开发

真实 Codex CLI 已完成一次 UUID 功能开发：

```text
StateSeal 创建 proposal worktree
Codex 阅读需求、编码并运行 Go 测试
StateSeal 独立验收
生成 checkpoint 与 receipt
用户查看 diff 并 apply
```

结果：

```text
Status:     ADMITTED
Checkpoint: cp_2805aea19e4b13457995d655
Tree:       77b964e90544
Receipt:    rcpt_36ce48ff61b5a68b33b959fd
Applied:    cac4609 Checkpoint candidate state
```

该场景使用独立运行环境，未调用内部 KBase。

## 5. 发现的问题

### P0：apply 成功后错误显示 STALE

复现率：2/2。

```text
Verified checkpoint applied.
Status:     ADMITTED
Freshness:  STALE
Stale:      trusted base changed after admission
```

代码已经正确应用，但状态机仍拿当前 `HEAD` 与 apply 前的 `BaseCommit` 比较，因此把 StateSeal 自己执行的 fast-forward 当成外部篡改。

影响：用户刚完成可信交付就看到“证据过期”，直接破坏产品的信任表达。

发布要求：apply 后应进入明确的 `APPLIED/CURRENT` 状态，或记录应用事件并让 freshness 认可精确 checkpoint commit。

### P1：CLI 错误静默

重复执行 `seal init`：

```text
Exit code: 10
Output:     <empty>
```

退出码正确，但普通用户无法知道失败原因和下一步操作。

发布要求：所有稳定非零退出码都必须向 stderr 输出原因、rule ID 和建议动作。

### P1：同一代码树重复生成认证结果

真实 Codex 场景中，同一 tree：

```text
77b964e9054470f1...
```

连续生成了 3 个 checkpoint/receipt。两次来自生命周期中间提交，一次来自 terminal completion。终态重新认证是必要的，但相同 tree 的中间 admission 可以去重或合并，避免重复执行昂贵 verifier 并制造审计噪声。

发布要求：以 `tree + policy + suite + environment` 为幂等键；保留终态 fresh recertification，但复用或折叠相同中间候选。

### P1：Race 测试存在时间断言抖动

产品预算控制场景通过，测试曾出现一次偶发失败。应把“总预算被执行”与“必然产生两条 evidence”拆成独立断言，避免慢机器或 race instrumentation 造成 CI 假失败。

### P2：Agent 原生日志较嘈杂

真实 Codex 输出混有模型缓存告警和大量 hook 日志。它不影响准入正确性，但会遮挡 StateSeal 的关键状态。

建议默认提供简洁事件视图，并允许 `--verbose` 展示完整 Agent 原始输出。

## 6. 产品价值判断

本轮测试证明了五项真实价值：

1. **不直接污染用户分支**：Agent 失败时，用户工作目录保持不变。
2. **验证绑定精确状态**：证据绑定 tree、policy、suite、cwd 和环境身份，不接受过期结果。
3. **Agent 无权宣布成功**：broker 根据外部 verifier 决定 `ADMITTED` 或 `REJECTED`。
4. **能够恢复正确中间状态**：Agent 后续回归时，最后一个正确 checkpoint 不会丢失。
5. **形成可审计交付证据**：receipt 和 timeline 解释代码为何可以交付，以及仍有哪些残余风险。

StateSeal 最适合的当前定位是：

> 面向长时间运行和无人值守 coding agent 的本地可信交付层。

它解决的不是“Agent 会不会写代码”，而是：

> Agent 说完成以后，我们是否能证明某一份精确代码确实满足交付规则，并且安全地把它带回用户分支。

## 7. 下一步发布门槛

按优先级建议：

1. 修复 apply 后 `STALE`，增加 apply → status 回归测试；
2. 让所有 CLI 错误输出可操作的文字说明；
3. 对相同 tree 的中间候选做幂等去重；
4. 修复 race 时间测试的稳定性；
5. 增加一条真实 Codex“首次失败 → 自动收到证据 → 修复 → 通过”的固定发布演示；
6. 再执行 24-case SealBench、3-stack matrix、race suite 和真实 Agent smoke test，作为 Alpha 发布门禁。

