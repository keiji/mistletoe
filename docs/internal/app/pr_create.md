# `pr create` サブコマンド Design Doc (mstl-gh)

## 1. 概要 (Overview)

`pr create` サブコマンドは、複数のリポジトリに対して一括でプルリクエスト (PR) を作成します。PR の本文には、全リポジトリの状態を記録したスナップショット情報が自動的に埋め込まれます。
また、依存関係グラフ（Mermaid形式）を指定することで、各PRの本文に関連するPRへのリンクを依存関係（依存・被依存）に基づいて分類して記載することができます。

## 2. 使用方法 (Usage)

```bash
mstl-gh pr create [options]
```

### オプション (Options)

| オプション | 短縮形 | 説明 | デフォルト |
| :--- | :--- | :--- | :--- |
| `--title` | `-t` | PR のタイトル。 | (エディタで入力) |
| `--body` | `-b` | PR の本文。 | (エディタで入力) |
| `--file` | `-f` | 設定ファイル (JSON) のパス。 | `.mstl/config.json` |
| `--dependencies` | | 依存関係グラフ（Mermaid形式）のMarkdownファイルパス。 | `.mstl/dependency-graph.md` |
| `--draft` | | ドラフトPRとして作成（リポジトリが対応している場合）。 | false |
| `--jobs` | `-j` | 並列実行数。 | 1 |
| `--overwrite` | `-w` | 既存PRの作成者が自分以外で、Mistletoeブロックがない場合でも上書きを許可する。 | false |
| `--ignore-stdin` | | 標準入力を無視する | false |
| `--verbose` | `-v` | デバッグ用の詳細ログを出力（実行された git/gh コマンドを表示） | false |

**注意**: 同じ種類のオプション（例: `--file` と `-f`）が同時に異なる値で指定された場合はエラーとなります。
**注意**: コマンドラインオプション（例: `--jobs`）は、設定ファイル（`config.json`）内の対応する設定値よりも優先されます。
**注意**: `--dependencies` オプションで指定されたファイルは、デフォルトの `.mstl/dependency-graph.md` よりも優先されます。

## 3. ロジックフロー (Logic Flow)

### 3.1. フローチャート (Flowchart)

