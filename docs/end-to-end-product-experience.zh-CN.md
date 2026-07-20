# StateSeal 端到端产品体验报告

本文记录一次真实、完整的 StateSeal 产品使用过程：用户提出开发目标，Codex 在隔离环境中实现需求，StateSeal 对精确代码快照进行独立验证，用户检查证据后应用 checkpoint，最终完成测试与远程推送。

这不是模拟输出。文中的 commit、checkpoint、tree 和 receipt 均来自实际执行。内部仓库地址和个人身份已脱敏。

## 1. 执行结论

核心交付链路执行成功：

```text
需求尚未实现
  → 用户定义目标与验收规则
  → Codex 在隔离 Worktree 中开发
  → StateSeal 固定候选代码快照
  → 干净环境独立验证
  → 生成 checkpoint 和 receipt
  → 用户检查 diff 与证据
  → 应用精确 checkpoint
  → test / race / vet 通过
  → 远程分支推送成功
```

本次同时发现两个产品体验问题：

1. 同一候选状态被终态认证三次，产生三个等价 receipt；
2. 精确 checkpoint 应用成功后，`seal status` 错误显示 `STALE`。

因此，本次测试的判断是：**核心可信交付能力通过，普通用户 Alpha 体验尚需修复状态与生命周期问题。**

## 2. 测试场景

目标项目为真实开源代码库 `github.com/google/uuid`，上游基线提交为：

```text
2d3c2a9 feat: Generate V6 from custom time (#172)
```

开发需求：

> 新增 `ParseCanonical(string) (UUID, error)`。只接受小写、带连字符的 RFC 9562 标准 UUID；拒绝大写、无连字符、URN、花括号和首尾空格格式；保持现有 `Parse` 的宽松兼容行为不变。

验收标准：

| 验收项 | 预期结果 |
| --- | --- |
| 小写标准 UUID | 接受 |
| 大写 UUID | 拒绝 |
| 无连字符 UUID | 拒绝 |
| `urn:uuid:` 格式 | 拒绝 |
| 花括号格式 | 拒绝 |
| 首尾空格 | 拒绝 |
| 原有 `Parse` 行为 | 保持兼容 |
| 原有测试和新增验收测试 | 全部通过 |
| 需求、测试、策略和 Hook | Agent 不得修改 |

## 3. 执行环境

| 项目 | 实际值 |
| --- | --- |
| StateSeal 源码 | `db72814ae1f092966a4fa50fec95517c5a5aa5fa` |
| StateSeal 版本 | `v0.0.0-20260717090553-db72814ae1f0` |
| StateSeal 平台 | `darwin/arm64` |
| Go | `go1.26.3` |
| Agent | Codex CLI `0.144.5` |
| 模型 | `gpt-5.5`，medium reasoning |
| 策略提交 | `d061a9e` |
| Verified commit | `b0ed47114fcaac416bd7b3d1fb2b950284c58066` |
| StateSeal checkpoint | `cp_d4bb816bc74b5b779a9b05c1` |
| StateSeal tree | `b90c221f8c2a` |
| 最终 receipt | `rcpt_46485983ae98cbaa9f630ba2` |
| 交付分支 | `feature/parse-canonical-product-test` |

## 4. 完整执行记录

### 4.1 确认需求尚未实现

输入：

```bash
git clone --branch experience/uuid-canonical-baseline \
  --single-branch <repository-url> uuid
cd uuid
go test ./...
```

输出：

```text
# github.com/google/uuid [github.com/google/uuid.test]
./canonical_acceptance_test.go:12:14: undefined: ParseCanonical
./canonical_acceptance_test.go:36:17: undefined: ParseCanonical
FAIL    github.com/google/uuid [build failed]
FAIL
```

结论：开发目标在 Agent 执行前确实未完成。

### 4.2 确认 StateSeal 版本

输入：

```bash
seal version
```

输出：

```text
StateSeal v0.0.0-20260717090553-db72814ae1f0
Commit:    db72814ae1f092966a4fa50fec95517c5a5aa5fa
Built:     2026-07-17T09:05:53Z
Go:        go1.26.3
Platform:  darwin/arm64
```

### 4.3 定义目标并接入 Codex

输入：

```bash
seal init --goal \
  "Add ParseCanonical to accept only lowercase hyphenated RFC 9562 UUIDs while preserving Parse compatibility."

seal adapter codex install
```

输出：

```text
StateSeal initialized

Policy:    <repository>/seal.yaml
Task:      uuid
Verifier:  go test
Mode:      enforce (default)

Codex adapter installed at <repository>/.codex/hooks.json
Commit the file as protected infrastructure before the first managed run.
```

