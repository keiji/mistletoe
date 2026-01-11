#!/bin/bash

# すべての手動テストケースを実行するスクリプト
# 各テストの結果は result_full.txt に追記される

# ファイルパスは絶対パスで指定
OUTPUT_FILE="$(pwd)/result_full.txt"
SCRIPT_DIR=$(dirname "$0")

# 結果ファイルを初期化
echo "========================================================" > "$OUTPUT_FILE"
echo "Full Manual Test Started at $(date)" >> "$OUTPUT_FILE"
echo "========================================================" >> "$OUTPUT_FILE"

echo "Building binaries..."
"$SCRIPT_DIR/build_all.sh"

echo "Running manual_test_mstl.py..."
python3 "$SCRIPT_DIR/manual_test_mstl.py" --output "$OUTPUT_FILE"

echo "Running manual_test_init_dest.py..."
python3 "$SCRIPT_DIR/manual_test_init_dest.py" --output "$OUTPUT_FILE"

echo "Running manual_test_sync_conflict.py..."
python3 "$SCRIPT_DIR/manual_test_sync_conflict.py" --output "$OUTPUT_FILE"

echo "Running manual_test_gh_pr_create.py..."
python3 "$SCRIPT_DIR/manual_test_gh_pr_create.py" --output "$OUTPUT_FILE"

echo "Running manual_test_gh_pr_create_draft.py..."
python3 "$SCRIPT_DIR/manual_test_gh_pr_create_draft.py" --output "$OUTPUT_FILE"

echo "Running manual_test_gh_pr_create_safety.py..."
python3 "$SCRIPT_DIR/manual_test_gh_pr_create_safety.py" --output "$OUTPUT_FILE"

echo "Running manual_test_gh_pr_update.py..."
python3 "$SCRIPT_DIR/manual_test_gh_pr_update.py" --output "$OUTPUT_FILE"

echo "Running manual_test_gh_pr_checkout.py..."
python3 "$SCRIPT_DIR/manual_test_gh_pr_checkout.py" --output "$OUTPUT_FILE"

echo "Running temp_repos_cleanup.py..."
python3 "$SCRIPT_DIR/temp_repos_cleanup.py"

echo "Full Manual Test Completed."