```mermaid
flowchart TD
    Start(["開始"]) --> ParseArgs["引数パース"]
    ParseArgs --> ValidateFlags{"オプション整合性チェック"}
    ValidateFlags -- "エラー" --> Stop(["終了"])
    ValidateFlags -- "OK" --> LoadConfigSub[["設定読み込み"]]
    LoadConfigSub --> LoadDep[["依存関係グラフ読み込み (Optional)"]]
    LoadDep --> ValidateAuth["gh CLI認証確認"]
    ValidateAuth --> CollectStatus["ステータス・PR状況収集 (Spinner)"]
    CollectStatus --> RenderTable["pr status テーブル表示"]
    RenderTable --> AnalyzeState[["状態解析 (Behind/Ahead/Equal, PR有無)"]]

    AnalyzeState --> CheckBehind{Pullが必要な\nリポジトリがあるか？\n(Behind)}
    CheckBehind -- "Yes" --> ErrorAbort["エラー停止: Pullが必要です"]

    CheckBehind -- "No" --> Categorize[["アクション分類"]]
    Categorize --> CatPushNeed["Push必要リスト\n(Ahead or PR更新)"]
    Categorize --> CatCreateNeed["PR作成必要リスト\n(Ahead & PRなし)"]
    Categorize --> CatUpdateNeed["PR更新必要リスト\n(PRあり)"]
    Categorize --> CatSkip["スキップリスト\n(Equal or NewBranchNoCommit)"]

    CatSkip --> CheckWorkable{"処理対象リポジトリがあるか？\n(Create or Update)"}
    CheckWorkable -- "No" --> Stop(["終了"])
    CheckWorkable -- "Yes" --> CheckPermissions{"既存PR更新権限確認\n(後述詳細)"}

    CheckPermissions -- "NG" --> ErrorPermission["エラー停止: 権限なし/上書き不可"]
    CheckPermissions -- "OK" --> CheckDraft{"Draftオプション有効？"}

    CheckDraft -- "Yes" --> MarkDraft["ドラフト作成モード"]
    CheckDraft -- "No" --> CheckAllPRs{"処理対象全リポジトリに\n既存PRが存在するか？"}
    MarkDraft --> CheckAllPRs

    CheckAllPRs -- "Yes (Updateのみ)" --> PromptUpdate["プロンプト: 説明を更新しますか？"]
    PromptUpdate -- "No" --> Stop
    PromptUpdate -- "Yes" --> SetSkipEditor["エディタ起動スキップ"]

    CheckAllPRs -- "No (Create含む)" --> PromptCreate["プロンプト: 作成しますか？"]
    PromptCreate -- "No" --> Stop
    PromptCreate -- "Yes" --> SetNoSkip["エディタ起動有効"]

    SetSkipEditor --> VerifyBase["GitHub権限・Baseブランチ確認"]
    SetNoSkip --> VerifyBase

    VerifyBase -- "エラー" --> ErrorState["エラー停止"]
    VerifyBase -- "OK" --> CheckEditor{"エディタ起動？"}
    CheckEditor -- "Yes" --> InputContent["タイトル・本文入力 (エディタ)\n(解析ルール適用)"]
    CheckEditor -- "No" --> GenSnapshot["スナップショット生成"]
    InputContent --> GenSnapshot

    GenSnapshot --> VerifyRevisions[["状態再確認 (リビジョン変動なし)"]]
    VerifyRevisions -- "変更あり" --> ErrorChanged["エラー停止: 状態不整合"]
    VerifyRevisions -- "OK" --> ExecPush["Push実行 (Push必要リスト)"]
    ExecPush --> ExecCreate["PR作成実行 (PR作成必要リスト)\n(Draft試行 -> 失敗時通常作成)"]
    ExecCreate --> ExecUpdate["PR本文更新 (作成済み+更新リスト)\n(スナップショット埋め込み)"]
    ExecUpdate --> ShowFinalStatus["最終ステータス表示"]
    ShowFinalStatus --> Stop
```

### 3.2. 状態判定とアクション分類 (State Analysis & Categorization)

ステータス収集後、各リポジトリの状態に基づいてアクションを以下の優先順位で分類します。

1.  **Pullが必要 (Behind)**:
    *   **条件**: ローカルブランチがリモートブランチより遅れている（リモートにありローカルにないコミットがある）。
    *   **アクション**: **エラー停止**。処理を中断し、対象リポジトリとブランチを表示します。

2.  **Pull Requestの更新 (Update PR)**:
    *   **条件**: リモートからBaseブランチへの有効な（Open状態の）Pull Requestが既に存在する。
    *   **アクション**:
        *   **Push**: ローカルがリモートより進んでいる場合 (`Ahead`) はPushリストに追加します。進んでいない場合 (`Equal`) はPush不要ですが、実装上はPushリストに含めても安全です（No-op）。
        *   **Update**: 最終工程でPR本文（Mistletoeブロック）を更新します。

3.  **Pull Requestの作成 (Create PR)**:
    *   **条件**: 有効なPRが存在せず、かつローカルブランチがリモートブランチより進んでいる (`Ahead`)。
    *   **アクション**:
        *   **Push**: Pushリストに追加します。
        *   **Create**: PR作成リストに追加します。
        *   **Update**: 作成後の最終工程でPR本文を更新します。

4.  **スキップ (No Action)**:
    *   **条件A**: 有効なPRが存在せず、かつローカルブランチとリモートブランチが同期している (`Equal`)。
    *   **条件B**: 有効なPRが存在せず、ローカルブランチと同名のリモートブランチが存在せず、かつローカルブランチのリビジョンがBaseブランチのリビジョンと一致している（「ブランチは作成したがコミットはしていない」状態）。
    *   **アクション**: PushもPR作成もしません。メモリ上で「PR不要」として保持し、後続の処理から除外します。

### 3.3. 既存PRの権限確認と上書きルール (Permission Check & Overwrite Rules)

既存の Pull Request が存在する場合、処理を開始する前に以下の権限確認と上書き判定を行います。

