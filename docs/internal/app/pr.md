# PR Subcommand Design

## Overview
The `pr` subcommand provides integration with GitHub Pull Requests, allowing users to create, view, and checkout PRs across multiple repositories managed by `mistletoe`.

## Specifications

### `pr create`
Creates Pull Requests for multiple repositories.

- **Flags**:
  - `--title`, `-t`: Specify the PR title.
  - `--body`, `-b`: Specify the PR body.
  - `--dependencies`, `-d`: Specify a Mermaid dependency graph file.
  - `--parallel`, `-p`: Parallel execution limit.
  - `--verbose`, `-v`: Enable verbose output.

- **Flow**:
  1.  **Status Collection**: Checks local status of all repositories.
  2.  **Validation**:
      -   Ensures repositories are on GitHub.
      -   Checks for write permissions.
      -   Verifies base branch existence on remote.
      -   Checks for existing PRs.
      -   Ensures no unpulled commits or conflicts.
  3.  **Input**:
      -   If `--title` or `--body` are missing, opens the default editor.
      -   **Parsing Logic**:
          -   If the first line of the input exceeds `PrTitleMaxLength` (256 characters):
              -   Title: First line truncated to `PrTitleMaxLength - 3` + `...`
              -   Body: The entire original input.
          -   If the first line is followed immediately by an empty line:
              -   Title: The first line.
              -   Body: The content starting from the third line (after the empty line).
          -   Otherwise:
              -   Title: The first line.
              -   Body: The rest of the content.
  4.  **Execution**:
      -   Pushes changes to the remote branch.
      -   Creates the PR using `gh pr create`.
      -   Uses a placeholder body initially.
  5.  **Snapshot & Update**:
      -   Generates a snapshot of the current configuration (revision/branch).
      -   Updates the PR body with the Mistletoe block (Snapshot + Related PRs + Dependency Graph).

### `pr status`
Displays the status of Pull Requests for the current branch.

-   **Columns**: Repository, Base, Branch/Rev, Status, PR (URL/Number).

### `pr checkout`
Restores the environment from a Mistletoe-managed PR.

-   **Flow**:
    -   Fetches the PR description.
    -   Parses the Mistletoe snapshot.
    -   Initializes repositories based on the snapshot.

## Internal Logic

### Mistletoe Body Block
The Mistletoe block is a structured section in the PR body containing:
1.  **Related Pull Requests**: Links to other PRs in the same group.
2.  **Snapshot**: A JSON object (hidden in `<details>`) defining the exact state of all repositories.
3.  **Dependency Graph**: A visual representation of repository dependencies (if provided).
