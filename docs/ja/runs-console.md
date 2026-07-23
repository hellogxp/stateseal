# Runs Console

[English](../runs-console.md) | 日本語

`seal ui` は、外部 authority state を読み取り専用で可視化する StateSeal の
ローカル観測画面です。

```bash
seal ui
seal ui --no-open
seal ui --address 127.0.0.1:9137
```

一覧では全リポジトリの実行を検索・絞り込みできます。詳細画面には次が含まれます。

- 候補、検証証拠、チェックポイント、復旧、判断、適用の状態来歴 DAG
- 正確なコード状態に結び付いた検証実行
- ハッシュチェーンを検証したイベントタイムライン
- verdict、rule、disposition、checkpoint を含む完了レシート

単純な実行では空の層を自動的に圧縮します。長いノード文字列は境界内で省略され、
全文はツールチップと Inspector に表示されます。

UI は 8 言語に対応し、ブラウザー言語を自動検出します。Goal、プロトコル値、
ID、digest、ログは監査内容として原文を保持します。

サーバーは loopback のみにバインドし、書き込み API を持ちません。イベントの
ハッシュ連続性に問題があれば、そのタイムラインは信頼済みとして表示されません。
