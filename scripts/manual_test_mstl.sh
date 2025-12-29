#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
log() {
    echo -e "${GREEN}[TEST]${NC} $1"
}

fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    exit 1
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

check_result() {
    if [ $? -eq 0 ]; then
        log "Success: $1"
    else
        fail "Failed: $1"
    fi
}

# 1. Setup Environment
log "Setting up environment..."

ROOT_DIR=$(pwd)
TEST_DIR=$(mktemp -d)
BIN_PATH="$TEST_DIR/bin/mstl"
REPOS_DIR="$TEST_DIR/repos"
REMOTE_DIR="$TEST_DIR/remotes"
CONFIG_FILE="$TEST_DIR/mstl_config.json"

log "Test Directory: $TEST_DIR"

# Cleanup trap
cleanup() {
    if [ -d "$TEST_DIR" ]; then
        log "Cleaning up temporary directory..."
        rm -rf "$TEST_DIR"
    fi
}
trap cleanup EXIT

# Configure git for tests (local environment)
export GIT_AUTHOR_NAME="Test User"
export GIT_AUTHOR_EMAIL="test@example.com"
export GIT_COMMITTER_NAME="Test User"
export GIT_COMMITTER_EMAIL="test@example.com"

# Build mstl
log "Building mstl..."
mkdir -p "$TEST_DIR/bin"
go build -o "$BIN_PATH" ./cmd/mstl
check_result "Build mstl"

# Verify version
log "Verifying version..."
"$BIN_PATH" version | grep "mstl version" > /dev/null
check_result "mstl version command"

# Verify help
log "Verifying help..."
"$BIN_PATH" help | grep "Usage:" > /dev/null
check_result "mstl help command"

# Setup Remotes
log "Setting up remote repositories..."
mkdir -p "$REMOTE_DIR"
git init --bare "$REMOTE_DIR/repo1.git" > /dev/null
git init --bare "$REMOTE_DIR/repo2.git" > /dev/null

# Create initial content in remotes (by cloning, committing, pushing)
log "Seeding remotes..."
TMP_SEED="$TEST_DIR/seed"
mkdir -p "$TMP_SEED"
git clone "$REMOTE_DIR/repo1.git" "$TMP_SEED/repo1" > /dev/null
cd "$TMP_SEED/repo1"
git checkout -b main
echo "# Repo 1" > README.md
git add README.md
git commit -m "Initial commit repo1" > /dev/null
git push origin main > /dev/null
# Set default branch in bare repo to avoid "remote HEAD refers to nonexistent ref"
git --git-dir="$REMOTE_DIR/repo1.git" symbolic-ref HEAD refs/heads/main
cd "$ROOT_DIR"

git clone "$REMOTE_DIR/repo2.git" "$TMP_SEED/repo2" > /dev/null
cd "$TMP_SEED/repo2"
git checkout -b main
echo "# Repo 2" > README.md
git add README.md
git commit -m "Initial commit repo2" > /dev/null
git push origin main > /dev/null
# Set default branch in bare repo to avoid "remote HEAD refers to nonexistent ref"
git --git-dir="$REMOTE_DIR/repo2.git" symbolic-ref HEAD refs/heads/main
cd "$ROOT_DIR"

# Create Configuration File
log "Creating mstl configuration..."
cat <<EOF > "$CONFIG_FILE"
{
  "repositories": [
    {
      "url": "$REMOTE_DIR/repo1.git"
    },
    {
      "url": "$REMOTE_DIR/repo2.git"
    }
  ]
}
EOF

# 2. Test Init
log "Testing 'init'..."
mkdir -p "$REPOS_DIR"
cd "$REPOS_DIR"
"$BIN_PATH" init -f "$CONFIG_FILE"
check_result "mstl init"

# Verify directories exist
[ -d "repo1" ] && [ -d "repo2" ]
check_result "Repositories cloned"

