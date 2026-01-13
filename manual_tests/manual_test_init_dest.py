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
import atexit

# Add current directory to sys.path to import interactive_runner
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from interactive_runner import InteractiveRunner, print_green, print_red

def log_header(msg):
    print_green(f"=== {msg} ===")

def log_pass(msg):
    print_green(f"[PASS] {msg}")

def log_fail(msg):
    print_red(f"[FAIL] {msg}")
    sys.exit(1)

def run_command(cmd, cwd=None, expect_error=False):
    """Runs a shell command and returns the exit code and output."""
    # Ensure --verbose is present for mstl commands
    if os.path.basename(cmd[0]).startswith('mstl') and "--verbose" not in cmd:
        cmd = cmd + ["--verbose"]

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

def create_bare_repo(path):
    """Creates a bare git repository."""
    os.makedirs(path, exist_ok=True)
    subprocess.run(["git", "init", "--bare", path], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)
    # Set default branch to main to avoid confusion
    subprocess.run(["git", "-C", path, "symbolic-ref", "HEAD", "refs/heads/main"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)

class InitDestTest:
    def __init__(self):
        self.root_dir = None

    def cleanup(self):
        if self.root_dir and os.path.exists(self.root_dir):
            print_green("Cleaning up temporary directory...")
            try:
                shutil.rmtree(self.root_dir)
            except Exception as e:
                print(f"Cleanup failed: {e}")

    def run(self):
        # Setup temporary directory
        self.root_dir = tempfile.mkdtemp(prefix="mstl_manual_test_")

        # Ensure cleanup runs even if we exit early via sys.exit(1)
        atexit.register(self.cleanup)

        script_dir = os.path.dirname(os.path.abspath(__file__))
        mstl_bin = os.path.abspath(os.path.join(script_dir, "../bin/mstl"))
        if sys.platform == "win32":
            mstl_bin += ".exe"

        if not os.path.exists(mstl_bin):
            log_fail(f"mstl binary not found at {mstl_bin}. Please run build_all.sh first.")

        # Setup config file
        # We need a dummy repo to refer to in the config
        repo_dir = os.path.join(self.root_dir, "upstream_repo.git")
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
        config_file = os.path.join(self.root_dir, "config.json")
        with open(config_file, "w") as f:
            json.dump(config_data, f)

        # Test Case 1: Destination exists and is a file -> Fail
        log_header("Test Case 1: Destination is a file")
        dest_file = os.path.join(self.root_dir, "file_dest")
        with open(dest_file, "w") as f:
            f.write("I am a file")

        code, out, err = run_command([mstl_bin, "init", "-f", config_file, "--dest", dest_file, "--ignore-stdin"], cwd=self.root_dir)
        if code != 0 and "specified path is a file" in out + err: # checking combined output just in case
            log_pass("Correctly failed when dest is a file")
        else:
            log_fail(f"Expected failure for file destination. Code: {code}, Output: {out}, Error: {err}")

        # Test Case 2: Destination does not exist, parent does not exist -> Fail
        log_header("Test Case 2: Parent directory missing")
        dest_deep = os.path.join(self.root_dir, "missing_parent", "target")
        code, out, err = run_command([mstl_bin, "init", "-f", config_file, "--dest", dest_deep, "--ignore-stdin"], cwd=self.root_dir)
        if code != 0 and "does not exist" in out + err:
            log_pass("Correctly failed when parent directory is missing")
        else:
            log_fail(f"Expected failure for missing parent. Code: {code}, Output: {out}, Error: {err}")

        # Test Case 3: Destination exists, not empty (Global check removed, but repo check strict)
        log_header("Test Case 3: Destination not empty (with conflict)")
        dest_not_empty = os.path.join(self.root_dir, "not_empty_dir")
        os.makedirs(dest_not_empty)
        # Create a conflicting repo directory that is not empty and not a git repo
        conflict_repo = os.path.join(dest_not_empty, "myrepo")
        os.makedirs(conflict_repo)
        with open(os.path.join(conflict_repo, "junk.txt"), "w") as f:
            f.write("junk")

        code, out, err = run_command([mstl_bin, "init", "-f", config_file, "--dest", dest_not_empty, "--ignore-stdin"], cwd=self.root_dir)
        if code != 0 and "directory myrepo exists, is not empty" in out + err:
            log_pass("Correctly failed when repo target is not empty and ineligible")
        else:
            log_fail(f"Expected failure for non-empty conflicted repo. Code: {code}, Output: {out}, Error: {err}")

        # Test Case 4: Destination exists, empty -> Success
        log_header("Test Case 4: Destination empty")
        dest_empty = os.path.join(self.root_dir, "empty_dir")
        os.makedirs(dest_empty)

        code, out, err = run_command([mstl_bin, "init", "-f", config_file, "--dest", dest_empty, "--ignore-stdin"], cwd=self.root_dir)
        if code == 0:
            if os.path.exists(os.path.join(dest_empty, "myrepo", ".git")):
                log_pass("Success: Repository cloned into empty destination")
            else:
                log_fail("Success reported, but repository not found in destination")
        else:
            log_fail(f"Expected success for empty dir. Code: {code}, Output: {out}, Error: {err}")

        # Test Case 5: Destination does not exist, parent exists -> Success (Create)
        log_header("Test Case 5: Create new destination")
        dest_new = os.path.join(self.root_dir, "new_dest")

        code, out, err = run_command([mstl_bin, "init", "-f", config_file, "--dest", dest_new, "--ignore-stdin"], cwd=self.root_dir)
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
        run_subdir = os.path.join(self.root_dir, "run_subdir")
        os.makedirs(run_subdir)

        code, out, err = run_command([mstl_bin, "init", "-f", config_file, "--ignore-stdin"], cwd=run_subdir)
        if code == 0:
             if os.path.exists(os.path.join(run_subdir, "myrepo", ".git")):
                log_pass("Success: Cloned into current directory by default")
             else:
                log_fail("Success reported, but repo not found in current directory")
        else:
            log_fail(f"Expected success for default dest. Code: {code}, Output: {out}, Error: {err}")

        print_green("All tests passed.")

def main():
    runner = InteractiveRunner("Init Destination Test")
    runner.parse_args()
    test = InitDestTest()

    description = (
        "This test verifies 'mstl init' with various destination scenarios:\n"
        "- Destination is a file (Should Fail)\n"
        "- Parent directory missing (Should Fail)\n"
        "- Destination not empty & conflict (Should Fail)\n"
        "- Destination empty (Should Success)\n"
        "- Destination new (Should Success)\n"
        "- Default destination (Should Success)"
    )

    runner.execute_scenario(
        "Destination Logic Check",
        description,
        test.run
    )

    runner.run_cleanup(test.cleanup)

if __name__ == "__main__":
    main()
