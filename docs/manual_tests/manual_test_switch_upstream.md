# 手動テスト: `switch`コマンドによるUpstream設定

## 概要
`switch`コマンドを使用してブランチを切り替えた際、リモートに同名のブランチが存在し、かつマージ可能である場合に、自動的にUpstream（追跡ブランチ）が設定されることを検証します。

## 前提条件
* `mstl` コマンドがビルドされ、パスが通っていること（またはテストスクリプトがパス解決すること）。
* Gitがインストールされていること。

## テストケース

### ケース1: リモートブランチが存在する場合のUpstream自動設定

#### 手順
1. テスト用のリモートリポジトリ（Bare）と、それをクローンしたローカルリポジトリを用意する。
2. リモートリポジトリに `feature/upstream-test` ブランチを作成する。
3. ローカルリポジトリには `feature/upstream-test` が存在しない状態にする。
4. `mstl switch -c feature/upstream-test` を実行する。
5. ローカルリポジトリのGit設定を確認し、Upstreamが正しく設定されているか確認する。

#### 期待される結果
* コマンドが成功し、ブランチが作成・切り替えられること。
* `git branch -vv` または `git config` の出力において、`feature/upstream-test` が `origin/feature/upstream-test` を追跡していること。
    * `branch.feature/upstream-test.remote` が `origin` であること。
    * `branch.feature/upstream-test.merge` が `refs/heads/feature/upstream-test` であること。

### ケース2: リモートブランチが存在しない場合

#### 手順
1. リモートリポジトリに存在しないブランチ名 `feature/no-remote` を指定して `mstl switch -c feature/no-remote` を実行する。

#### 期待される結果
* ブランチは作成されるが、Upstreamは設定されないこと。

## クリーンアップ
* テスト用に作成した一時ディレクトリおよびリポジトリを削除する。
