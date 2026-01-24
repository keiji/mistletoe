# Manual Test: Switch Consistency Check

## 概要 (Overview)
`switch -c` (作成モード) 実行時に、各リポジトリの現在のブランチ名が一致していない場合、警告と現状のステータスを表示し、ユーザーに続行の確認を求める機能を確認します。

## テストシナリオ (Test Scenario)

1.  **環境セットアップ**
    *   2つのローカルリポジトリ (`repo1`, `repo2`) を作成する。
    *   `repo1` は `main` ブランチ、`repo2` は `dev` ブランチにチェックアウトしておく。
    *   `mstl.json` を設定する。

2.  **不整合時の警告確認 (Abort)**
    *   コマンド: `mstl switch -c new-feature` を実行する。
    *   期待される動作:
        *   "Branch names do not match" という警告が表示される。
        *   `[repo1] main` および `[repo2] dev` という現在のブランチ名が表示される。
        *   "Do you want to continue? (yes/no):" というプロンプトが表示される。
    *   アクション: `no` (または `n`) を入力する。
    *   期待される結果:
        *   処理が中止され、エラー終了する。
        *   ブランチは変更されない。

3.  **不整合時の続行確認 (Proceed)**
    *   コマンド: `mstl switch -c new-feature` を再度実行する。
    *   アクション: `yes` (または `y`) を入力する。
    *   期待される結果:
        *   処理が継続される。
        *   両方のリポジトリが `new-feature` ブランチに切り替わる（存在しなければ作成される）。

## 実行方法 (How to Run)
```bash
python3 manual_tests/manual_test_switch_check.py
```
