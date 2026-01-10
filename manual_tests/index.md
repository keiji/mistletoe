# Manual Test Implementation Policy

This document outlines the implementation guidelines for manual test scripts in the `manual_tests/` directory.

## 1. Output Coloring

*   **Rule:** All instructional texts, status messages, and logs output by the test script must be displayed in **Green**.
*   **Implementation:** Use the `print_green` function from `interactive_runner.py` or equivalent logic.
*   **Reason:** To distinguish script output from the output of the tools being tested (e.g., `git`, `mstl`).

## 2. GitHub Repository Creation

*   **Rule:** When a test script requires creating repositories on GitHub (real, not mocked):
    1.  The script must first generate or determine the list of repositories/directories to be created.
    2.  Display this list to the user.
    3.  Explicitly ask for user confirmation (e.g., "Do you want to create these repositories and run the test? [Y/n]") before proceeding with creation.
*   **Implementation:** See `manual_test_gh_pr_create.py` as a reference implementation.

## 3. Consistency

*   Use `interactive_runner.py` where applicable to standardize the "Setup -> Execute -> Verify -> Cleanup" workflow.
*   Ensure temporary directories and repositories are cleaned up (or the user is prompted to clean them up) after execution.