用户检查生成的 `seal.yaml`，将需求和验收测试加入保护范围：

```yaml
task:
  id: uuid
  goal: Add ParseCanonical to accept only lowercase hyphenated RFC 9562 UUIDs while preserving Parse compatibility.

state:
  include:
    - "**"
  protected:
    - seal.yaml
    - REQUIREMENT.md
    - canonical_acceptance_test.go
    - .git/**
    - .codex/**

admission:
  checks:
    - id: tests
      command: [go, test, ./...]

completion:
  checks:
    - id: tests
      command: [go, test, ./...]
  recertify_latest_checkpoint: true
  on_missing_evidence: abstain
  on_stale_evidence: reject
```

输入：

```bash
seal doctor
git add seal.yaml .codex/hooks.json
git commit -m "Configure StateSeal admission policy"
```

输出：

```text
✓ Git repository
✓ seal.yaml
✓ git executable
Platform: darwin/arm64

[experience/uuid-canonical-baseline d061a9e] Configure StateSeal admission policy
 2 files changed, 79 insertions(+)
 create mode 100644 .codex/hooks.json
 create mode 100644 seal.yaml
```

### 4.4 信任 Codex Hook

普通用户首次使用时，应启动 Codex，执行 `/hooks`，检查并信任项目 Hook。

本次为无人值守测试，使用 Codex 的 `--dangerously-bypass-hook-trust` 临时代替人工点击。该参数只跳过 Hook 信任交互，没有绕过 StateSeal 的策略、保护路径、验证、终态重认证或用户确认应用。

### 4.5 启动开发 Loop

实际输入：

```bash
seal run -- codex exec \
  --ephemeral \
  --dangerously-bypass-hook-trust \
  -s workspace-write \
  "Implement the requirement in REQUIREMENT.md. Add exported ParseCanonical(string) (UUID, error). It must accept only lowercase hyphenated canonical RFC 9562 UUID text, reject all non-canonical representations covered by canonical_acceptance_test.go, preserve the existing permissive Parse behavior, and run go test ./.... Do not modify REQUIREMENT.md, any test file, seal.yaml, or agent hook configuration."
```

StateSeal 创建隔离工作区：

```text
Proposal worktree: <user-cache>/stateseal/f4b9e984eefaf906/uuid/proposal-d061a9e489f9
```

Codex 的有效行为：

```text
读取 REQUIREMENT.md 和验收测试
→ 阅读现有 Parse 实现
→ 修改 uuid.go 和 util.go
→ 运行 gofmt
→ 执行 go test ./...
→ 检查最终修改范围
```

关键测试输出：

```text
ok      github.com/google/uuid  0.997s
```

### 4.6 StateSeal 给出交付判断

终端输出：

```text
StateSeal — ADMITTED
Receipt: rcpt_46485983ae98cbaa9f630ba2
Tree:    b90c221f8c2a
Mode:    enforce (ALLOWED)
Residual risk:
  - Only configured checks were evaluated.
  - The execution host was not independently attested.
```

输入：

```bash
seal status
seal explain
seal diff
```

应用前状态：

```text
Task:       uuid
Status:     ADMITTED
Freshness:  CURRENT
Mode:       enforce
Coverage:   intermediate + terminal
Checkpoint: cp_d4bb816bc74b5b779a9b05c1
Tree:       b90c221f8c2a
Receipt:    rcpt_46485983ae98cbaa9f630ba2

Verdict: ADMITTED
Next: inspect with `seal diff`, then use `seal apply`
```

`seal diff` 显示只有两个实现文件发生变化：

```text
M util.go
M uuid.go
```

需求、验收测试、`seal.yaml` 和 Codex Hook 均未被修改。

### 4.7 检查 checkpoint 与 receipt

Checkpoint commit：

```text
commit=b0ed47114fcaac416bd7b3d1fb2b950284c58066
tree=373059cdb18e53ef45e68c9845936c4b8e708978
author=<repository-configured identity>
committer=<repository-configured identity>
subject=Checkpoint candidate state
```

StateSeal 创建 checkpoint 前后，目标仓库的 `user.name` 和 `user.email` 保持不变。

本次产生三个 receipt：

```text
rcpt_277766154836e91df3817c54
rcpt_f6ef1b319b21a399cc4a364a
rcpt_46485983ae98cbaa9f630ba2
```

三者均通过结构完整性检查，并绑定同一个 StateSeal tree：

```text
Receipt <id> is structurally intact.
Verdict: ADMITTED
Tree:    b90c221f8c2a
Note: local receipt integrity does not establish CI authority.
```

这说明重复 receipt 指向同一候选状态，不是三个不同的实现。

### 4.8 用户确认并应用

输入：

