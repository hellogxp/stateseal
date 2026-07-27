# はじめに

[English](../getting-started.md) | 日本語

StateSeal の通常フローは、コンピューターへのインストール、Agent の統合、
プロジェクトの検証契約の確認、そしてタスク目標の入力です。

## インストールと統合

GitHub Actions が確定した Git tag から構築したリリースをインストールします。
インストーラーはプラットフォームを選択し、公開 SHA-256 を検証します。

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal version
seal integrate codex-desktop
seal integrate status
```

匿名アーティファクトまたはソースチェックアウトでは、Git と Go 1.24 以降を
用意して `make install` を実行します。

プロジェクトのルートで最初のタスクを開始します。

```bash
cd /path/to/project
seal run "入力検証を追加し、互換性を維持し、テストを含める"
```

初回は StateSeal が検出した admission、completion、保護対象を表示します。
そのコマンドが最低限の配信ゲートとして妥当な場合だけ確認してください。

StateSeal は隔離された proposal を作り、候補を独立して検証し、最後の検証済み
チェックポイントを保持し、終了時に新しい evaluator で再認証します。
`ADMITTED` の後も、ユーザーが明示的に適用するまで元のブランチは変更されません。

## 実行を確認

```bash
seal ui
```

Runs Console では、全リポジトリの状態来歴、検証証拠、イベント、完了レシートを
確認できます。詳しくは [Runs Console](runs-console.md) を参照してください。

`ADMITTED` は `seal.yaml` に記載されたチェックだけを対象とします。仕様の完全性、
リモート CI、ホストの信頼は残余リスクとして扱われます。
