# gitc

`gitc` is a command-line tool for managing multiple Git repositories using a central JSON configuration file. It simplifies operations like cloning, switching branches, status checking, and syncing across multiple projects concurrently.

## Usage

```bash
gitc [global options] <command> [command options] [arguments]
```

## Global Options

*   `-f, --file <path>`: Specifies the path to the configuration file. This is required for most commands unless specified as a subcommand option.
*   `-v, --version`: Prints the version information and exits.

## Commands

### `init`

Initializes repositories defined in the configuration file. It clones repositories if they don't exist and checks out the specified revision or branch.

**Usage:**
```bash
gitc init [options]
```

**Options:**
*   `-f, --file <path>`: Configuration file (overrides global option).
*   `-p, --parallel <int>`: Number of parallel processes to use (default: 1).
*   `--depth <int>`: Create a shallow clone with a history truncated to the specified number of commits.

### `status`

Displays a status table for all configured repositories, showing the current branch, remote status, and synchronization state.

**Usage:**
```bash
gitc status [options]
```

**Options:**
*   `-f, --file <path>`: Configuration file (overrides global option).
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
gitc push [options]
```

**Options:**
*   `-f, --file <path>`: Configuration file (overrides global option).
*   `-p, --parallel <int>`: Number of parallel processes (default: 1).

### `sync`

Updates repositories by pulling changes from the remote `origin`.
*   If conflicts are detected in any repository, the process aborts.
*   If updates are available, it prompts the user to choose a strategy: `merge`, `rebase`, or `abort`.

**Usage:**
```bash
gitc sync [options]
```

**Options:**
*   `-f, --file <path>`: Configuration file (overrides global option).
*   `-p, --parallel <int>`: Number of parallel processes (default: 1).

### `switch`

Switches the active branch for all configured repositories. It verifies that the branch exists (or can be created) in all repositories before performing the switch.

**Usage:**
```bash
# Switch to an existing branch
gitc switch [options] <branch_name>

# Create and switch to a new branch
gitc switch [options] -c <branch_name>
```

**Options:**
*   `-f, --file <path>`: Configuration file (overrides global option).
*   `-p, --parallel <int>`: Number of parallel processes (default: 1).
*   `-c, --create <branch_name>`: Create a new branch with the specified name and switch to it.

### `freeze`

Scans the current directory for subdirectories that are Git repositories and generates a configuration file representing the current state (URL, Branch/Revision).

**Usage:**
```bash
gitc freeze -f <output_file>
```

**Options:**
*   `-f, --file <path>`: Path for the output configuration file. **(Required)**

### `print`

Prints a simple list of configured repository URLs and their target branches.

**Usage:**
```bash
gitc -f <config_file> print
```

*Note: The `print` command relies on the global `-f` flag.*

### `version`

Prints the version of `gitc` and the path to the git executable being used.

**Usage:**
```bash
gitc version
```

## Configuration File Format

The configuration file is a JSON file containing a list of repositories.

**Example `gitc.json`:**

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
