# StateSeal

[English](README.md) · [简体中文](README.zh-CN.md) · **日本語** ·
[한국어](README.ko.md) · [Español](README.es.md) ·
[Português do Brasil](README.pt-BR.md) · [Deutsch](README.de.md) ·
[Français](README.fr.md)

StateSeal は Coding Agent 向けの**読み取り専用デリバリーインテリジェンス層**です。
リクエスト、観測されたコード状態、検査、成果物、失敗、Agent の完了報告を
リアルタイムの全体図に再構成します。

Agent の起動・誘導・停止・拒否・承認・Apply は行いません。Hook のインストール、
モデル通信のプロキシ、Prompt やブランチの変更も行いません。

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

表示する主張は `Observed`（直接観測）、`Derived`（決定的な関連付け）、
`Inferred`（誤る可能性のある診断仮説）に分類されます。

詳細は [English documentation](docs/index.md) を参照してください。
