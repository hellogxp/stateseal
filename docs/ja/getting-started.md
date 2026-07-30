# はじめに

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

StateSeal はローカルの Codex セッションを読み取り専用で検出します。Hook、MCP、
承認、Agent の制御は不要です。`COMPLETED` は Agent が完了を報告したことだけを
意味し、コードの正しさを保証しません。

分析は `Observed`、`Derived`、`Inferred` に分類されます。詳細は
[英語ガイド](../getting-started.md)を参照してください。
