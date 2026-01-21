#!/usr/bin/env python3
import os
import sys
import subprocess
import shutil
import tempfile
import json
from interactive_runner import InteractiveRunner

def create_bare_repo(path):
    """Creates a bare git repository."""
    os.makedirs(path, exist_ok=True)
    subprocess.run(["git", "init", "--bare", path], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)
    # Set default branch to main to avoid confusion
    subprocess.run(["git", "-C", path, "symbolic-ref", "HEAD", "refs/heads/main"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)

def main():
    runner = InteractiveRunner("Manual Test: Init Safety Check")
    runner.parse_args()

    # Create temp directory
    temp_dir = tempfile.mkdtemp(prefix="mstl-init-safety-")
    print(f"Created temp directory: {temp_dir}")

    try:
        # Create a dummy remote repo
        remote_repo_dir = os.path.join(temp_dir, "remote_repo.git")
        create_bare_repo(remote_repo_dir)

        # Create config.json
        config = {
            "repositories": [
                {"url": f"file://{remote_repo_dir}", "id": "repo1"}
            ]
        }
        config_path = os.path.join(temp_dir, "config.json")
        with open(config_path, "w") as f:
            json.dump(config, f)

        # Create an unexpected file
        unexpected_file = os.path.join(temp_dir, "garbage.txt")
        with open(unexpected_file, "w") as f:
            f.write("I am unexpected")

        # Command to run init
        # We must use --ignore-stdin to prevent mstl from reading config from stdin,
        # which would consume the piped input intended for the prompt.
        mstl_path = os.path.abspath("mstl")
        if not os.path.exists(mstl_path):
            # Try go run
            cmd = ["go", "run", "cmd/mstl/main.go", "init", "-f", config_path, "--dest", temp_dir, "--ignore-stdin"]
        else:
            cmd = [mstl_path, "init", "-f", config_path, "--dest", temp_dir, "--ignore-stdin"]

        print("Running init in a dirty directory...")

        print("Test 1: Rejecting the safety check (input 'n')")
        p = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        stdout, stderr = p.communicate(input="n\n")

        print("--- Stdout ---")
        print(stdout)
        print("--- Stderr ---")
        print(stderr)

        if f"Current directory: {temp_dir}" in stdout:
             runner.log(f"Correct directory path displayed: {temp_dir}", status="SUCCESS")
        else:
             runner.fail(f"Directory path mismatch or missing. Expected: {temp_dir}")

        if "This directory contains files/directories not in the repository list" in stdout:
            runner.log("Safety warning displayed.", status="SUCCESS")
        else:
            runner.fail("Safety warning NOT displayed.")

        if "initialization aborted by user" in stderr or "initialization aborted by user" in stdout:
             runner.log("Initialization aborted correctly.", status="SUCCESS")
        elif p.returncode != 0:
             runner.log("Process exited with error as expected.", status="SUCCESS")
        else:
             runner.fail("Process did not fail as expected.")


        print("Test 2: Accepting the safety check (input 'y')")
        p = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        stdout, stderr = p.communicate(input="y\n")

        print("--- Stdout ---")
        print(stdout)
        print("--- Stderr ---")
        print(stderr)

        if "This directory contains files/directories not in the repository list" in stdout:
            runner.log("Safety warning displayed.", status="SUCCESS")

        if f"Cloning file://{remote_repo_dir}" in stdout or "Cloning..." in stdout:
             runner.log("Proceeded to clone after confirmation.", status="SUCCESS")
        elif "Error" in stdout or "Error" in stderr:
            # If it failed at clone step, we are good.
            if "initialization aborted by user" not in stdout and "initialization aborted by user" not in stderr:
                runner.log("Passed safety check (failed later at clone as expected).", status="SUCCESS")
            else:
                runner.fail("Aborted by user despite sending 'y'.")

        print("Test 3: Bypass safety check with --yes")
        # Cleanup cloned repo from Test 2 to ensure Test 3 verifies clone behavior
        shutil.rmtree(os.path.join(temp_dir, "repo1"))

        # Add --yes to the command
        cmd_yes = cmd + ["--yes"]
        p = subprocess.Popen(cmd_yes, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        # No input provided to confirm prompt is skipped
        stdout, stderr = p.communicate()

        print("--- Stdout ---")
        print(stdout)
        print("--- Stderr ---")
        print(stderr)

        if "Are you sure you want to initialize in this directory?" in stdout:
             runner.fail("Prompt displayed despite --yes flag.")
        else:
             runner.log("Prompt skipped with --yes flag.", status="SUCCESS")

        if "This directory contains files/directories not in the repository list" in stdout:
             runner.log("Safety warning message still displayed (as expected).", status="SUCCESS")

        if f"Cloning file://{remote_repo_dir}" in stdout or "Cloning..." in stdout:
             runner.log("Proceeded to clone automatically.", status="SUCCESS")
        elif "Error" in stdout or "Error" in stderr:
            if "initialization aborted by user" not in stdout:
                runner.log("Passed safety check (failed later at clone as expected).", status="SUCCESS")
            else:
                runner.fail("Aborted by user unexpectedly.")

    finally:
        shutil.rmtree(temp_dir)
        print("Cleaned up temp directory.")

if __name__ == "__main__":
    main()
