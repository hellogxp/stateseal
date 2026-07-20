# StateSeal 产品端到端测试报告

## 1. 测试结论

StateSeal 已在真实开源项目上完成一次端到端验证：用户提出开发目标，Codex 完成功能开发，StateSeal 在隔离环境中验证精确候选状态，生成可追溯的 checkpoint 与 receipt，用户确认后应用并推送交付分支。

**结论：核心可信交付链路通过；当前 Alpha 仍需修复状态显示和重复认证问题，之后再作为普通用户可体验版本发布。**

## 2. 测试场景

测试项目：`github.com/google/uuid`

用户目标：

> 新增 `ParseCanonical(string) (UUID, error)`，只接受小写、带连字符的标准 UUID；拒绝大写、紧凑格式、URN、花括号和首尾空格；保持现有 `Parse` 行为兼容。

验收要求：

| 项目 | 预期 |
| --- | --- |
| 标准小写 UUID | 接受 |
| 非标准表示形式 | 拒绝 |
| 原有 `Parse` 行为 | 不变 |
| 原有与新增测试 | 全部通过 |
| 需求、测试、策略、Hook | Agent 不得修改 |

## 3. 正常产品流程

```text
用户定义目标和验收要求
        │
        ▼
StateSeal 固定基线、策略和受保护文件
        │
        ▼
Codex 在隔离 Worktree 中开发和自测
        │
        ▼
StateSeal 固定候选代码快照并独立验证
        │
        ├─ 失败：拒绝交付并反馈原因
        │
        └─ 通过：生成 checkpoint + receipt
                         │
                         ▼
                  用户检查并应用
```

用户不需要在开发前手工验证代码。测试中对基线执行失败检查，仅用于证明需求原本没有实现，不属于正常产品操作。

## 4. 实际执行

本次真实测试使用的 UUID 项目目录是：

```text
/private/tmp/stateseal-product-test.fSp4Gi/uuid
```

这是目标代码仓库，不是 StateSeal 源码目录。StateSeal 自动创建的隔离 proposal 位于用户缓存目录，两者不能混用。

以下记录假设用户没有修改 `PATH`。所有 StateSeal 命令均使用本次构建产物的绝对路径：

```text
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal
```

### 4.1 进入目标项目

输入：

```bash
cd /private/tmp/stateseal-product-test.fSp4Gi/uuid
pwd
git status --short --branch
```

测试开始时的输出：

```text
/private/tmp/stateseal-product-test.fSp4Gi/uuid
## experience/uuid-canonical-baseline
```

完成 `seal apply` 后，该目录切换到 `feature/parse-canonical-product-test`。当前在本机执行 `git status --short --branch` 的输出为：

```text
## feature/parse-canonical-product-test
```

正常使用时，`init`、`adapter`、`doctor`、`run`、`status`、`diff` 和 `apply` 都应在这个目标项目目录中执行。

### 4.2 确认 StateSeal 版本

输入：

```bash
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal version
```

输出：

```text
StateSeal v0.0.0-20260717090553-db72814ae1f0
Commit:    db72814ae1f092966a4fa50fec95517c5a5aa5fa
Built:     2026-07-17T09:05:53Z
Go:        go1.26.3
Platform:  darwin/arm64
```

### 4.3 初始化项目

输入：

```bash
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal init \
  --goal "Add ParseCanonical to accept only lowercase hyphenated RFC 9562 UUIDs while preserving Parse compatibility."
```

输出：

```text
StateSeal initialized

Policy:    <repository>/seal.yaml
Task:      uuid
Verifier:  go test
Mode:      enforce (default)
```

安装 Codex 适配器：

```bash
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal adapter codex install
```

输出：

```text
Codex adapter installed at <repository>/.codex/hooks.json
Commit the file as protected infrastructure before the first managed run.
```

检查配置：

```bash
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal doctor
```

输出：

```text
✓ Git repository
✓ seal.yaml
✓ git executable
Platform: darwin/arm64
```

本次策略将 `REQUIREMENT.md`、验收测试、`seal.yaml`、`.git/**` 和 `.codex/**` 设为受保护路径，并将 `go test ./...` 配置为 admission 与 completion verifier。

### 4.4 启动受控开发

当前 Alpha 的实际输入：

```bash
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal run -- \
  codex exec \
  --ephemeral \
  --dangerously-bypass-hook-trust \
  -s workspace-write \
  "Implement the requirement in REQUIREMENT.md. Add exported ParseCanonical(string) (UUID, error). It must accept only lowercase hyphenated canonical RFC 9562 UUID text, reject all non-canonical representations covered by canonical_acceptance_test.go, preserve the existing permissive Parse behavior, and run go test ./.... Do not modify REQUIREMENT.md, any test file, seal.yaml, or agent hook configuration."
```

StateSeal 首先输出隔离工作区：

```text
Proposal worktree: <user-cache>/stateseal/f4b9e984eefaf906/uuid/proposal-d061a9e489f9
```

Codex 的执行过程：

```text
Agent activity
  Read REQUIREMENT.md and canonical_acceptance_test.go
  Inspected the existing Parse implementation
  Modified uuid.go and util.go
  Ran gofmt
  Ran go test ./...

Test result
  ok      github.com/google/uuid  0.997s
```

Codex 没有直接修改用户工作分支，也没有修改需求、测试、策略或 Hook。

随后 StateSeal 固定候选状态，在干净环境中独立执行 verifier，并输出最终判断：

