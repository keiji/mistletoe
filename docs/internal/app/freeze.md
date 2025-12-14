# `freeze` サブコマンド Design Doc

## 1. 概要 (Overview)

`freeze` サブコマンドは、現在のディレクトリ直下にあるすべての Git リポジトリをスキャンし、その時点の状態（リモート URL、ブランチ、またはリビジョン）を記録した新しい設定ファイル（JSON）を生成します。現在の環境をスナップショットとして保存する際に使用します。

## 2. 使用方法 (Usage)

```bash
mstl freeze --file <output_path>
```

### オプション (Options)

| オプション | 短縮形 | 説明 | デフォルト |
| :--- | :--- | :--- | :--- |
| `--file` | `-f` | **(必須)** 出力する設定ファイルのパス。 | - |

## 3. 動作仕様 (Specifications)

1.  **ディレクトリ走査**: カレントディレクトリのすべてのサブディレクトリをチェックします。
2.  **Git 判定**: `.git` ディレクトリを持つものだけを対象とします。
3.  **情報抽出**:
    *   **ID**: ディレクトリ名を ID として使用します。
    *   **URL**: `git remote get-url origin` で取得します。失敗時は `git config --get remote.origin.url` を試行します。
    *   **Branch/Revision**:
        *   現在の HEAD がブランチを指している場合、そのブランチ名を `branch` フィールドに設定します。
        *   **Detached HEAD** 状態の場合（ブランチ名が "HEAD"）、現在のコミットハッシュを `revision` フィールドに設定し、`branch` は設定しません。
4.  **ファイル出力**:
    *   指定されたパスにファイルが既に存在する場合は、上書きせずにエラー終了します。
    *   出力形式はインデントされた JSON です。

## 4. 内部ロジック (Internal Logic)

### 4.1. フローチャート (Flowchart)

```mermaid
flowchart TD
    Start([開始]) --> ParseArgs[引数パース]
    ParseArgs --> CheckFile{出力ファイル指定あり？}
    CheckFile -- No --> ErrorArg[エラー: ファイル必須]
    CheckFile -- Yes --> CheckExist{ファイル存在？}

    CheckExist -- Yes --> ErrorExist[エラー: ファイル既存]
    CheckExist -- No --> ScanDir[カレントディレクトリ走査]

    ScanDir --> Loop[ディレクトリループ]

    subgraph "各ディレクトリの処理"
        Loop --> CheckGit{Is Git Repo?}
        CheckGit -- No --> Skip[スキップ]
        CheckGit -- Yes --> GetURL[Remote URL取得]
        GetURL --> GetHEAD[HEAD状態取得]

        GetHEAD --> CheckDetached{Detached HEAD?}
        CheckDetached -- No --> SetBranch[Branch設定]
        CheckDetached -- Yes --> SetRev[Revision設定]

        SetBranch --> AppendList[リストに追加]
        SetRev --> AppendList
    end

    Loop --> CheckEnd{全ディレクトリ完了？}
    CheckEnd -- No --> Loop
    CheckEnd -- Yes --> Marshal[JSON生成]
    Marshal --> WriteFile[ファイル書き込み]
    WriteFile --> End([終了])

    ErrorArg --> End
    ErrorExist --> End
```

### 4.2. 詳細ロジック

1.  **出力ファイルチェック**: `os.Stat` を使用して出力先ファイルが既に存在するか確認し、存在する場合は誤って上書きしないように終了します。
2.  **リポジトリ情報の構築**:
    *   `os.ReadDir(".")` でエントリを取得します。
    *   各ディレクトリについて、`rev-parse --abbrev-ref HEAD` を実行します。
        *   戻り値が `HEAD` 文字列そのものであれば、Detached HEAD とみなして `rev-parse HEAD` で完全なハッシュを取得し、`Revision` フィールドに格納します。
        *   それ以外の場合はブランチ名として `Branch` フィールドに格納します。
    *   設定ファイルの構造体（`Config`）において、`Branch` と `Revision` はポインタ型（`*string`）であるため、該当しないフィールドは `nil`（JSON 上では省略または null）として扱われます。これにより、`branch` と `revision` の相互排他性を表現します。
