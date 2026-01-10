# 手動テスト

mistletoe の機能を手動で検証するためのテスト設計書をまとめます。

## 運用フロー

手動テストの追加・変更は以下のフローで行います。

1.  **一覧の更新**: 本ドキュメント (`docs/manual_tests/index.md`) の「テストケース」セクションに、新しいテストケースを追記、または既存の項目を修正します。
2.  **設計書の作成**: テストケースに対応する個別の設計書を `docs/manual_tests/` 配下に作成（または修正）します。設計書は日本語で記述し、マークダウン形式とします。
    *   例: `docs/manual_tests/manual_test_gh_pr_create.md`
3.  **スクリプトの作成**: 設計書の内容に基づき、`manual_tests/` 配下に実行可能なテストスクリプトを作成（または修正）します。
    *   例: `manual_tests/manual_test_gh_pr_create.py`

## テストケース

現在定義されている手動テストケースは以下の通りです。

| ID | テストケース名 | 説明 | 設計書 | スクリプト |
| :--- | :--- | :--- | :--- | :--- |
| `mstl_basic` | mstl 基本機能テスト | `mstl` の基本機能（init, status, switch, push, sync, snapshot）を検証します。 | [`docs/manual_tests/manual_test_design.md`](./manual_test_design.md) | `manual_tests/manual_test_mstl.py` |
| `gh_pr_create` | GitHub PR 作成フロー | `mstl-gh pr create` コマンドによる依存関係を含む Pull Request の作成フローを検証します。リポジトリD（孤立）を含む4つのリポジトリ構成で実施します。 | [`docs/manual_tests/manual_test_gh_pr_create.md`](./manual_test_gh_pr_create.md) | `manual_tests/manual_test_gh_pr_create.py` |
| `gh_pr_create_safety` | PR作成競合安全性テスト | `pr create` 実行中の並列操作による競合状態の検知を検証します。 | [`docs/manual_tests/manual_test_gh_pr_create_safety.md`](./manual_test_gh_pr_create_safety.md) | `manual_tests/manual_test_gh_pr_create_safety.py` |
| `gh_pr_checkout` | PR チェックアウト検証 | `mstl-gh pr checkout` コマンドによる環境復元フローを検証します。 | [`docs/manual_tests/manual_test_gh_pr_checkout.md`](./manual_test_gh_pr_checkout.md) | `manual_tests/manual_test_gh_pr_checkout.py` |
| `gh_pr_update` | PR 更新フロー検証 | `mstl-gh pr update` コマンドによる依存関係情報の更新を検証します。 | [`docs/manual_tests/manual_test_gh_pr_update.md`](./manual_test_gh_pr_update.md) | `manual_tests/manual_test_gh_pr_update.py` |
| `init_dest` | 初期化先ディレクトリ検証 | `init` コマンドの出力先ディレクトリに関する様々な条件（既存、空ではない、など）を検証します。 | [`docs/manual_tests/manual_test_init_dest.md`](./manual_test_init_dest.md) | `manual_tests/manual_test_init_dest.py` |