1.  **編集権限の確認**:
    *   現在のユーザーがその Pull Request に対して編集権限 (`viewerCanEditFiles`) を持っているか確認します。
    *   権限がない場合、エラーメッセージを表示して処理を中止します。

2.  **上書き判定**:
    *   編集権限がある場合、以下の条件で上書き（Mistletoeブロックの追記・更新）可否を判定します。
        *   **Mistletoeブロックあり**: 既存PRに既に Mistletoe ブロックが存在する場合、**上書き可能**と判断します。
        *   **Mistletoeブロックなし & 作成者が自分**: Mistletoe ブロックがなく、PR作成者が現在のユーザーである場合、**上書き可能**と判断します。
        *   **Mistletoeブロックなし & 作成者が他人**: Mistletoe ブロックがなく、PR作成者が現在のユーザーでない場合：
            *   `--overwrite` (`-w`) オプションが指定されていれば、**上書き可能**と判断します。
            *   指定されていない場合、**エラー**として処理を中止し、`--overwrite` オプションの使用を促します。

### 3.4. 既存PRの更新スキップ条件 (Skip Update for Closed/Merged PRs)

既存のPull Requestが存在する場合でも、そのステータスが `MERGED` または `CLOSED` である場合、Descriptionの更新（スナップショットの埋め込み）は**明示的にスキップ**されます。
これらのPRは、ステータス収集時には「既存PRあり」として検出されますが、本文更新処理（`updatePrDescriptions`）の段階でフィルタリングされ、APIコールを行わずに無視されます。したがって、Open（またはDraft）状態のPRのみが更新対象となります。

### 3.5. 依存関係の解析 (Dependency Parsing)

`--dependencies` オプションで指定されたファイルは以下のルールで解析されます：
*   **形式**: Markdownファイル内のMermaidグラフ（`graph` または `flowchart`）。
*   **矢印と線種**:
    *   有効な有向矢印（Dependency）として、`-->`, `==>`, `-.->` など、末尾が `>` で終わる有向線を抽出します。
    *   `-- "label" -->` や `== label ==>` のようにラベルや装飾が含まれていても、有向矢印であれば依存関係として認識します。
    *   `--o`, `--x`, `---` など、末尾が `>` でない（有向でない）線は無視されます。
*   **方向**:
    *   `A --> B`: A は B に依存する。
    *   `A <--> B`: 相互依存。AはBに依存し、かつBはAに依存する（始点が `<` で始まる場合）。
*   **ノードID**:
    *   `ID["Label"]` や `ID{Label}` の形式であっても、先頭のID部分のみを使用して照合します。
*   **検証**: グラフ内の抽出されたノードIDは、設定ファイル (`repositories` 内の `id`) と一致する必要があります。一致しないIDが含まれる場合はエラーとして終了します。

### 3.6. Mistletoe ブロック (Mistletoe Block)

PR 本文の末尾に、自動生成された不可視（または折りたたみ）ブロックを追加します。

```markdown
(ユーザー入力本文)

------------------ (ランダムな区切り線)
## Mistletoe

### Related Pull Request(s)
...

<details><summary>mistletoe-related-pr-[identifier].json</summary>
... JSON Data ...
</details>

### snapshot

<details><summary>mistletoe-snapshot-[identifier].json</summary>
... JSON Snapshot Data ...
</details>

```
(Base64 encoded JSON data)
```

-------------------------------------
```

区切り線の長さ計算:
*   上部区切り線 (Top): 長さ `N` (4以上16以下のランダムな整数)
*   下部区切り線 (Bottom):
    *   `N` が奇数の場合: `N * 2 - 2`
    *   `N` が偶数の場合: `N * 2 - 1`

ブロック構成要素:
1.  **関連 PR リンク**:
    *   他のリポジトリで作成された関連 PR へのリンク一覧。
    *   `--dependencies` が指定された場合、以下のセクションに分類して記載されます：
        *   **Dependencies**: このリポジトリが依存している先のリポジトリのPR。
        *   **Used by**: このリポジトリに依存している元のリポジトリのPR。
        *   **Related to**: 依存関係がない、またはグラフに含まれないリポジトリのPR。
            *   **注記**: Dependencies と Used by が存在せず、Related to のみが存在する場合、"Related to" の小見出しは省略され、リストが直接表示されます。
    *   指定がない場合は、単一のリストとして全関連PRを表示します。
