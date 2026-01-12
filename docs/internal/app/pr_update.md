# `pr update` サブコマンド Design Doc (mstl-gh)

## 1. 概要 (Overview)

`pr update` サブコマンドは、既に存在するPull Request (PR) を更新します。
対象となるリポジトリ（OpenまたはDraft状態のPRが存在するもの）に対して、以下の処理を行います：
1.  **Push**: ローカルブランチがリモートより進んでいる場合、変更をPushします。
2.  **Description更新**: PRのDescriptionに含まれるMistletoeブロック（スナップショット情報および依存関係グラフ）を最新の状態に更新します。

新しいPRの作成は行いません。

## 2. 使用方法 (Usage)

```bash
mstl-gh pr update [options]
```

### オプション (Options)

| オプション | 短縮形 | 説明 | デフォルト |
| :--- | :--- | :--- | :--- |
| `--file` | `-f` | 設定ファイル (JSON) のパス。 | `.mstl/config.json` |
| `--dependencies` | `-d` | 依存関係グラフ（Mermaid形式）のMarkdownファイルパス。 | `.mstl/dependency-graph.md` |
| `--jobs` | `-j` | 並列実行数。 | 1 |
| `--overwrite` | `-w` | 既存PRの作成者が自分以外で、Mistletoeブロックがない場合でも上書きを許可する。 | false |
| `--ignore-stdin` | | 標準入力を無視する | false |
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
    CheckTarget -- "Yes" --> VerifyPush{"Pushが必要か？\n(Ahead)"}

    VerifyPush -- "Yes" --> ExecPush["Push実行"]
    VerifyPush -- "No" --> GenSnapshot["スナップショット生成"]
    ExecPush --> GenSnapshot

    GenSnapshot --> ExecUpdate["PR本文更新\n(スナップショット埋め込み)"]
    ExecUpdate --> ShowFinalStatus["最終ステータス表示"]
    ShowFinalStatus --> Stop
```

### 3.2. 状態判定とアクション (State Analysis & Actions)

ステータス収集後、各リポジトリの状態に基づいて処理を決定します。

1.  **Pullが必要 (Behind)**:
    *   **条件**: ローカルブランチがリモートブランチより遅れている。
    *   **アクション**: **エラー停止**。

2.  **競合 (Conflict)**:
    *   **条件**: マージ競合が発生している。
    *   **アクション**: **エラー停止**。

3.  **Pull Requestの更新 (Update PR)**:
    *   **条件**: 有効な（OpenまたはDraft状態の）Pull Requestが存在する。
    *   **アクション**:
        *   **Push**: ローカルがリモートより進んでいる場合 (`Ahead`)、`git push origin <branch>` を実行します。
        *   **Update**: DescriptionのMistletoeブロックを置換・追記します。

### 3.3. 依存関係の解析 (Dependency Parsing)

`pr create` と同様。

### 3.4. Mistletoe ブロック (Mistletoe Block)

PR 本文の更新仕様は `pr create` と完全に同一です。

### 3.5. 制約事項 (Constraints)

*   **GitHub のみ**: URL が GitHub を指していないリポジトリはスキップまたはエラー。
*   **クリーンな状態**: コンフリクトやBehind状態のリポジトリがある場合は実行できません。
*   **PR作成なし**: このコマンドは新しいPRを作成しません。新規作成が必要な場合は `pr create` を使用してください。

### 3.6. 更新対象外のPR (Exclusion of Closed/Merged PRs)

`MERGED` または `CLOSED` 状態の Pull Request は、更新対象から明示的に除外されます。これらが存在しても無視され、Open な PR がなければ「対象なし」として処理を終了します。
処理ロジックにおいても、APIコールによる再確認を行う前にリストから除外されるため、`MERGED` または `CLOSED` のPRに対して更新リクエストが送信されることはありません。

### 3.7. デバッグ (Debugging)

`--verbose` オプション指定時の挙動は `pr create` と同様です。
