# StateSeal

[English](README.md) · **简体中文** · [日本語](README.ja.md) ·
[한국어](README.ko.md) · [Español](README.es.md) ·
[Português do Brasil](README.pt-BR.md) · [Deutsch](README.de.md) ·
[Français](README.fr.md)

StateSeal 将 Coding Agent 产生的候选代码转换为与精确代码状态绑定、经过独立验证、
可恢复、可重新认证并且可审计的交付结果，同时明确披露验证覆盖和剩余风险。

> 模型提出修改，证据支持判断，Broker 决定准入，Git 保存历史，用户确认应用。

StateSeal 不是另一个 Coding Agent，也不会替代测试。它包装现有 Agent 和验证命令，
确保最终交付的代码正是经过验证的代码。

![StateSeal 可信交付链路](docs/assets/stateseal-trust-pipeline.svg)

## 核心能力

| 能力 | 解决的问题 |
| --- | --- |
| 精确状态绑定 | 证明验证结果属于哪一个代码树，而不是“某次运行曾经通过” |
| 独立验证 | 在干净 evaluator 中执行项目策略，避免 Agent 自报成功 |
| 检查点恢复 | 后续候选回归时保留并重新认证最后一个可信状态 |
| 外部准入 | Broker 根据证据、覆盖和策略决定最终处置 |
| 可审计凭证 | 记录事件链、证据来源、覆盖范围和剩余风险 |

## 为什么需要 StateSeal

长时间运行的 Agent 可能先让测试通过，随后继续修改并引入回归，却仍然报告成功。
StateSeal 将候选状态、验证证据、检查点、重新认证和最终准入变成明确的协议状态：

```text
WORKING → CANDIDATE → VERIFYING → VERIFIED → RECERTIFYING → ADMITTED
                           ↘ REJECTED                 ↘ STALE / ABSTAINED
```

准入由外部 Broker 决定。失败的候选状态不会覆盖已经验证的检查点。

## 快速开始

推荐安装由 GitHub Actions 从确定 Git tag 构建的发行版；安装器会选择当前平台、
校验 SHA-256 后安装到 `~/.local/bin`：

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal version

cd your-project
seal integrate codex-desktop
seal run "修复重复回调，并保持现有接口兼容"
```

匿名评审或源码检出可在仓库中执行 `make install`，要求 Git 与 Go 1.24 或更高版本。

查看本机所有仓库的运行记录：

```bash
seal ui
```

Runs Console 提供实时列表、状态溯源 DAG、验证器证据、可信事件时间线、
检查点恢复过程和带密码学摘要的完成凭证。UI 只读，并且只监听本机回环地址。

![StateSeal Runs Console](docs/assets/stateseal-runs-console.svg)

## 适配与自动化

StateSeal 支持 Codex Desktop、Codex CLI、Claude Code 和 Qoder 的集成路径；
其他终端 Agent 可通过 `seal run -- <command>` 使用同一验证协议。结构化 JSON、
稳定规则 ID 和完成凭证可用于 CI、评测与研究复现。

## 安全边界

`ADMITTED` 只表示精确检查点通过了 `seal.yaml` 中配置的验证策略。它不证明需求或
测试套件完整，也不证明执行主机未被攻破。StateSeal 因此始终报告验证覆盖、证据来源
和剩余风险；用户分支在显式应用成功前保持不变。

## 文档

- [中文文档中心](docs/zh-CN/index.md)
- [快速上手](docs/zh-CN/getting-started.md)
- [Runs Console](docs/zh-CN/runs-console.md)
- [产品全景](docs/zh-CN/product-map.md)
- [英文技术参考](docs/index.md)

协议字段、命令、digest 和日志保留英文原值，避免审计语义在翻译中发生变化。