# 3. Test Status (Clean)
log "Testing 'status' (Clean)..."
"$BIN_PATH" status -f "$CONFIG_FILE"
check_result "mstl status"
# Ideally we'd parse the output, but exit code 0 is a good start.
# Let's ensure no "!" or symbols are in the output for a clean state
OUTPUT=$("$BIN_PATH" status -f "$CONFIG_FILE")
# Filter out the legend before checking for conflict symbols
if echo "$OUTPUT" | grep -v "Status Legend" | grep -q "!"; then
    fail "Status showed conflict on clean repo"
fi

# 4. Test Switch
log "Testing 'switch'..."
"$BIN_PATH" switch -f "$CONFIG_FILE" -c feature/test-branch
check_result "mstl switch -c"

cd repo1
CURRENT_BRANCH=$(git symbolic-ref --short HEAD)
if [ "$CURRENT_BRANCH" != "feature/test-branch" ]; then
    fail "repo1 not on feature/test-branch (was $CURRENT_BRANCH)"
fi
cd ..

# 5. Test Push
log "Testing 'push'..."
cd repo1
echo "Change in repo1" >> README.md
git add README.md
git commit -m "Update repo1" > /dev/null
cd ..

# Verify status shows unpushed
OUTPUT=$("$BIN_PATH" status -f "$CONFIG_FILE")
if ! echo "$OUTPUT" | grep -q ">"; then
    fail "Status did not show unpushed commit (>)"
fi

# Push (requires yes)
log "Running push (piping 'yes')..."
echo "yes" | "$BIN_PATH" push -f "$CONFIG_FILE"
check_result "mstl push"

# Verify remote has commit
cd "$TMP_SEED/repo1"
git fetch origin > /dev/null
if ! git log origin/feature/test-branch --oneline | grep -q "Update repo1"; then
    fail "Remote repo1 does not have the pushed commit"
fi
cd "$REPOS_DIR"

# 6. Test Sync
log "Testing 'sync'..."
# We just pushed feature/test-branch for both repos.
# Let's switch back to main for a clean sync test on existing branch.

log "Switching back to main for sync test..."
# Note: mstl switch requires directories to exist. We are in REPOS_DIR.
"$BIN_PATH" switch -f "$CONFIG_FILE" main
check_result "Switch back to main"

# Update remote repo2 (simulating another user pushing to main)
cd "$TMP_SEED/repo2"
git checkout main > /dev/null
echo "Remote Change repo2" >> README.md
git add README.md
git commit -m "Remote update repo2" > /dev/null
git push origin main > /dev/null
cd "$REPOS_DIR"

# Verify status shows pullable
# Need to fetch first? mstl status/sync usually fetches.
OUTPUT=$("$BIN_PATH" status -f "$CONFIG_FILE")
if ! echo "$OUTPUT" | grep -q "<"; then
    fail "Status did not show pullable commit (<)"
fi

# Run sync
log "Running sync..."
"$BIN_PATH" sync -f "$CONFIG_FILE"
check_result "mstl sync"

# Verify local repo2 has the update
cd repo2
if ! grep -q "Remote Change repo2" README.md; then
    fail "repo2 did not receive remote changes"
fi
cd ..

# 7. Test Snapshot
log "Testing 'snapshot'..."
# Run snapshot in the repos directory
"$BIN_PATH" snapshot
check_result "mstl snapshot"

if ! ls mistletoe-snapshot-*.json 1> /dev/null 2>&1; then
    fail "Snapshot file not created"
fi

log "Checking snapshot content..."
SNAPSHOT_FILE=$(ls mistletoe-snapshot-*.json | head -n 1)
if ! grep -q "repo1" "$SNAPSHOT_FILE"; then
    fail "Snapshot missing repo1"
fi
if ! grep -q "main" "$SNAPSHOT_FILE"; then
    fail "Snapshot missing main branch info"
fi

log "${GREEN}All tests passed!${NC}"
