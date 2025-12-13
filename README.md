# mistletoe

`mstl` is a command-line tool for managing multiple Git repositories using a central JSON configuration file. It simplifies operations like cloning, switching branches, status checking, and syncing across multiple projects concurrently.

## Usage

```bash
mstl <command> [options] [arguments]
```

## Commands

### `init`

Initializes repositories defined in the configuration file. It clones repositories if they don't exist and checks out the specified revision or branch.

**Usage:**
```bash
mstl init -f <config_file> [options]
```

**Options:**
*   `-f, --file <path>`: Configuration file.
*   `-p, --parallel <int>`: Number of parallel processes to use (default: 1).
*   `--depth <int>`: Create a shallow clone with a history truncated to the specified number of commits.

### `status`

Displays a status table for all configured repositories, showing the current branch, remote status, and synchronization state.

**Usage:**
```bash
mstl status -f <config_file> [options]
```

**Options:**
*   `-f, --file <path>`: Configuration file.
*   `-p, --parallel <int>`: Number of parallel processes (default: 1).

**Status Indicators:**
*   `>` (Green): Local branch has unpushed commits.
*   `<` (Yellow): Remote branch has updates (pullable, fast-forward/merge).
*   `x` (Red): Remote branch has updates, but there are conflicts.
*   `-`: Clean (synchronized).

**Table Columns:**
*   **Repository**: Repository ID or directory name.
*   **Config**: Branch/Revision defined in the configuration.
*   **Local**: Current local branch and short commit hash.
*   **Remote**: Remote branch and short commit hash (Yellow if local is behind).
*   **Status**: Synchronization status symbol.

### `push`

Checks for unpushed commits in all repositories and pushes them to the remote `origin`. It prompts for confirmation before executing the push.

**Usage:**
```bash
mstl push -f <config_file> [options]
```

**Options:**
*   `-f, --file <path>`: Configuration file.
*   `-p, --parallel <int>`: Number of parallel processes (default: 1).

### `sync`

Updates repositories by pulling changes from the remote `origin`.
*   If conflicts are detected in any repository, the process aborts.
*   If updates are available, it prompts the user to choose a strategy: `merge`, `rebase`, or `abort`.

**Usage:**
```bash
mstl sync -f <config_file> [options]
```

**Options:**
*   `-f, --file <path>`: Configuration file.
*   `-p, --parallel <int>`: Number of parallel processes (default: 1).

### `switch`

Switches the active branch for all configured repositories. It verifies that the branch exists (or can be created) in all repositories before performing the switch.

**Usage:**
```bash
# Switch to an existing branch
mstl switch -f <config_file> [options] <branch_name>

# Create and switch to a new branch
mstl switch -f <config_file> -c <branch_name>
```

**Options:**
*   `-f, --file <path>`: Configuration file.
*   `-p, --parallel <int>`: Number of parallel processes (default: 1).
*   `-c, --create <branch_name>`: Create a new branch with the specified name and switch to it.

### `freeze`

Scans the current directory for subdirectories that are Git repositories and generates a configuration file representing the current state (URL, Branch/Revision).

**Usage:**
```bash
mstl freeze -f <output_file>
```

**Options:**
*   `-f, --file <path>`: Path for the output configuration file. **(Required)**

### `print`

Prints a simple list of configured repository URLs and their target branches.

**Usage:**
```bash
mstl print -f <config_file>
```

**Options:**
*   `-f, --file <path>`: Configuration file.

### `version`

Prints the version of `mstl` and the path to the git executable being used.

**Usage:**
```bash
mstl version
```

## Configuration File Format

The configuration file is a JSON file containing a list of repositories.

**Example `repos.json`:**

```json
{
  "repositories": [
    {
      "url": "https://github.com/example/repo1.git",
      "branch": "main",
      "id": "my-repo-1"
    },
    {
      "url": "https://github.com/example/repo2.git",
      "revision": "a1b2c3d4",
      "branch": "feature/new-ui"
    }
  ]
}
```

*   **url** (Required): The remote URL of the Git repository.
*   **id** (Optional): The directory name to clone into. If omitted, the name is derived from the URL.
*   **branch** (Optional): The branch to checkout or switch to.
*   **revision** (Optional): A specific commit hash to checkout (primarily used by `init`).
