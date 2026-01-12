# GitHub Pull Request 更新テスト設計書

本ドキュメントは、`mstl-gh` の `pr update` コマンドを使用して、既存の Pull Request の Mistletoe ブロック（依存関係情報など）を更新する機能の手動テスト手順を定義します。

## 概要

このテストでは、以下のシナリオを検証します。

1.  4つのリポジトリ（A, B, C, D）で Pull Request を作成する（A -> B -> C, D は独立）。
2.  ローカルの依存関係定義（`dependency-graph.md`）を変更し、D が A に依存するようにする（D -> A）。
3.  `pr update` を実行する。
4.  リポジトリ D の Pull Request が更新され、A への依存関係が表示されることを確認する。

## テスト構成

*   **リポジトリ構成**: 4つのリポジトリ（A, B, C, D）。
*   **初期依存関係**: A -> B -> C, D (独立)。
*   **変更後依存関係**: A -> B -> C, D -> A。
*   **ブランチ**: `feature/update-test`

## テスト手順

本テストは、`manual_tests/manual_test_gh_pr_update.py` スクリプトによって半自動化されています。

### 1. 前準備（PR作成フロー）

1.  4つのリポジトリを作成・初期化します。
2.  初期依存関係で `pr create` を実行し、Pull Request を作成します。

### 2. 依存関係の変更

ローカルの `.mstl/dependency-graph.md` (または指定ファイル) を編集し、以下の行を追加します。

```mermaid
    Repo_D --> Repo_A
```

### 3. `pr update` の実行

以下のコマンドを実行します。

```bash
mstl-gh pr update \
  --dependencies dependency-graph.md \
  --verbose
```

### 4. 検証（手動確認）

リポジトリ D の Pull Request ページを確認し、以下の点を確認します。

1.  **依存関係の更新**: 本文の「Dependencies」セクションに、リポジトリ A へのリンクが追加されていること。
2.  **他のPR**: リポジトリ A の PR に「Dependents」として D が追加されていること（双方向リンクの整合性確認）。

## クリーンアップ

テスト終了後、GitHub リポジトリおよびローカルの一時ディレクトリを削除します。
