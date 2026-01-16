#!/usr/bin/env python3
import os
import sys
import subprocess
import shutil
import tempfile
import json
from interactive_runner import InteractiveRunner

def main():
    runner = InteractiveRunner("Manual Test: Init Safety Check")
    runner.parse_args()

    # Create temp directory
    temp_dir = tempfile.mkdtemp(prefix="mstl-init-safety-")
    print(f"Created temp directory: {temp_dir}")

    try:
        # Create config.json
        config = {
            "repositories": [
                {"url": "https://github.com/example/repo1.git", "id": "repo1"}
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
        mstl_path = os.path.abspath("mstl")
        if not os.path.exists(mstl_path):
            # Try go run
            cmd = ["go", "run", "cmd/mstl/main.go", "init", "-f", config_path, "--dest", temp_dir]
        else:
            cmd = [mstl_path, "init", "-f", config_path, "--dest", temp_dir]

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

        if "Cloning https://github.com/example/repo1.git" in stdout or "Cloning..." in stdout:
             runner.log("Proceeded to clone after confirmation.", status="SUCCESS")
        elif "Error" in stdout or "Error" in stderr:
            # If it failed at clone step, we are good.
            if "initialization aborted by user" not in stdout and "initialization aborted by user" not in stderr:
                runner.log("Passed safety check (failed later at clone as expected).", status="SUCCESS")
            else:
                runner.fail("Aborted by user despite sending 'y'.")

    finally:
        shutil.rmtree(temp_dir)
        print("Cleaned up temp directory.")

if __name__ == "__main__":
    main()