```text
StateSeal — ADMITTED
Receipt: rcpt_46485983ae98cbaa9f630ba2
Tree:    b90c221f8c2a
Mode:    enforce (ALLOWED)
Residual risk:
  - Only configured checks were evaluated.
  - The execution host was not independently attested.
```

### 4.5 检查交付证据

输入：

```bash
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal status
```

输出：

```text
Task:       uuid
Status:     ADMITTED
Freshness:  CURRENT
Mode:       enforce
Coverage:   intermediate + terminal
Checkpoint: cp_d4bb816bc74b5b779a9b05c1
Tree:       b90c221f8c2a
Receipt:    rcpt_46485983ae98cbaa9f630ba2
```

输入：

```bash
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal explain
```

输出：

```text
Verdict: ADMITTED
Next: inspect with `seal diff`, then use `seal apply`
```

检查实际代码变更：

```bash
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal diff
```

关键输出：

```text
M util.go
M uuid.go
```

检查 receipt：

```bash
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal inspect \
  rcpt_46485983ae98cbaa9f630ba2
```

输出：

```text
Receipt rcpt_46485983ae98cbaa9f630ba2 is structurally intact.
Verdict: ADMITTED
Tree:    b90c221f8c2a
Note: local receipt integrity does not establish CI authority.
```

### 4.6 用户确认并应用

输入：

```bash
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal apply \
  --branch feature/parse-canonical-product-test
```

输出：

```text
Verified checkpoint applied.
```

应用后的 HEAD：

```text
b0ed471 Checkpoint candidate state
d061a9e Configure StateSeal admission policy
1653da9 Add canonical UUID acceptance specification
2d3c2a9 feat: Generate V6 from custom time (#172)
```

检查 Apply 后状态：

```bash
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal status
```

实际输出：

```text
Task:       uuid
Status:     ADMITTED
Freshness:  STALE
Stale:      trusted base changed after admission
Checkpoint: cp_d4bb816bc74b5b779a9b05c1
```

此时 HEAD 就是 verified checkpoint，因此该输出暴露了状态模型缺陷；预期应为 `APPLIED / CURRENT`。Apply 本身及代码一致性没有失败。

### 4.7 最终复验与推送

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

`go vet` 和 `git diff --check` 无错误输出。这里的 race 和 vet 是产品测试人员在 apply 后执行的额外复验，不属于 receipt 声明的证据范围。

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

远程分支最终指向 verified commit：

```text
b0ed47114fcaac416bd7b3d1fb2b950284c58066
refs/heads/feature/parse-canonical-product-test
```

### 4.8 目标产品入口

当前 Alpha 已跑通底层链路，但正式产品不应要求用户在初始化和 Agent 命令中重复输入目标。目标入口应进一步收敛为：

```bash
/Users/xuepinxueping.gxpg.gxp/paper-loop-engineering/stateseal/bin/seal run \
  --agent codex \
  --goal "Implement ParseCanonical according to REQUIREMENT.md"
```

该命令属于目标产品设计，本次测试使用的 Alpha 尚未实现这一语法。

## 5. 验证结果

| 验证项 | 结果 |
| --- | --- |
| Agent 隔离开发 | 通过 |
| 受保护文件未被修改 | 通过 |
| 标准 UUID 接受规则 | 通过 |
| 非标准格式拒绝规则 | 通过 |
| 原有行为兼容 | 通过 |
| `go test ./...` | 通过 |
| 完成阶段重新认证 | 通过 |
| checkpoint 与验证状态精确绑定 | 通过 |
| receipt 结构完整 | 通过 |
| Git 用户身份未被改写 | 通过 |
| 用户显式确认后应用 | 通过 |
| 远程分支与 verified checkpoint 一致 | 通过 |

关键交付证据：

| 对象 | 标识 |
| --- | --- |
| Verified commit | `b0ed47114fcaac416bd7b3d1fb2b950284c58066` |
| Checkpoint | `cp_d4bb816bc74b5b779a9b05c1` |
| StateSeal tree | `b90c221f8c2a` |
| Selected receipt | `rcpt_46485983ae98cbaa9f630ba2` |

## 6. 发现的问题

### P0：Apply 后状态错误

应用精确 checkpoint 后，`seal status` 错误显示 `STALE`。正确状态应为 `APPLIED / CURRENT`。

### P1：同一状态重复认证

同一候选 tree 被三个生命周期入口重复认证，产生三个等价 receipt。终态认证应只执行一次，相同 tree 应去重。

### P2：终端输出层级不清晰

Agent 日志、Hook 消息、验证过程和最终结论混合显示。正式输出应分为：Agent activity、Verification、Final decision、Next action。

## 7. 信任边界

本次 `ADMITTED` 准确表示：

> receipt 绑定的精确候选状态，在指定策略和干净验证环境中通过了受保护路径检查与 `go test ./...`；最终应用和推送的代码与该候选状态一致。

它不表示所有未知缺陷均已排除，也不表示本地主机已经获得独立可信证明。StateSeal 提供的是可验证、可追溯、范围明确的交付证据，而不是没有边界的“绝对正确”声明。

## 8. 发布判断

| 能力 | 判断 |
| --- | --- |
| 核心架构与产品价值 | 已验证 |
| Codex 真实项目接入 | 已验证 |
| 独立验证与证据绑定 | 已验证 |
| 精确 checkpoint 交付 | 已验证 |
| 普通用户交互完整度 | 尚需改进 |
| Alpha 发布准备度 | 修复 P0、P1 后可体验 |

下一开发优先级：修复 Apply 状态模型，合并终态生命周期并实现同 tree 去重，最后收敛单一任务入口和终端结果展示。
