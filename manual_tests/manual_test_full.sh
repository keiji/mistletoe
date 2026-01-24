#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

# すべての手動テストケースを実行するスクリプト
# 各テストの結果は result_full.txt に追記される

# ファイルパスは絶対パスで指定
OUTPUT_FILE="$(pwd)/result_full.txt"
SCRIPT_DIR=$(dirname "$0")

# 開始時間の記録
START_TIME_SECONDS=$(date +%s)
START_TIME_DISPLAY=$(date)

# 引数解析
YES_FLAG=""
if [[ "$1" == "--yes" ]]; then
    YES_FLAG="--yes"
    echo "Running in NON-INTERACTIVE mode (--yes)"
fi

# 結果ファイルを初期化
echo "========================================================" > "$OUTPUT_FILE"
echo "Full Manual Test Started at $START_TIME_DISPLAY" >> "$OUTPUT_FILE"
echo "========================================================" >> "$OUTPUT_FILE"

echo "Building binaries..."
"$SCRIPT_DIR/build_all.sh"

echo "Running manual_test_mstl.py..."
python3 "$SCRIPT_DIR/manual_test_mstl.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_config_search.py..."
python3 "$SCRIPT_DIR/manual_test_config_search.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_parent_config_switch.py..."
python3 "$SCRIPT_DIR/manual_test_parent_config_switch.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_init_dest.py..."
python3 "$SCRIPT_DIR/manual_test_init_dest.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_init_safety.py..."
python3 "$SCRIPT_DIR/manual_test_init_safety.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_init_dependencies.py..."
python3 "$SCRIPT_DIR/manual_test_init_dependencies.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_sync_conflict.py..."
python3 "$SCRIPT_DIR/manual_test_sync_conflict.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_switch_upstream.py..."
python3 "$SCRIPT_DIR/manual_test_switch_upstream.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_switch_remote.py..."
python3 "$SCRIPT_DIR/manual_test_switch_remote.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_switch_check.py..."
python3 "$SCRIPT_DIR/manual_test_switch_check.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_upstream_safety.py..."
python3 "$SCRIPT_DIR/manual_test_upstream_safety.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_pr_categorization.py..."
python3 "$SCRIPT_DIR/manual_test_pr_categorization.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_gh_pr_create.py..."
python3 "$SCRIPT_DIR/manual_test_gh_pr_create.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_gh_pr_create_draft.py..."
python3 "$SCRIPT_DIR/manual_test_gh_pr_create_draft.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_gh_pr_create_safety.py..."
python3 "$SCRIPT_DIR/manual_test_gh_pr_create_safety.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_gh_pr_update.py..."
python3 "$SCRIPT_DIR/manual_test_gh_pr_update.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_gh_pr_checkout.py..."
python3 "$SCRIPT_DIR/manual_test_gh_pr_checkout.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_pr_create_behind.py..."
python3 "$SCRIPT_DIR/manual_test_pr_create_behind.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running manual_test_pr_create_missing_base.py..."
python3 "$SCRIPT_DIR/manual_test_pr_create_missing_base.py" --output "$OUTPUT_FILE" $YES_FLAG

echo "Running reset_test.py..."
python3 "$SCRIPT_DIR/reset_test.py"

echo "Running temp_repos_cleanup.py..."
python3 "$SCRIPT_DIR/temp_repos_cleanup.py" $YES_FLAG

echo "Full Manual Test Completed."

# 終了時間の記録と計算
END_TIME_SECONDS=$(date +%s)
END_TIME_DISPLAY=$(date)
DURATION_SECONDS=$((END_TIME_SECONDS - START_TIME_SECONDS))

HOURS=$((DURATION_SECONDS / 3600))
MINUTES=$(((DURATION_SECONDS % 3600) / 60))
SECONDS=$((DURATION_SECONDS % 60))

SUMMARY_MSG="
========================================================
Full Manual Test Execution Summary
========================================================
Start Time:     $START_TIME_DISPLAY
End Time:       $END_TIME_DISPLAY
Total Duration: ${HOURS}h ${MINUTES}m ${SECONDS}s
========================================================"

echo "$SUMMARY_MSG"
echo "$SUMMARY_MSG" >> "$OUTPUT_FILE"
