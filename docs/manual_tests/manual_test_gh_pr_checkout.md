# GitHub Pull Request チェックアウトテスト設計書

本ドキュメントは、`mstl-gh` の `pr checkout` コマンドを使用して、Pull Request の情報から開発環境を復元（初期化）する機能の手動テスト手順を定義します。

## 概要

このテストでは、以下のシナリオを検証します。

1.  4つのリポジトリ（A, B, C, D）で Pull Request を作成する（`pr create` 相当）。
2.  作成された Pull Request の URL を指定して `pr checkout` を実行する。
3.  指定したディレクトリにすべてのリポジトリがクローンされ、正しい状態（ブランチ・コミット）になっているか検証する。
4.  `--depth 1` オプションを指定した浅いクローン（Shallow Clone）の動作を検証する。

## テスト構成

*   **リポジトリ構成**: 4つのリポジトリ（A, B, C, D）。
*   **依存関係**: A -> B -> C, D (独立)。
*   **ブランチ**: `feature/checkout-test`

## テスト手順

本テストは、`manual_tests/manual_test_gh_pr_checkout.py` スクリプトによって半自動化されています。

### 1. 前準備（PR作成フロー）

1.  4つのリポジトリを作成・初期化します。
2.  各リポジトリに変更を加え、`pr create` を実行して Pull Request を作成します。
3.  リポジトリ A の Pull Request URL を取得します。

### 2. `pr checkout` の実行（通常）

以下のコマンドを実行します。

```bash
mstl-gh pr checkout \
  -u <Repo_A_PR_URL> \
  --dest ./pr_checkout \
  --verbose
```

### 3. `pr checkout` の実行（Shallow Clone）

以下のコマンドを実行します。

```bash
mstl-gh pr checkout \
  -u <Repo_A_PR_URL> \
  --dest ./pr_checkout_shallow \
  --depth 1 \
  --verbose
```

### 4. 検証（自動・手動）

1.  **ディレクトリ生成**: カレントディレクトリに `pr_checkout` および `pr_checkout_shallow` ディレクトリが作成されていること。
2.  **リポジトリ展開**: 各ディレクトリ内に、4つのリポジトリ（A, B, C, D）のディレクトリが存在すること。
3.  **状態の復元**: 各リポジトリが `feature/checkout-test` ブランチ（または作成時のコミット）にチェックアウトされていること。
4.  **依存関係**: `.mstl/dependency-graph.md` が復元されていること。
5.  **履歴の深さ（Shallow）**: `pr_checkout_shallow` 内のリポジトリにおいて、コミット履歴が1件のみであることを確認する（`git rev-list --count HEAD` が 1）。

## クリーンアップ

テスト終了後、GitHub リポジトリおよびローカルの一時ディレクトリ（`pr_checkout`, `pr_checkout_shallow` を含む）を削除します。
