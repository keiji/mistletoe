# Manual Tests

This directory contains interactive manual tests and scripts to verify the functionality of `mstl` and `mstl-gh`.

These tests are designed to be run in an isolated environment (like Docker) to avoid modifying your local configuration or creating persistent repositories on your GitHub account (although the tests do create and delete temporary repositories).

## Prerequisites

*   **Docker**: Recommended for running tests in a clean environment.
*   **GitHub Personal Access Token (PAT)**: Required for `mstl-gh` tests to authenticate with GitHub. The token needs `repo` and `delete_repo` scopes.

## Running Tests with Docker (Recommended)

The easiest way to run these tests is using the provided Dockerfile. This ensures all dependencies (Go, Python, git, gh CLI) are installed and configured correctly.

### 1. Build the Docker Image

You can build the image in two ways:

#### Option A: Build with Pre-Authentication (Recommended)

Pass your GitHub token as a build argument. This creates an image where `gh` is already logged in, so you don't need to authenticate every time you run the container.

```bash
# Replace <YOUR_GITHUB_TOKEN> with your actual token
docker build --build-arg GITHUB_TOKEN=<YOUR_GITHUB_TOKEN> -t mstl-gh-test -f manual_tests/Dockerfile.manual_test .
```

> **Note:** Be careful with your shell history when passing tokens on the command line.

#### Option B: Standard Build

Build the image without a token. You will need to log in manually inside the container.

```bash
docker build -t mstl-gh-test -f manual_tests/Dockerfile.manual_test .
```

### 2. Run the Container

Start the container interactively, mounting the current directory so you can edit scripts if needed.

```bash
docker run -it --rm -v $(pwd):/app mstl-gh-test /bin/bash
```

### 3. Run Test Scripts

Inside the container, navigate to the `manual_tests` directory (if not already there) and run the desired test script using Python 3.

If you didn't use Option A (Pre-Authentication), run `gh auth login` first.

**Example: Test Pull Request Creation (`mstl-gh`)**

```bash
python3 manual_tests/manual_test_gh_pr_create.py
```

**Example: Test Core Functionality (`mstl`)**

```bash
python3 manual_tests/manual_test_mstl.py
```

Follow the on-screen prompts. These scripts act as interactive guides, performing setup steps automatically and pausing to let you verify the results (e.g., checking URLs in your browser).

## Available Tests

*   **`manual_test_gh_pr_create.py`**: Verifies the `mstl-gh pr create` workflow, including dependency graph parsing, snapshot generation, and bulk PR creation.
*   **`manual_test_mstl.py`**: Verifies core `mstl` commands (`init`, `status`, `switch`, `push`, `sync`) against local bare repositories.
*   **`manual_test_gh_safety.py`**: Verifies safety checks (race conditions) for `pr create`.
*   **`manual_test_init_dest.py`**: Verifies the `init` command's destination logic.
*   **`manual_test_sync_conflict.py`**: Verifies `sync` behavior when merge conflicts occur.

## Cleanup

The test scripts attempt to clean up the temporary repositories they create (prefixed with `mistletoe-test-`) upon completion or failure.

If a test is interrupted and artifacts remain, you can run the cleanup script (requires `gh` authentication):

```bash
python3 manual_tests/temp_repos_cleanup.py
```
