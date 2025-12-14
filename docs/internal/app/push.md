# `push` サブコマンド Design Doc

## 1. 概要 (Overview)

`push` サブコマンドは、ローカルの変更をリモートリポジトリに送信します。実行前に全リポジトリのステータスを確認し、競合やプルが必要な状態であれば安全のために処理を中断します。ユーザーの確認を経て、未プッシュの変更があるすべてのリポジトリに対して並列に `git push` を実行します。

## 2. 使用方法 (Usage)

```bash
mstl push [options]
```

### オプション (Options)

| オプション | 短縮形 | 説明 | デフォルト |
| :--- | :--- | :--- | :--- |
| `--file` | `-f` | 設定ファイル (JSON) のパス。 | - |
| `--parallel` | `-p` | ステータス確認およびプッシュ時の並列プロセス数。 | 1 |

## 3. 動作仕様 (Specifications)

1.  **事前チェック**: `status` コマンドと同様のロジックで全リポジトリの状態を確認します。
2.  **安全性確保**:
    *   **競合 (Conflict)** が検出された場合、エラーメッセージを表示して直ちに終了します。
    *   **同期が必要 (Pullable)** なリポジトリがある場合、エラーメッセージを表示して直ちに終了します。
3.  **ユーザー確認**: プッシュ対象（未プッシュの変更があるリポジトリ）が存在する場合、ユーザーに実行確認（`y/yes`）を求めます。
4.  **プッシュ実行**: 確認が得られた場合、対象リポジトリに対して並列にプッシュを実行します。

## 4. 内部ロジック (Internal Logic)

### 4.1. フローチャート (Flowchart)

```mermaid
flowchart TD
    Start([開始]) --> ParseArgs[引数パース]
    ParseArgs --> LoadConfig[設定読み込み]
    LoadConfig --> ValidateEnv[環境検証]
    ValidateEnv --> CollectStatus[ステータス収集 (statusロジック再利用)]

    CollectStatus --> RenderTable[ステータステーブル表示]
    RenderTable --> CheckSafety{安全チェック}

    CheckSafety -- "Conflictあり" --> ErrorConflict[エラー: Conflicts detected]
    CheckSafety -- "Pullableあり" --> ErrorSync[エラー: Sync required]

    CheckSafety -- "問題なし" --> FilterPushable[Push対象抽出 (Unpushed)]
    FilterPushable --> CheckEmpty{対象あり？}
    CheckEmpty -- No --> MsgEmpty[メッセージ: No repositories to push] --> End([終了])

    CheckEmpty -- Yes --> UserPrompt{ユーザー確認 (y/n)}
    UserPrompt -- No --> End
    UserPrompt -- Yes --> PushLoop[並列プッシュ実行]

    subgraph "プッシュ実行"
        PushLoop --> GitPush[git push origin branch]
        GitPush --> ResultLog[結果表示]
    end

    ResultLog --> End
```

### 4.2. 詳細ロジック

1.  **ステータス収集と検証**:
    *   `status_logic.go` の `CollectStatus` 関数を再利用して、全リポジトリの最新状態を取得します。
    *   取得した結果（`StatusRow` のスライス）を走査し、`HasConflict` が true のリポジトリがあれば "Conflicts detected. Cannot push." で終了します。
    *   `IsPullable` が true のリポジトリがあれば "Sync required." で終了します。

2.  **対象抽出**:
    *   `HasUnpushed` が true のリポジトリのみを抽出してリスト化します。

3.  **対話的実行**:
    *   `bufio.NewReader` を使用して標準入力からユーザーの承諾を得ます。
    *   `git push origin <branch_name>` コマンドを実行します。この際、`RunGitInteractive` ラッパー（または同等のエラーハンドリング付き実行関数）を使用し、失敗時はエラーを出力しますが、他のリポジトリのプッシュは続行します。
