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

### Core Functionality (基本機能)

| ID | テストケース名 | 説明 | 設計書 | スクリプト |
| :--- | :--- | :--- | :--- | :--- |
| `mstl_basic` | mstl 基本機能テスト | `mstl` の基本機能（init, status, switch, push, sync, snapshot）を検証します。 | [`docs/manual_tests/manual_test_design.md`](./manual_test_design.md) | `manual_tests/manual_test_mstl.py` |
| `init_dest` | 初期化先ディレクトリ検証 | `init` コマンドの出力先ディレクトリに関する様々な条件（既存、空ではない、など）を検証します。 | [`docs/manual_tests/manual_test_init_dest.md`](./manual_test_init_dest.md) | `manual_tests/manual_test_init_dest.py` |
| `init_dependencies` | 初期化時依存関係検証 | `init` コマンドの `--dependencies` フラグの動作（正常コピー、不正ID検知、ファイル欠損）を検証します。 | - | `manual_tests/manual_test_init_dependencies.py` |
| `config_search` | 親ディレクトリ設定探索 | カレントディレクトリに設定がなく、親ディレクトリにある場合の探索およびバリデーション動作を検証します。 | - | `manual_tests/manual_test_config_search.py` |
| `parent_config_switch` | 親ディレクトリ設定スイッチ | 親ディレクトリの設定が見つかった際、ワーキングコンテキストが親ディレクトリに正しく切り替わることを検証します。 | - | `manual_tests/manual_test_parent_config_switch.py` |

### GitHub Integration (PRs) (GitHub連携)

| ID | テストケース名 | 説明 | 設計書 | スクリプト |
| :--- | :--- | :--- | :--- | :--- |
| `gh_pr_create` | GitHub PR 作成フロー | `mstl-gh pr create` コマンドによる依存関係を含む Pull Request の作成フローを検証します。 | [`docs/manual_tests/manual_test_gh_pr_create.md`](./manual_test_gh_pr_create.md) | `manual_tests/manual_test_gh_pr_create.py` |
| `gh_pr_create_draft` | GitHub PR 作成 (Draft) | `mstl-gh pr create` コマンドで `--draft` フラグを使用し、Draft Pull Request が作成されることを検証します。 | [`docs/manual_tests/manual_test_gh_pr_create_draft.md`](./manual_test_gh_pr_create_draft.md) | `manual_tests/manual_test_gh_pr_create_draft.py` |
| `gh_pr_update` | PR 更新フロー検証 | `mstl-gh pr update` コマンドによる依存関係情報の更新を検証します。 | [`docs/manual_tests/manual_test_gh_pr_update.md`](./manual_test_gh_pr_update.md) | `manual_tests/manual_test_gh_pr_update.py` |
| `gh_pr_checkout` | PR チェックアウト検証 | `mstl-gh pr checkout` コマンドによる環境復元フローを検証します。 | [`docs/manual_tests/manual_test_gh_pr_checkout.md`](./manual_test_gh_pr_checkout.md) | `manual_tests/manual_test_gh_pr_checkout.py` |
| `pr_categorization` | PR カテゴリ分けロジック | `pr create` 実行時にリポジトリの状態（Push要/不要、PR作成要/不要）が正しく分類されるかを検証します。 | - | `manual_tests/manual_test_pr_categorization.py` |

### Safety & Edge Cases (安全性・エッジケース)

| ID | テストケース名 | 説明 | 設計書 | スクリプト |
| :--- | :--- | :--- | :--- | :--- |
| `gh_pr_create_safety` | PR作成競合安全性テスト | `pr create` 実行中の並列操作による競合状態の検知を検証します。 | [`docs/manual_tests/manual_test_gh_pr_create_safety.md`](./manual_test_gh_pr_create_safety.md) | `manual_tests/manual_test_gh_pr_create_safety.py` |
| `init_safety` | 初期化時安全確認 | リストにないファイルが存在するディレクトリでの `init` 実行時に、警告プロンプトが表示されるかを検証します。 | - | `manual_tests/manual_test_init_safety.py` |
| `mstl_sync_conflict` | mstl sync 競合テスト | `mstl sync` コマンド実行時にマージ競合が発生した場合の挙動を検証します。 | [`docs/manual_tests/manual_test_sync_conflict.md`](./manual_test_sync_conflict.md) | `manual_tests/manual_test_sync_conflict.py` |
| `switch_upstream` | Switch Upstream 設定検証 | `switch` コマンドによる自動的な Upstream 設定の挙動を検証します。 | [`docs/manual_tests/manual_test_switch_upstream.md`](./manual_test_switch_upstream.md) | `manual_tests/manual_test_switch_upstream.py` |
| `switch_remote` | Switch Remote Fallback | ローカルにブランチが存在せずリモートにのみ存在する場合の、自動 Fetch および Checkout 動作を検証します。 | - | `manual_tests/manual_test_switch_remote.py` |
| `upstream_safety` | Upstream 設定安全性 | ブランチ名不一致やリモートブランチ消失時に、`status` コマンドが Upstream 設定を解除する動作を検証します。 | - | `manual_tests/manual_test_upstream_safety.py` |
| `pr_create_behind` | PR作成時 Behind 検知 | ローカルブランチがリモートより遅れている、または分岐している場合に、`pr create` がエラーとなることを検証します。 | - | `manual_tests/manual_test_pr_create_behind.py` |
| `pr_create_missing_base` | PR作成時 Base Branch 欠損 | リモートに Base Branch が存在しないリポジトリが、`pr create` の対象からスキップされることを検証します。 | - | `manual_tests/manual_test_pr_create_missing_base.py` |
