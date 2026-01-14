# Configuration Resolution Logic

This document describes the logic for resolving the configuration file path (`.mstl/config.json`) when running `mstl` and `mstl-gh` commands.

## Overview

The tool determines which configuration file to use based on the following priority:

1.  **Standard Input (stdin)**: If configuration data is piped to the command, it is used directly.
2.  **Command Line Flag**: If `--file` or `-f` is specified, that file is used.
3.  **Current Directory**: Defaults to `.mstl/config.json` in the current working directory.
4.  **Parent Directory Search** (New): If not found in the current directory, the tool attempts to locate a configuration in the parent directory of the current Git repository's root, provided specific validation criteria are met.

## Parent Directory Search Logic

When the default configuration file (`.mstl/config.json`) is not found in the current directory, and no file was explicitly provided via flags or stdin, the following search logic is triggered:

1.  **Git Repository Check**: The tool checks if the current directory is inside a Git repository.
2.  **Parent Path Resolution**: If inside a Git repository, it identifies the root of that repository and looks for `.mstl/config.json` in its parent directory (`../.mstl/config.json`).
3.  **Validation**: If a configuration file is found in the parent directory, it validates whether the current directory structure matches the configuration.
    *   For each repository defined in the configuration:
        *   Checks if a directory with the Repository ID exists in the parent directory.
        *   Checks if that directory is a Git repository.
        *   Verifies that the `origin` remote URL of that Git repository matches the URL defined in the configuration.
4.  **User Prompt**: If validation succeeds, the user is prompted to confirm usage of the found configuration:
    > "Current directory does not have .mstl, but found one in {parent}/.mstl. Use this configuration? (yes/no)"
5.  **Fallback**:
    *   If validation fails, or the user declines, the tool reports the original "configuration file not found" error.
    *   If the user accepts, the discovered configuration file is used.

## Rationale

This feature improves the developer experience when working inside a specific repository managed by `mstl`. It allows users to run `mstl` commands from within a managed sub-repository without needing to navigate up to the root or manually specify the configuration file path, provided the structure is consistent.
