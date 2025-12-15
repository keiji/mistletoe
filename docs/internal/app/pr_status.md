# `pr status` サブコマンド Design Doc (mstl-gh)

## 1. 概要 (Overview)

`pr status` サブコマンドは、各リポジトリに関連付けられた現在のプルリクエスト (PR) の情報を表示します。ローカルのステータス情報と GitHub 上の PR 情報を統合して表示します。

## 2. 使用方法 (Usage)

```bash
mstl-gh pr status [options]
```

### オプション (Options)

| オプション | 短縮形 | 説明 | デフォルト |
| :--- | :--- | :--- | :--- |
| `--file` | `-f` | 設定ファイル (JSON) のパス。 | `mistletoe.json` |
| `--parallel` | `-p` | 並列プロセス数。 | 1 |

## 3. 出力形式 (Output Format)

```text
+------------+-------+--------+---------------+--------+
| REPOSITORY | PR    | BASE   | BRANCH/REV    | STATUS |
+------------+-------+--------+---------------+--------+
| frontend   | #123  | main   | feature/ui    | OPEN   |
| backend    | -     | main   | feature/api   |   >    |
| tools      | #456  | develop| fix/bug       | MERGED |
+------------+-------+--------+---------------+--------+
```

*   **PR**: PR 番号。存在しない場合はハイフン。
*   **BASE**: PR のベースブランチ、または設定上のブランチ。
*   **STATUS**:
    *   PR がある場合: PR の状態 (OPEN, MERGED, CLOSED, DRAFT)。
    *   PR がない場合: ローカル Git ステータス ( `>` など)。

## 4. ロジックフロー (Logic Flow)

### 4.1. フローチャート (Flowchart)

```mermaid
flowchart TD
    Start(["開始"]) --> LoadConfig["設定ロード"]
    LoadConfig --> ExecLoop["並列実行ループ"]

    subgraph "情報収集"
        ExecLoop --> GitStatus["Gitステータス取得 (status同様)"]
        GitStatus --> GHList["gh pr list --head <current-branch>"]
        GHList --> Combine["情報統合"]
    end

    Combine --> RenderTable["テーブル描画"]
    RenderTable --> Stop(["終了"])
```

### 4.2. 統合ロジック

1.  `status` コマンドと同様に、ローカルおよびリモートの Git 情報の収集。
2.  `gh` CLI を使用して、現在のブランチに関連する PR の検索。
3.  PR が見つかった場合、そのステータス（State）を優先して表示。見つからない場合、Git の同期ステータスの表示。
