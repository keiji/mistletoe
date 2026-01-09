# mstl-gh Manual Test Design

This document describes the design of the manual testing scripts for `mstl-gh`, specifically focusing on the interactive testing scripts.

## 1. Overview

The manual testing suite consists of interactive Python scripts that guide a human tester through various `mstl-gh` workflows (e.g., creating Pull Requests, updating them). Unlike fully automated tests, these scripts:
1.  Set up a temporary environment (GitHub repositories).
2.  Describe a test scenario and expected outcome to the user.
3.  Prompt the user to execute the command (or verify execution).
4.  Ask the user to verify the result manually (e.g., by checking a browser).
5.  Clean up the environment.

## 2. Scripts Structure

### 2.1. Shared Libraries
*   `scripts/gh_test_env.py`: Handles the setup and teardown of the test environment.
    *   Generates unique repository names (UUID-based).
    *   Creates private repositories using `gh repo create`.
    *   Builds the `mstl-gh` binary.
    *   Generates configuration (`mistletoe.json`) and dependency graphs.
    *   Cleans up (renames and deletes) repositories.
*   `scripts/interactive_runner.py`: Manages the user interaction flow.
    *   Parses arguments (e.g., `-o/--output`).
    *   Displays test scenarios and expected results.
    *   Prompts for confirmation (`yes/no`).
    *   Logs results (PASS/FAIL/SKIP) to stdout and optionally to a file.

### 2.2. Test Scripts
*   `scripts/manual_test_gh_pr_create.py`: Tests the "Create Pull Request" workflow.
    *   **Scenario:** Creates 3 repositories with a dependency chain (A -> B -> C).
    *   **Action:** Runs `mstl-gh pr create` interactively.
    *   **Verification:** User checks if PRs are created with correct dependency links.

### 2.3. Legacy Scripts
*   `scripts/manual_test_gh.py`: The original monolithic test script.
    *   **Safety:** Requires the user to type "I AGREE" to proceed.
    *   **Scope:** Runs a fixed sequence of all tests (`init`, `switch`, `status`, `pr create`, etc.).

## 3. Usage

### Running a Test
```bash
python3 scripts/manual_test_gh_pr_create.py [-o results.log]
```

### Flow
1.  **Setup:** The script creates temporary repositories (e.g., `mistletoe-test-1234-A`).
2.  **Prompt:** The script displays the expected result.
    ```
    [Expected Result]
    This test will create Pull Requests in...
    Do you want to execute the process? [Y/n]:
    ```
3.  **Execution:** The script runs `mstl-gh` commands. The user may need to interact with the CLI (e.g., typing "yes").
4.  **Verification:** The script asks if the result matches the expectation.
    ```
    Process complete. Is the behavior as expected? [Y/n]:
    ```
5.  **Logging:** The result is logged.
6.  **Cleanup:** The script asks to delete the temporary repositories.

## 4. Implementation Details

*   **Language:** Python 3.
*   **Dependencies:** `gh` CLI (authenticated), `git`, `go` (for building).
*   **Output:** All user-facing messages are in English.
*   **Safety:** Repositories are strictly namespaced with UUIDs. Cleanup handles renaming to `*-deleting` before deletion to prevent accidental data loss.

## 5. Future Extensions

To add a new test scenario:
1.  Create a new script (e.g., `scripts/manual_test_gh_pr_sync.py`).
2.  Import `GhTestEnv` and `InteractiveRunner`.
3.  Define the scenario logic function.
4.  Call `runner.execute_scenario()`.
