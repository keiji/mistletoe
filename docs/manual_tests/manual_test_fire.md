# Manual Test: Fire Command

## 概要 (Overview)
`mstl fire` コマンドが、ユーザー対話なしで全リポジトリの変更を新規ブランチにコミット・プッシュすることを確認します。

## テストシナリオ (Test Scenario)

1.  **環境セットアップ**:
    *   1つのリポジトリ（`repo1`）を持つ `config.json` を用意。
    *   `repo1` に未コミットの変更（新規ファイル作成など）を加える。

2.  **実行**:
    *   `mstl fire` を実行。

3.  **検証**:
    *   コマンドが成功し、以下の出力が含まれること:
        *   `FIRE command initiated`
        *   `Secured in mstl-fire-repo1-...`
        *   `FIRE command completed`
    *   `repo1` の現在のブランチが `mstl-fire-repo1-...` で始まっていること。
    *   未コミットの変更がコミットされていること（`git status` が clean）。
    *   コミットメッセージに "Emergency commit triggered by mstl" が含まれていること。
    *   (可能であれば) リモートにPushされたことを確認（ローカルテスト環境では `origin` がローカルパスであれば確認可能）。

## 実行方法 (Execution)

```bash
python3 manual_tests/manual_test_fire.py
```
