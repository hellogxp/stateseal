# 快速上手

源码构建需要 Go 1.24 或更高版本：

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

StateSeal 会打开本地页面并旁路发现近期 Codex 会话。不需要初始化项目，不安装
Agent Hook，不注册交付控制 MCP，也不需要审批。

```bash
seal ui --address 127.0.0.1:9137
seal ui --no-open
```

全景图按以下链路展示：

```text
Agent 会话 → Skill 归因 → 工具调用 → 观测结果
```

- **Observed**：会话中直接存在的事件；
- **Derived**：根据 ID、路径和时间确定性关联；
- **Inferred**：可能出错的诊断假设。

“已完成”只表示 Agent 发出了会话完成事件，不代表代码语义正确、可以上线或已经审批。

UI 只监听本机回环地址，默认隐藏原始工具参数和输出，并对常见密钥模式进行脱敏。

如果旧版安装过生命周期 Hook 或 StateSeal MCP 配置，应从 Agent 配置中删除。升级后
遗留 Hook 命令会保持静默并始终返回中性结果。
