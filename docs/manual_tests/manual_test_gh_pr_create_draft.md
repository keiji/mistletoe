# GitHub Pull Request 作成テスト設計書 (Draft)

本ドキュメントは、`mstl-gh` の `pr create` コマンドを使用して、ドラフト (Draft) 状態の Pull Request を作成する機能の手動テスト手順を定義します。

## 概要

このテストでは、以下のシナリオを検証します。

1.  複数の**パブリック**リポジトリの準備と初期化。
2.  各リポジトリへの変更のコミット。
3.  `--draft` フラグを指定した `pr create` の実行。
4.  GitHub 上での Draft Pull Request の作成確認。

## 前提条件

*   GitHub CLI (`gh`) がインストールされ、ログイン済みであること。
*   Git がインストールされていること。
*   テスト用の Python スクリプト実行環境が整っていること。
*   インターネットに接続されていること（GitHub API を使用するため）。

## テスト構成

*   **リポジトリ構成**: 4つのリポジトリ（A, B, C, D）を使用します。すべてのリポジトリは**Public**として作成されます。
*   **依存関係**:
    *   A -> B -> C (A は B に依存し、B は C に依存する)。
    *   D は他のリポジトリと依存関係を持たない（独立）。
*   **ブランチ**: `feature/interactive-test-draft`

## テスト手順

本テストは、`manual_tests/manual_test_gh_pr_create_draft.py` スクリプトによって半自動化されています。

### 1. 環境セットアップ（自動）

スクリプトを実行すると、以下の処理が自動的に行われます。

1.  `mstl-gh` バイナリのビルド。
2.  一時的なリポジトリ名（`mistletoe-test-*`）の生成。

### 2. リポジトリ作成と初期化（自動）

1.  GitHub 上にテスト用リポジトリが4つ、**Public**設定で作成されます。
2.  `mstl-gh init` が実行され、ローカルにリポジトリがクローンされます。
3.  各リポジトリにダミーの Git ユーザー設定が行われます。

### 3. 変更の作成（自動）

1.  `mstl-gh switch -c feature/interactive-test-draft` が実行され、全リポジトリでブランチが切り替わります。
2.  各リポジトリに `test.txt` が追加され、コミットされます。

### 4. `pr create` の実行（対話的）

スクリプトは以下のコマンドを実行します。

```bash
mstl-gh pr create \
  -t "Interactive Test Draft PR" \
  -b "Testing interactive script with draft" \
  --dependencies dependencies.md \
  --draft \
  --verbose
```

**操作:**
*   コマンド実行中、ツールがユーザーに入力を求めます。
*   Pull Request を作成するか問われた際、`yes` と入力します。

### 5. 検証（手動確認）

スクリプトの実行完了後、表示された URL にアクセスし、以下の点を確認します。

1.  **Draft PRの作成**: リポジトリ A, B, C, D すべてに対して Pull Request が作成され、ステータスが **Draft** になっていること。
2.  **依存関係の表示**:
    *   通常の `pr create` と同様に、Mistletoe ブロック（依存関係情報）が正しく埋め込まれていること。

## クリーンアップ

テスト終了後、スクリプトは作成した GitHub リポジトリとローカルの一時ディレクトリを削除します。
