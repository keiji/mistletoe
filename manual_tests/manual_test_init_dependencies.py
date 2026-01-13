#!/usr/bin/env python3
"""
Manual test script for checking the `mstl init --dependencies` functionality.
"""

import os
import shutil
import tempfile
import subprocess
import json
import sys
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
    # Ensure --verbose is present for mstl commands
    if os.path.basename(cmd[0]).startswith('mstl') and "--verbose" not in cmd:
        cmd = cmd + ["--verbose"]

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

class InitDependenciesTest:
    def __init__(self):
        self.root_dir = None
        self.mstl_bin = None

    def cleanup(self):
        if self.root_dir and os.path.exists(self.root_dir):
            print_green("Cleaning up temporary directory...")
            try:
                shutil.rmtree(self.root_dir)
            except Exception as e:
                print(f"Cleanup failed: {e}")

    def run(self):
        # Setup temporary directory
        self.root_dir = tempfile.mkdtemp(prefix="mstl_manual_test_deps_")
        atexit.register(self.cleanup)

        script_dir = os.path.dirname(os.path.abspath(__file__))
        self.mstl_bin = os.path.abspath(os.path.join(script_dir, "../bin/mstl"))
        if sys.platform == "win32":
            self.mstl_bin += ".exe"

        if not os.path.exists(self.mstl_bin):
            log_fail(f"mstl binary not found at {self.mstl_bin}. Please run build_all.sh first.")

        # Setup 3 bare repos
        repos = {}
        for name in ["repoA", "repoB", "repoC"]:
            repo_path = os.path.join(self.root_dir, name)
            os.makedirs(repo_path)
            subprocess.run(["git", "init", "--bare"], cwd=repo_path, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)
            # Set default branch
            subprocess.run(["git", "-C", repo_path, "symbolic-ref", "HEAD", "refs/heads/main"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)
            repos[name] = repo_path

        # Create config.json
        config_path = os.path.join(self.root_dir, "config.json")
        config = {
            "repositories": [
                {"id": "repoA", "url": "file://" + repos["repoA"]},
                {"id": "repoB", "url": "file://" + repos["repoB"]},
                {"id": "repoC", "url": "file://" + repos["repoC"]}
            ]
        }
        with open(config_path, "w") as f:
            json.dump(config, f)

        # Create valid dependency graph
        dep_path = os.path.join(self.root_dir, "dep.md")
        with open(dep_path, "w") as f:
            f.write("```mermaid\ngraph TD\n    repoA --> repoB\n    repoB --> repoC\n```\n")

        # Create invalid dependency graph (syntax ok, invalid ID)
        invalid_dep_path = os.path.join(self.root_dir, "invalid_dep.md")
        with open(invalid_dep_path, "w") as f:
            f.write("```mermaid\ngraph TD\n    repoA --> repoZ\n```\n")

        # Test Case 1: Valid dependencies
        log_header("Test Case 1: Init with valid dependencies")
        dest_dir_valid = os.path.join(self.root_dir, "work_valid")

        cmd = [
            self.mstl_bin, "init",
            "-f", config_path,
            "--dependencies", dep_path,
            "--dest", dest_dir_valid,
            "--ignore-stdin"
        ]

        code, out, err = run_command(cmd)
        if code != 0:
            log_fail(f"Init failed for valid deps. Code: {code}, Output: {out}, Error: {err}")

        # Check .mstl/dependency-graph.md
        dep_output = os.path.join(dest_dir_valid, ".mstl", "dependency-graph.md")
        if not os.path.exists(dep_output):
            log_fail("dependency-graph.md not created")

        with open(dep_output, "r") as f:
            content = f.read()
            if "repoA --> repoB" in content and "repoB --> repoC" in content:
                log_pass("Correctly validated and copied valid dependency graph")
            else:
                log_fail(f"Content mismatch in dependency graph: {content}")

        # Test Case 2: Invalid dependencies (ID not found)
        log_header("Test Case 2: Init with invalid dependencies (ID not found)")
        dest_dir_invalid = os.path.join(self.root_dir, "work_invalid")

        cmd = [
            self.mstl_bin, "init",
            "-f", config_path,
            "--dependencies", invalid_dep_path,
            "--dest", dest_dir_invalid,
            "--ignore-stdin"
        ]

        code, out, err = run_command(cmd)
        if code != 0 and "Error validating dependency graph" in out + err and "not found in configuration" in out + err:
            log_pass("Correctly failed when dependency graph contains invalid ID")
        else:
            log_fail(f"Expected failure for invalid ID. Code: {code}, Output: {out}, Error: {err}")

        # Test Case 3: Missing dependency file
        log_header("Test Case 3: Init with missing dependency file")
        dest_dir_missing = os.path.join(self.root_dir, "work_missing")

        cmd = [
            self.mstl_bin, "init",
            "-f", config_path,
            "--dependencies", os.path.join(self.root_dir, "does_not_exist.md"),
            "--dest", dest_dir_missing,
            "--ignore-stdin"
        ]

        code, out, err = run_command(cmd)
        if code != 0 and "Error reading dependency file" in out + err:
            log_pass("Correctly failed when dependency file is missing")
        else:
            log_fail(f"Expected failure for missing file. Code: {code}, Output: {out}, Error: {err}")

        print_green("All tests passed.")

def main():
    runner = InteractiveRunner("Init Dependencies Test")
    runner.parse_args()
    test = InitDependenciesTest()

    description = (
        "This test verifies 'mstl init --dependencies' functionality:\n"
        "- Valid dependency file (Should Success & Copy)\n"
        "- Invalid ID in dependency file (Should Fail with 'not found')\n"
        "- Missing dependency file (Should Fail)"
    )

    runner.execute_scenario(
        "Dependencies Flag Check",
        description,
        test.run
    )

    runner.run_cleanup(test.cleanup)

if __name__ == "__main__":
    main()
