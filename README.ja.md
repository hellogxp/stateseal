# StateSeal

[English](README.md) · [简体中文](README.zh-CN.md) · **日本語** ·
[한국어](README.ko.md) · [Español](README.es.md) ·
[Português do Brasil](README.pt-BR.md) · [Deutsch](README.de.md) ·
[Français](README.fr.md)

StateSeal は、Coding Agent が生成した候補を、正確なコード状態に結び付けられ、
独立して検証され、復旧・再認証・監査が可能な配信結果に変換します。検証範囲と
残余リスクも明示します。

> モデルが変更を提案し、証拠が判断を支え、Broker が承認を決定し、Git が履歴を保存し、ユーザーが適用します。

StateSeal は別の Coding Agent でも、テストの代替でもありません。既存の Agent と
検証コマンドを包み、配信されるコードが検証済みコードと完全に一致することを保証します。

![StateSeal の信頼済み配信パイプライン](docs/assets/stateseal-trust-pipeline.svg)

## 主な機能

| 機能 | 価値 |
| --- | --- |
| 正確な状態への結合 | 検証結果を、それを生成したコードツリーに結び付けます |
| 独立検証 | クリーンな evaluator でポリシーを実行し、Agent の自己申告に依存しません |
| チェックポイント復旧 | 後続候補が回帰しても、最後の信頼済み状態を保持します |
| 外部承認 | Broker が証拠、範囲、ポリシーから最終判断を行います |
| 監査可能なレシート | 来歴、証拠の出所、残余リスクを記録します |

## 必要な理由

長時間動作する Agent は、テスト合格後に編集を続けて回帰を導入しながら、成功を
報告する可能性があります。StateSeal は候補、証拠、チェックポイント、再認証、
承認を明示的なプロトコル状態として扱います。

```text
WORKING → CANDIDATE → VERIFYING → VERIFIED → RECERTIFYING → ADMITTED
                           ↘ REJECTED                 ↘ STALE / ABSTAINED
```

## クイックスタート

GitHub Actions が確定した Git tag から構築したリリースをインストールします。
インストーラーはプラットフォームを選択し、SHA-256 を検証します。

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal version
cd your-project
seal integrate codex-desktop
seal run "重複コールバックを修正し、互換性を維持する"
seal ui
```

匿名レビューまたはソースチェックアウトでは、Git と Go 1.24 以降を用意して
`make install` を実行できます。

Runs Console では、全リポジトリの実行、状態来歴 DAG、検証証拠、信頼済みイベント、
チェックポイント復旧、完了レシートを確認できます。UI は読み取り専用で、
ループバックアドレスだけを使用します。

![StateSeal Runs Console](docs/assets/stateseal-runs-console.svg)

## 統合と自動化

Codex Desktop、Codex CLI、Claude Code、Qoder 向けの統合を提供します。その他の
ターミナル Agent も `seal run -- <command>` で同じプロトコルを利用できます。
JSON 出力、安定したルール ID、完了レシートは CI や研究再現にも利用できます。

## 信頼境界

`ADMITTED` は、正確なチェックポイントが `seal.yaml` の検証ポリシーを満たしたこと
だけを意味します。仕様やテストの完全性、ホストの安全性は証明しません。

## ドキュメント

- [日本語ドキュメント](docs/ja/index.md)
- [はじめに](docs/ja/getting-started.md)
- [Runs Console](docs/ja/runs-console.md)
- [英語の技術リファレンス](docs/index.md)

プロトコル値、コマンド、digest、ログは監査上の意味を保つため英語のまま表示されます。
