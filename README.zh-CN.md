# StateSeal

[English](README.md) · **简体中文** · [日本語](README.ja.md) ·
[한국어](README.ko.md) · [Español](README.es.md) ·
[Português do Brasil](README.pt-BR.md) · [Deutsch](README.de.md) ·
[Français](README.fr.md)

StateSeal 是面向 Coding Agent 的**只读运行时智能层**。它把一次交付中的用户请求、
Skill、工具、产物、失败和 Agent 报告结果重建成实时全景图，但不启动、不引导、
不阻断、不审批，也不应用 Agent 的工作。

> 观察执行，解释链路，但绝不成为执行的一部分。

![StateSeal 运行全景图](docs/assets/stateseal-runs-console.svg)

## 解决什么问题

Agent 的运行证据通常分散在消息、工具调用、文件、测试输出和子 Agent 中。
StateSeal 帮助开发者快速回答：

- 预期的 Skill 是否被发现并真正使用；
- 哪些工具按什么顺序执行，哪里失败；
- 生成或修改了哪些文件和产物；
- 哪些是明确观测，哪些是派生归因，哪些只是诊断推断；
- 时间花在哪里，失败会话应该从哪里排查。

## 不可破坏的产品边界

```text
Agent 自有会话日志 + 本地产物
                ↓  只读
          标准化与关联
                ↓
      全景 · 归因 · 诊断 · 审计
```

StateSeal 不安装生命周期 Hook、不代理模型请求、不启动或接管 Agent、不修改 Prompt、
不返回 deny/block、不审批交付、不修改分支、不应用代码。执行及其结果始终由 Agent
和用户负责。

所有分析均标注证据等级：

| 等级 | 含义 |
| --- | --- |
| **Observed / 明确观测** | 直接来自 Agent 会话或本地运行事件 |
| **Derived / 派生** | 根据调用 ID、路径和时间确定性关联 |
| **Inferred / 推断** | 诊断假设，可用于排查，但不冒充事实 |

## 快速开始

源码安装需要 Go 1.24 或更高版本：

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

也可以安装发布版本：

```bash
curl --proto '=https' --tlsv1.2 -fsSL \
  https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

UI 只监听本机回环地址，旁路发现 Codex 本地 JSONL 会话，并在 Agent 运行时持续刷新。
默认不展示原始工具参数和完整输出。

## 当前能力

- Codex 本地会话的无 Hook、只读发现；
- 实时会话总览和事件时间线；
- 请求 → Skill → 工具 → 观测结果的归因 DAG；
- 明确区分 Observed / Derived / Inferred；
- 隐私优先摘要和常见密钥脱敏；
- 以只读历史档案展示旧版 StateSeal 证据；
- 八种 UI 语言。

## 默认命令

`seal ui` 打开运行全景图，`seal workspace list` 只读发现仓库，
`seal version` 显示构建身份。

受控执行、验证 Gate、准入、Apply、恢复、生命周期 Hook 安装和交付控制 MCP
工具均不再注册。

## 文档

- [文档中心](docs/zh-CN/index.md)
- [快速上手](docs/zh-CN/getting-started.md)
- [运行全景图](docs/zh-CN/runs-console.md)
- [产品全景](docs/zh-CN/product-map.md)
- [英文观察模型](docs/observation-model.md)

Apache-2.0 License。
