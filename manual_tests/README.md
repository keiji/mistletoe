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

Pass your GitHub token as a secret. This creates an image where `gh` is already logged in, so you don't need to authenticate every time you run the container.

**Setting the Secret:**

To pass the secret, you generally use the `--secret` flag with `docker build`. You can source the secret from an environment variable or a file.

**Method 1: Using an Environment Variable (Simplest)**

Ensure you have your token in an environment variable (e.g., `GITHUB_TOKEN`).

```bash
export GITHUB_TOKEN="ghp_your_token_here"
# The 'env=GITHUB_TOKEN' part tells Docker to read the value from your shell's environment variable
docker build --secret id=mistletoe_manual_test_github_token,env=GITHUB_TOKEN -t mstl-gh-test -f manual_tests/Dockerfile.manual_test .
```

**Method 2: Using a File**

If you have your token in a file (e.g., `my_token.txt`):

```bash
docker build --secret id=mistletoe_manual_test_github_token,src=my_token.txt -t mstl-gh-test -f manual_tests/Dockerfile.manual_test .
```

> **Note:** The `id` must be exactly `mistletoe_manual_test_github_token` as expected by the Dockerfile.

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

### Core Functionality
*   **`manual_test_mstl.py`**: Verifies core `mstl` commands (`init`, `status`, `switch`, `push`, `sync`) against local bare repositories.
*   **`manual_test_init_dest.py`**: Verifies `init` command behavior for destination folders (existing, missing, non-empty, etc.).
*   **`manual_test_init_dependencies.py`**: Verifies `init` with `--dependencies` flag (valid graph, invalid ID, missing file).
*   **`manual_test_config_search.py`**: Verifies parent configuration search logic and validation handling.
*   **`manual_test_parent_config_switch.py`**: Verifies that the tool correctly switches its working context to the parent directory when a parent configuration is found.

### GitHub Integration (PRs)
*   **`manual_test_gh_pr_create.py`**: Verifies the standard `pr create` workflow, including dependency graph parsing, snapshot generation, and bulk PR creation.
*   **`manual_test_gh_pr_create_draft.py`**: Verifies creation of Draft PRs using the `--draft` flag.
*   **`manual_test_gh_pr_update.py`**: Verifies `pr update` workflow, ensuring description bodies (dependencies) are updated without creating new PRs.
*   **`manual_test_gh_pr_checkout.py`**: Verifies `pr checkout` workflow, restoring a development environment from PR URLs.
*   **`manual_test_pr_categorization.py`**: Verifies the categorization logic for `pr create` (Push & Create, Create Only, Push & Update, etc.).

### Safety & Edge Cases
*   **`manual_test_gh_pr_create_safety.py`**: Verifies race condition checks during `pr create` (detects if repo changes between status collection and execution).
*   **`manual_test_init_safety.py`**: Verifies safety prompts when initializing in a directory containing unlisted files.
*   **`manual_test_sync_conflict.py`**: Verifies `sync` behavior when merge conflicts occur (should halt and allow manual resolution).
*   **`manual_test_switch_upstream.py`**: Verifies automatic upstream configuration when switching to a branch that exists remotely.
*   **`manual_test_switch_remote.py`**: Verifies fallback behavior: fetching and checking out a remote branch if it doesn't exist locally.
*   **`manual_test_upstream_safety.py`**: Verifies that `status` unsets upstream configuration if branch names mismatch or the remote branch is missing.
*   **`manual_test_pr_create_behind.py`**: Verifies that `pr create` fails if local branches are behind or diverged from the remote.
*   **`manual_test_pr_create_missing_base.py`**: Verifies that `pr create` correctly skips repositories where the configured base branch is missing on the remote.

## Cleanup

The test scripts attempt to clean up the temporary repositories they create (prefixed with `mistletoe-test-` or similar) upon completion or failure.

If a test is interrupted and artifacts remain, you can run the cleanup script (requires `gh` authentication):

```bash
python3 manual_tests/temp_repos_cleanup.py
```
