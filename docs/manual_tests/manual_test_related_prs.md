# 関連Pull Request表示テスト（Merged/Closed含む）

本ドキュメントは、`mstl-gh` の `pr create` コマンドにおいて、同じリモートブランチ・Baseブランチを対象とするPull Requestが複数存在する場合（Merged/Closed含む）、それらが正しく「Related Pull Request(s)」に含まれるかを確認する手動テスト手順を定義します。

## 概要

このテストでは、以下のシナリオを検証します。

1.  PR A を作成し、マージする。
2.  同じブランチに新しいコミットを追加し、PR B を作成する。
3.  PR B（および他リポジトリのPR）の本文に、PR A（Merged）と PR B（Open）の両方が表示されることを確認する。

## テスト手順

### 1. 準備

任意のテスト用リポジトリ（または既存リポジトリ）を使用し、`mstl-gh` プロジェクトをセットアップします。

```bash
# setup
mstl-gh init ...
mstl-gh switch -c feature/related-pr-test
```

### 2. PR A の作成とマージ

1.  変更をコミットします。
    ```bash
    echo "Change A" > file_a.txt
    git add file_a.txt
    git commit -m "Add file A"
    ```
2.  PRを作成します。
    ```bash
    mstl-gh pr create -t "PR A" -b "First PR"
    ```
3.  作成された PR A を GitHub 上でマージします（"Squash and merge" または "Create a merge commit"）。
    *   **注意**: ブランチは削除しないでください（または、削除しても同じ名前でPushされることを前提とします）。

### 3. PR B の作成

1.  ローカルで新しい変更をコミットします。
    ```bash
    echo "Change B" > file_b.txt
    git add file_b.txt
    git commit -m "Add file B"
    ```
2.  PRを作成します。
    ```bash
    mstl-gh pr create -t "PR B" -b "Second PR"
    ```

### 4. 検証

作成された PR B の本文（Description）を確認します。

*   **期待値**:
    *   `## Mistletoe` ブロック内の `### Related Pull Request(s)` セクションに、**PR A (Merged)** と **PR B (Open)** の両方の URL がリストされていること。
    *   (複数リポジトリで実施した場合) 他のリポジトリの同時作成された PR の本文にも、このリポジトリの PR A と PR B が両方リストされていること。

## 補足

*   Closed（マージせずにクローズ）された PR についても同様の手順で、リストに含まれることを確認できます。