```bash
seal apply --branch feature/parse-canonical-product-test
```

输出：

```text
Verified checkpoint applied.
```

应用后的 HEAD 与 verified checkpoint 完全一致：

```text
b0ed471 Checkpoint candidate state
d061a9e Configure StateSeal admission policy
1653da9 Add canonical UUID acceptance specification
2d3c2a9 feat: Generate V6 from custom time (#172)
```

中间没有 amend、squash 或身份重写。

### 4.9 最终复验

输入：

```bash
go test ./...
go test -race ./...
go vet ./...
git diff --check d061a9e..HEAD
```

输出：

```text
ok      github.com/google/uuid  1.478s
ok      github.com/google/uuid  6.274s
```

`go vet` 和 `git diff --check` 无错误输出。

需要准确区分：receipt 绑定的是策略中配置的 `go test ./...`；race 和 vet 是 apply 后的额外复验，不属于 receipt 声明的证据范围。

### 4.10 推送交付分支

输入：

```bash
git push -u origin feature/parse-canonical-product-test
```

输出：

```text
To <repository-url>
 * [new branch] feature/parse-canonical-product-test
     -> feature/parse-canonical-product-test
branch 'feature/parse-canonical-product-test' set up to track
  'origin/feature/parse-canonical-product-test'.
```

远程分支最终指向同一个 verified commit：

```text
b0ed47114fcaac416bd7b3d1fb2b950284c58066
refs/heads/feature/parse-canonical-product-test
```

至此，功能交付链路完成。

## 5. 验收结果

| 检查项 | 结果 | 证据 |
| --- | --- | --- |
| 初始需求未实现 | 通过 | 基线编译失败 |
| Agent 未直接修改用户分支 | 通过 | 隔离 Proposal Worktree |
| 需求和验收测试受到保护 | 通过 | 最终文件变更检查 |
| 原有 `Parse` 行为保持兼容 | 通过 | 验收测试与上游测试 |
| 配置的 verifier 通过 | 通过 | `go test ./...` |
| 完成阶段重新认证 | 通过 | `COMPLETION_RECERTIFIED` 事件 |
| 代码与 checkpoint 精确绑定 | 通过 | commit `b0ed471`、tree `b90c221f8c2a` |
| Receipt 结构完整 | 通过 | 三次 `seal inspect` |
| Git 身份保持不变 | 通过 | checkpoint 前后配置一致 |
| 用户显式确认应用 | 通过 | `seal apply` |
| Apply 后 test / race / vet | 通过 | 三项命令成功 |
| 远程推送 | 通过 | 远程分支指向 `b0ed471` |
| 单一终态判断 | 未通过 | 同一状态认证三次 |
| Apply 后状态正确 | 未通过 | 错误显示 `STALE` |

## 6. 发现的问题

### P0：Apply 后状态错误

精确 checkpoint 应用成功后，`seal status` 输出：

```text
Status:     ADMITTED
Freshness:  STALE
Stale:      trusted base changed after admission
Checkpoint: cp_d4bb816bc74b5b779a9b05c1
```

此时 HEAD 就是 verified checkpoint，不应该被判断为失效。合理状态应类似：

```text
Status:     APPLIED
Freshness:  CURRENT
```

### P1：重复终态认证

同一个 StateSeal tree 产生三轮相同的终态事件和三个 receipt。当前 `PostToolUse`、`Stop` 与外层 `seal run` 都可能完成终态认证。

目标行为应是：中间边界只产生 verified checkpoint；终态只执行一次 fresh completion；相同 tree 的重复提交应被去重。

### P2：终端信息噪声较多

Codex 环境警告、Hook 消息和 StateSeal 判断混在一起。产品输出应明确分为：

```text
Agent activity
Verification activity
Final decision
Next user action
```

## 7. 信任结论

本次 `ADMITTED` 能够支持的准确结论是：

> StateSeal tree `b90c221f8c2a` 对应的精确候选状态，在干净环境中满足受保护路径和 `go test ./...` 策略；用户应用并推送的 commit 与 verified checkpoint 完全一致，且 Git 身份未被 StateSeal 改写。

它不代表需求或测试覆盖了所有未知情况，也不代表本地主机绝对可信。Local receipt 的结构完整性同样不能替代 CI authority。

## 8. 产品判断

StateSeal 已经在真实开源项目上证明核心产品逻辑成立：

```text
用户定义目标
→ Agent 提出代码
→ StateSeal 独立验证精确状态
→ 用户查看证据
→ 用户应用 verified checkpoint
→ 精确状态完成推送
```

当前核心功能可以工作，但在面向普通用户发布 Alpha 前，应先修复 apply 后状态模型和重复终态认证，再优化终端信息层级。