2.  **関連 PR リンク (JSON形式, `<details>` 内)**:
    *   ファイル名: `mistletoe-related-pr-[identifier].json`
    *   内容: 関連PRのURLを分類したJSONデータ。
        ```json
        {
            "dependencies": ["..."],
            "dependents": ["..."],
            "others": ["..."]
        }
        ```
3.  **JSON スナップショット (`<details>` 内)**:
    *   ファイル名: `mistletoe-snapshot-[identifier].json`
    *   内容: 整形された JSON データ。スナップショット内の `base-branch` には設定ファイルの `base-branch`（なければ `branch`）が反映されます。
4.  **Base64 エンコードデータ (コードブロック)**:
    *   目的: 自動処理用の機械可読データの提供。
    *   内容: スナップショット JSON の Base64 エンコード文字列。
    *   **注記**: `<details>` タグの外側に配置し、コードブロックで囲みます。
5.  **依存関係グラフ (`<details>` 内, Optional)**:
    *   条件: `--dependencies` オプション指定時。
    *   summary: `mistletoe-dependencies-[identifier].mmd`
    *   内容: 指定されたMermaidグラフの生データ（`mermaid` コードブロック内）。GitHub上でプレビュー表示されます。

これにより、レビュー担当者はスナップショット情報を参照でき、将来的な自動検証やリンク連携が可能になります。

### 3.7. 制約事項 (Constraints)

*   **GitHub のみ**: URL が GitHub を指していないリポジトリはスキップまたはエラー。
*   **クリーンな状態**: 全てのリポジトリが最新（Up-to-date）であり、ローカルの変更がないことが推奨されますが、実装上は「プッシュ可能であること」の確認。
*   **Detached HEAD 禁止**: ブランチ上にいない（Detached HEAD）リポジトリがある場合、PR 作成先が不明確なためエラー。
*   **Baseブランチの存在**: PRの作成先となるBaseブランチが設定ファイルに指定（`base-branch` 優先、なければ `branch`）されており、かつリモートに存在しない場合、エラーとして終了します。

### 3.8. 状態の再確認 (Final Verification)

Push実行の直前に、各リポジトリの現在のリビジョン (HEAD) が、処理開始時に収集したステータスと一致しているかを再確認します。
これは、エディタ入力中などの待機時間に、バックグラウンドや別ターミナルでリポジトリの状態が変更されていないことを保証するための安全措置です。

*   **一致**: 処理を続行します。
*   **不一致**: エラーメッセージを表示して処理を中止します。

### 3.9. デバッグ (Debugging)

`--verbose` オプションが指定された場合、実行される `git` および `gh` コマンドが標準エラー出力に出力されます。

### 3.10. エディタ入力の解析ルール (Editor Input Parsing Rules)

エディタから入力されたテキストは、以下のルールに従って「タイトル」と「本文」に分割されます。
PRタイトルの最大文字数 (`PrTitleMaxLength`) は 256 文字です。

1.  **1行目が最大文字数を超える場合**:
    *   **タイトル**: 1行目を `PrTitleMaxLength - 3` 文字で切り出し、末尾に `...` を付与します。
    *   **本文**: 入力されたテキスト全体（1行目を含む）。
2.  **1行目の直下が空行の場合** (標準的な Git コミットメッセージ形式):
    *   **タイトル**: 1行目。
    *   **本文**: 3行目以降のすべてのテキスト。
3.  **上記以外**:
    *   **タイトル**: 1行目。
    *   **本文**: 入力されたテキスト全体（1行目を含む）。

**例**:
*   *入力*:
    ```
    Title line

    Body line 1
    Body line 2
    ```
    *解析結果*: Title="Title line", Body="Body line 1\nBody line 2"

*   *入力*:
    ```
    Title line
    Body line 1
    Body line 2
    ```
    *解析結果*: Title="Title line", Body="Title line\nBody line 1\nBody line 2"
