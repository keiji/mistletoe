# `init` Subcommand Design Document

## 1. Overview

The `init` subcommand initializes the local environment by cloning and configuring the repositories specified in the configuration file. It ensures the environment matches the desired state defined in the configuration, supporting parallel execution and shallow cloning.

## 2. Usage

```bash
mstl init --file <path> [options]
```

### Options

| Option | Short | Description | Default |
| :--- | :--- | :--- | :--- |
| `--file` | `-f` | **(Required)** Path to the configuration file (JSON). | - |
| `--depth` | | Create a shallow clone with a history truncated to the specified number of commits. | 0 (full clone) |
| `--parallel` | `-p` | Number of parallel processes to use for cloning/checking out. | 1 |

## 3. Configuration Structure

The command expects a JSON configuration file containing a list of repositories.

```json
{
  "repositories": [
    {
      "url": "https://github.com/example/repo.git",
      "id": "repo-directory-name",
      "branch": "main",
      "revision": "commit-hash"
    }
  ]
}
```

*   **url**: (Required) The remote repository URL.
*   **id**: (Optional) The directory name for the repository. If omitted, the directory name is derived from the URL (base name).
*   **branch**: (Optional) The target branch to checkout.
*   **revision**: (Optional) The target commit hash to checkout.

## 4. Logic Flow

The execution flow consists of **Configuration Loading**, **Environment Validation**, and **Parallel Execution**.

### 4.1. Flowchart

```mermaid
flowchart TD
    Start([Start]) --> ParseArgs[Parse Arguments]
    ParseArgs --> CheckFile{Config File Specified?}
    CheckFile -- No --> ErrorFile[Error: File Required]
    CheckFile -- Yes --> LoadConfig[Load Configuration]
    LoadConfig --> ValidateEnv[Validate Environment (All Repos)]

    ValidateEnv -- "Error (Repo invalid, Dir not empty, URL mismatch)" --> ErrorExit([Exit with Error])
    ValidateEnv -- "Success" --> ExecLoop[Parallel Execution Loop]

    subgraph "Per Repository Execution"
        ExecLoop --> CheckState{Check Dir State}

        CheckState -- "Dir Missing or Empty" --> Clone[git clone]
        CheckState -- "Git Repo Exists" --> SkipClone[Skip Clone]

        Clone --> CheckRev{Config has Revision?}
        SkipClone --> CheckRev

        CheckRev -- Yes --> CheckoutRev[git checkout revision]
        CheckoutRev --> CheckBranchWithRev{Config has Branch?}

        CheckBranchWithRev -- Yes --> CreateBranch[git checkout -b branch]
        CheckBranchWithRev -- No --> Detached[Result: Detached HEAD]

        CheckRev -- No --> CheckBranchOnly{Config has Branch?}

        CheckBranchOnly -- Yes --> CheckoutBranch[git checkout branch]
        CheckBranchOnly -- No --> DefaultBranch[Result: Default Branch]
    end

    ErrorFile --> ErrorExit
    ErrorExit --> Stop([Stop])
```

### 4.2. Environment Validation

Before performing any write operations, `init` validates the environment to ensure consistency and prevent data loss. If any check fails for any repository, the command aborts immediately.

1.  **Directory Consistency**:
    *   If a target directory exists but is **not** a Git repository, it must be empty. If it contains files, validation fails.
    *   If a target directory exists and **is** a Git repository, its `remote.origin.url` must match the configuration.

2.  **Branch Conflict Check**:
    *   If both `revision` and `branch` are specified for a repository, the command intends to create a new branch from that revision.
    *   Validation checks if the specified `branch` already exists (either locally or remotely). If it does, validation fails to prevent overwriting an existing branch.

### 4.3. Execution Logic

Repositories are processed in parallel (up to the limit specified by `--parallel`).

1.  **Cloning**:
    *   Executed if the directory does not exist or is empty.
    *   Skipped if a valid Git repository already exists at the target path.
    *   Applies `--depth` if specified (only during clone).

2.  **Checkout / Switch**:
    *   **Revision & Branch**: Checks out the specific `revision` (detached HEAD), then creates the `branch` (`git checkout -b`).
    *   **Revision Only**: Checks out the `revision` and leaves the repository in a detached HEAD state.
    *   **Branch Only**: Switches to the existing `branch`.
    *   **Neither**: No action taken after clone (remains on default branch).
