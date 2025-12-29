# mstl Manual Test Design

This document outlines the design for a shell script to verify the core functionality of the `mstl` command-line tool. The test script is intended to be run in a Unix-like environment (Linux/macOS) with `git` installed.

## 1. Overview

The test script will:
1.  Build the `mstl` binary from source.
2.  Create a temporary testing directory.
3.  Set up "remote" bare Git repositories to simulate a server environment.
4.  Generate a `mstl` configuration file pointing to these remotes.
5.  Execute `mstl` subcommands in a logical sequence.
6.  Verify the outcome of each command (exit codes, file existence, git state).

## 2. Prerequisites

*   Go (to build `mstl`)
*   Git
*   Bash

## 3. Test Scenarios

### 3.1. Basic Information
*   **Command:** `mstl version`
*   **Expected Result:** output contains "mstl version".
*   **Command:** `mstl help`
*   **Expected Result:** output contains usage information and list of commands.

### 3.2. Initialization (`init`)
*   **Setup:** Create a config file `mstl_config.json` defining two repositories (`repo1`, `repo2`).
*   **Command:** `mstl init -f mstl_config.json`
*   **Expected Result:**
    *   Directories `repo1` and `repo2` are created.
    *   Both are valid git repositories.
    *   Both are checked out to the default branch (e.g., `main`).

### 3.3. Status (`status`) - Clean
*   **Command:** `mstl status -f mstl_config.json`
*   **Expected Result:**
    *   Output table shows both repositories.
    *   Status column implies "clean" (no symbols like `!`, `<`, `>`).

### 3.4. Branch Switching (`switch`)
*   **Command:** `mstl switch -f mstl_config.json -c feature/test-branch`
*   **Expected Result:**
    *   Both local repositories are switched to `feature/test-branch`.
    *   `git symbolic-ref HEAD` in each repo returns `refs/heads/feature/test-branch`.

### 3.5. Modifications & Push (`push`)
*   **Setup:** Make a commit in `repo1`.
*   **Command:** `mstl status -f mstl_config.json`
*   **Expected Result:** `repo1` shows unpushed status (`>`).
*   **Command:** `mstl push -f mstl_config.json` (may require `yes` input or flags if interactive).
    *   *Note:* Since `push` might prompt, the script needs to handle this (e.g., `echo "yes" | ...`).
*   **Expected Result:**
    *   Push succeeds.
    *   `mstl status` shows clean again.
    *   Remote `repo1` has the new commit.

### 3.6. Remote Updates & Sync (`sync`)
*   **Setup:** Create a commit in `repo2`'s remote (simulate another user).
*   **Command:** `mstl status -f mstl_config.json`
*   **Expected Result:** `repo2` shows pullable status (`<`).
*   **Command:** `mstl sync -f mstl_config.json` (may require interactive handling for rebase/merge strategy, defaulting to pull).
*   **Expected Result:**
    *   Sync succeeds.
    *   Local `repo2` has the remote commit.
    *   `mstl status` shows clean.

### 3.7. Snapshot (`snapshot`)
*   **Command:** `mstl snapshot` (run inside the parent directory of repos).
*   **Expected Result:**
    *   A JSON file (e.g., `mistletoe-snapshot-*.json`) is created.
    *   Content includes `repo1` and `repo2` with their current branch/revision.

## 4. Implementation Details

*   The script should use `set -e` to fail fast on errors.
*   Use a `trap` to clean up the temporary directory on exit.
*   Functions should be used to encapsulate common verification steps (e.g., `assert_git_branch`).
*   Output should be colored (Green for pass, Red for fail) for readability.
