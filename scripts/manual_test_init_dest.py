#!/usr/bin/env python3
"""
Manual test script for checking the `mstl init --dest` functionality.
This script sets up various file system states and runs `mstl init` to verify
that the destination validation logic works as expected.
"""

import os
import shutil
import subprocess
import tempfile
import sys
import json

# Define colors for output
class Colors:
    HEADER = '\033[95m'
    OKBLUE = '\033[94m'
    OKGREEN = '\033[92m'
    WARNING = '\033[93m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'

def log_header(msg):
    print(f"{Colors.HEADER}=== {msg} ==={Colors.ENDC}")

def log_pass(msg):
    print(f"{Colors.OKGREEN}[PASS] {msg}{Colors.ENDC}")

def log_fail(msg):
    print(f"{Colors.FAIL}[FAIL] {msg}{Colors.ENDC}")

def run_command(cmd, cwd=None, expect_error=False):
    """Runs a shell command and returns the exit code and output."""
    try:
        result = subprocess.run(
            cmd,
            cwd=cwd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True,
            check=False
        )
        return result.returncode, result.stdout, result.stderr
    except Exception as e:
        print(f"Error running command: {e}")
        return -1, "", str(e)

def build_mstl(output_path):
    """Builds the mstl binary."""
    log_header("Building mstl")
    cmd = ["go", "build", "-o", output_path, "./cmd/mstl"]
    code, out, err = run_command(cmd)
    if code != 0:
        log_fail(f"Build failed:\n{out}\n{err}")
        sys.exit(1)
    log_pass("Build successful")

def create_bare_repo(path):
    """Creates a bare git repository."""
    os.makedirs(path, exist_ok=True)
    subprocess.run(["git", "init", "--bare", path], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)
    # Set default branch to main to avoid confusion
    subprocess.run(["git", "-C", path, "symbolic-ref", "HEAD", "refs/heads/main"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)

def main():
    # Setup temporary directory
    root_dir = tempfile.mkdtemp(prefix="mstl_manual_test_")
    mstl_bin = os.path.join(root_dir, "mstl")

    try:
        # Build mstl
        build_mstl(mstl_bin)

        # Setup config file
        # We need a dummy repo to refer to in the config
        repo_dir = os.path.join(root_dir, "upstream_repo.git")
        create_bare_repo(repo_dir)

        # Create a valid config
        config_data = {
            "repositories": [
                {
                    "url": f"file://{repo_dir}",
                    "id": "myrepo"
                }
            ]
        }
        # Place config file in root_dir
        config_file = os.path.join(root_dir, "config.json")
        with open(config_file, "w") as f:
            json.dump(config_data, f)

        # Test Case 1: Destination exists and is a file -> Fail
        log_header("Test Case 1: Destination is a file")
        dest_file = os.path.join(root_dir, "file_dest")
        with open(dest_file, "w") as f:
            f.write("I am a file")

        code, out, err = run_command([mstl_bin, "init", "-f", config_file, "--dest", dest_file], cwd=root_dir)
        if code != 0 and "specified path is a file" in out + err: # checking combined output just in case
            log_pass("Correctly failed when dest is a file")
        else:
            log_fail(f"Expected failure for file destination. Code: {code}, Output: {out}, Error: {err}")

        # Test Case 2: Destination does not exist, parent does not exist -> Fail
        log_header("Test Case 2: Parent directory missing")
        dest_deep = os.path.join(root_dir, "missing_parent", "target")
        code, out, err = run_command([mstl_bin, "init", "-f", config_file, "--dest", dest_deep], cwd=root_dir)
        if code != 0 and "does not exist" in out + err:
            log_pass("Correctly failed when parent directory is missing")
        else:
            log_fail(f"Expected failure for missing parent. Code: {code}, Output: {out}, Error: {err}")

        # Test Case 3: Destination exists, not empty -> Fail
        log_header("Test Case 3: Destination not empty")
        dest_not_empty = os.path.join(root_dir, "not_empty_dir")
        os.makedirs(dest_not_empty)
        with open(os.path.join(dest_not_empty, "junk.txt"), "w") as f:
            f.write("junk")

        code, out, err = run_command([mstl_bin, "init", "-f", config_file, "--dest", dest_not_empty], cwd=root_dir)
        if code != 0 and "is not empty" in out + err:
            log_pass("Correctly failed when destination is not empty")
        else:
            log_fail(f"Expected failure for non-empty dir. Code: {code}, Output: {out}, Error: {err}")

        # Test Case 4: Destination exists, empty -> Success
        log_header("Test Case 4: Destination empty")
        dest_empty = os.path.join(root_dir, "empty_dir")
        os.makedirs(dest_empty)

        code, out, err = run_command([mstl_bin, "init", "-f", config_file, "--dest", dest_empty], cwd=root_dir)
        if code == 0:
            if os.path.exists(os.path.join(dest_empty, "myrepo", ".git")):
                log_pass("Success: Repository cloned into empty destination")
            else:
                log_fail("Success reported, but repository not found in destination")
        else:
            log_fail(f"Expected success for empty dir. Code: {code}, Output: {out}, Error: {err}")

        # Test Case 5: Destination does not exist, parent exists -> Success (Create)
        log_header("Test Case 5: Create new destination")
        dest_new = os.path.join(root_dir, "new_dest")

        code, out, err = run_command([mstl_bin, "init", "-f", config_file, "--dest", dest_new], cwd=root_dir)
        if code == 0:
            if os.path.isdir(dest_new) and os.path.exists(os.path.join(dest_new, "myrepo", ".git")):
                log_pass("Success: Directory created and repository cloned")
            else:
                log_fail("Success reported, but directory not created or repo missing")
        else:
            log_fail(f"Expected success for new dir. Code: {code}, Output: {out}, Error: {err}")

        # Test Case 6: Default destination (current dir)
        log_header("Test Case 6: Default destination (.)")
        # We need a clean subdir to run this in, so we don't mess up the root
        run_subdir = os.path.join(root_dir, "run_subdir")
        os.makedirs(run_subdir)

        # NOTE: This is where we need to check if the script is robust enough.
        # If we run "mstl init" in run_subdir, it defaults to ".", so it should check if run_subdir is empty.
        # run_subdir IS empty (we just made it).
        # But we need to pass the config file. Since config_file is in root_dir, we pass absolute path or relative.
        # The script creates config_file in root_dir.
        # So passing `-f config_file` works if config_file is absolute path.

        # However, let's verify if we put anything inside run_subdir that would cause failure.
        # If we put the config file INSIDE run_subdir, it would fail because dir is not empty.
        # In this script, config_file is at `root_dir/config.json`.
        # So running init in `run_subdir` (which is empty) using a config outside should work.

        code, out, err = run_command([mstl_bin, "init", "-f", config_file], cwd=run_subdir)
        if code == 0:
             if os.path.exists(os.path.join(run_subdir, "myrepo", ".git")):
                log_pass("Success: Cloned into current directory by default")
             else:
                log_fail("Success reported, but repo not found in current directory")
        else:
            log_fail(f"Expected success for default dest. Code: {code}, Output: {out}, Error: {err}")

    finally:
        # Cleanup
        shutil.rmtree(root_dir)

if __name__ == "__main__":
    main()
