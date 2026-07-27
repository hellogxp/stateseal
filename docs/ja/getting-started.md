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
codex plugin marketplace add /path/to/stateseal
codex plugin add stateseal@stateseal
```

Plugin は Skill と MCP 登録を同梱し、インストール済み Core を使用します。
MCP の別インストールは不要です。`marketplace add` はローカル checkout の登録
だけを行い、公開やアップロードはしません。新しい Codex タスクでは通常の
コード変更要求が自動かつ可視的に StateSeal を起動します。

匿名アーティファクトまたはソースチェックアウトでは、Git と Go 1.24 以降を
用意して `make install` を実行します。

Plugin を利用できない場合、CLI/headless ではリポジトリを明示します。

```bash
seal run --repo /path/to/project "入力検証を追加し、互換性を維持し、テストを含める"
```

非 Git の親 Workspace では子リポジトリを検出し、複数ある場合は
`seal workspace list` で確認して `--repo` で明示的に選択します。

最初のプロジェクト契約と最終 Apply は明示的に確認します。StateSeal 自身の
確認、MCP、Store、隔離 Worker が利用できない場合、通常の Agent 開発は停止せず、
結果を `UNVERIFIED` と表示して Receipt を発行しません。正常な enforce
ポリシーによる検証拒否は引き続き有効です。

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
