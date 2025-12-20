# `pr update` サブコマンド Design Doc (mstl-gh)

## 1. 概要 (Overview)

`pr update` サブコマンドは、既に存在するPull Request (PR) のDescriptionに含まれるMistletoeブロック（スナップショット情報および依存関係グラフ）を更新します。
新しいPRの作成や、GitリポジトリへのPushは行いません。
ローカルリポジトリの状態に基づき、最新のコミットハッシュ情報と指定された依存関係グラフを既存のPRに反映させるために使用します。

## 2. 使用方法 (Usage)

```bash
mstl-gh pr update [options]
```

### オプション (Options)

| オプション | 短縮形 | 説明 | デフォルト |
| :--- | :--- | :--- | :--- |
| `--file` | `-f` | 設定ファイル (JSON) のパス。 | `mistletoe.json` |
| `--dependencies` | `-d` | 依存関係グラフ（Mermaid形式）のMarkdownファイルパス。 | (なし) |
| `--parallel` | `-p` | 並列実行数。 | 1 |
| `--verbose` | `-v` | デバッグ用の詳細ログを出力（実行された git/gh コマンドを表示） | false |

## 3. ロジックフロー (Logic Flow)

### 3.1. フローチャート (Flowchart)

```mermaid
flowchart TD
    Start(["開始"]) --> LoadConfigSub[["設定読み込み"]]
    LoadConfigSub --> LoadDep[["依存関係グラフ読み込み (Optional)"]]
    LoadDep --> ValidateAuth["gh CLI認証確認"]
    ValidateAuth --> CollectStatus["ステータス・PR状況収集 (Spinner)"]
    CollectStatus --> RenderTable["pr status テーブル表示"]
    RenderTable --> AnalyzeState[["状態解析 (Behind/Conflict, PR有無)"]]

    AnalyzeState --> CheckBehind{Pullが必要な\nリポジトリがあるか？\n(Behind)}
    CheckBehind -- "Yes" --> ErrorAbort["エラー停止: Pullが必要です"]

    CheckBehind -- "No" --> Categorize[["更新対象特定"]]
    Categorize --> CheckTarget{"更新可能なPRがあるか？\n(Open/Draft)"}

    CheckTarget -- "No" --> Stop(["終了 (更新対象なし)"])
    CheckTarget -- "Yes" --> GenSnapshot["スナップショット生成"]

    GenSnapshot --> ExecUpdate["PR本文更新\n(スナップショット埋め込み)"]
    ExecUpdate --> ShowFinalStatus["最終ステータス表示"]
    ShowFinalStatus --> Stop
```

### 3.2. 状態判定とアクション (State Analysis & Actions)

ステータス収集後、各リポジトリの状態に基づいて処理を決定します。

1.  **Pullが必要 (Behind)**:
    *   **条件**: ローカルブランチがリモートブランチより遅れている（リモートにありローカルにないコミットがある）。
    *   **アクション**: **エラー停止**。スナップショットが古くなる可能性があるため、更新前にPullを要求します。

2.  **競合 (Conflict)**:
    *   **条件**: マージ競合が発生している。
    *   **アクション**: **エラー停止**。

3.  **Detached HEAD**:
    *   **条件**: ブランチ上にいない。
    *   **アクション**: **エラー停止**。

4.  **Pull Requestの更新 (Update PR)**:
    *   **条件**: 有効な（OpenまたはDraft状態の）Pull Requestが存在する。
    *   **アクション**:
        *   スナップショットを生成し、PRのDescriptionにあるMistletoeブロックを置換（なければ追記）します。
        *   `--dependencies` が指定されている場合、依存関係グラフも更新されます。
        *   `create` コマンドとは異なり、ローカルが `Ahead` であってもPushは行いません。あくまでメタデータの更新のみです。

### 3.3. 依存関係の解析 (Dependency Parsing)

`pr create` と同様のルールで依存関係ファイルを解析します。

*   **形式**: Markdownファイル内のMermaidグラフ。
*   **検証**: グラフ内のノードIDは、設定ファイル内のリポジトリIDと一致する必要があります。

### 3.4. Mistletoe ブロック (Mistletoe Block)

PR 本文の更新仕様は `pr create` と完全に同一です。
既存のMistletoeブロック（`## Mistletoe` ヘッダーと区切り線で識別）が存在する場合はその範囲を置換し、存在しない場合は末尾に追記します。

### 3.5. 制約事項 (Constraints)

*   **GitHub のみ**: URL が GitHub を指していないリポジトリはスキップまたはエラー。
*   **クリーンな状態**: コンフリクトやBehind状態のリポジトリがある場合は実行できません。
*   **Pushなし**: このコマンドはコードのPushを行いません。コードの変更を反映させたい場合は `git push` を手動で行うか、`pr create` を使用してください。

### 3.6. デバッグ (Debugging)

`--verbose` オプション指定時の挙動は `pr create` と同様です。
