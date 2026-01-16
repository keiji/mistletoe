#!/usr/bin/env python3
import os
import shutil
import subprocess
import sys
import json
import time

# Add current directory to sys.path to import interactive_runner
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from interactive_runner import InteractiveRunner, print_green, print_red

def run_test_logic():
    # Get absolute path to main.go
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)
    main_go_path = os.path.join(project_root, "cmd", "mstl", "main.go")

    test_workspace = os.path.abspath("manual_test_switch_remote_workspace")
    if os.path.exists(test_workspace):
        shutil.rmtree(test_workspace)
    os.makedirs(test_workspace)

    try:
        print_green("Setting up local test environment...")

        # 1. Initialize remote repo (bare)
        remote_dir = os.path.join(test_workspace, "remote.git")
        os.makedirs(remote_dir)
        subprocess.run(["git", "init", "--bare"], cwd=remote_dir, check=True, stdout=subprocess.DEVNULL)

        # 2. Initialize origin setup repo to push content to remote
        origin_setup_dir = os.path.join(test_workspace, "origin_setup")
        os.makedirs(origin_setup_dir)
        subprocess.run(["git", "init"], cwd=origin_setup_dir, check=True, stdout=subprocess.DEVNULL)
        with open(os.path.join(origin_setup_dir, "README.md"), "w") as f:
            f.write("# Test Repo\n")
        subprocess.run(["git", "add", "README.md"], cwd=origin_setup_dir, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=origin_setup_dir, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "remote", "add", "origin", remote_dir], cwd=origin_setup_dir, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "push", "-u", "origin", "master"], cwd=origin_setup_dir, check=True, stdout=subprocess.DEVNULL)

        # Create a feature branch and push it
        branch_name = "feature/remote-only"
        subprocess.run(["git", "checkout", "-b", branch_name], cwd=origin_setup_dir, check=True, stdout=subprocess.DEVNULL)
        with open(os.path.join(origin_setup_dir, "feature.txt"), "w") as f:
            f.write("Feature content")
        subprocess.run(["git", "add", "feature.txt"], cwd=origin_setup_dir, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "commit", "-m", "Feature commit"], cwd=origin_setup_dir, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "push", "-u", "origin", branch_name], cwd=origin_setup_dir, check=True, stdout=subprocess.DEVNULL)

        # Switch back to master
        subprocess.run(["git", "checkout", "master"], cwd=origin_setup_dir, check=True, stdout=subprocess.DEVNULL)

        # 3. Clone to local_dir (the one managed by mstl)
        local_dir = os.path.join(test_workspace, "local_repo")
        subprocess.run(["git", "clone", remote_dir, local_dir], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

        # 4. Ensure the branch does NOT exist locally
        # git clone usually maps remote branches to origin/, but only creates local 'master'
        # Verify it doesn't exist locally
        proc = subprocess.run(["git", "show-ref", "--verify", "--quiet", "refs/heads/" + branch_name], cwd=local_dir)
        if proc.returncode == 0:
            print("Branch existed locally unexpectedly. Deleting it.")
            subprocess.run(["git", "branch", "-D", branch_name], cwd=local_dir, check=True, stdout=subprocess.DEVNULL)

        # 5. Create mstl config
        # Use "id" to specify the local directory name ("local_repo")
        config = {
            "repositories": [
                {"id": "local_repo", "url": remote_dir}
            ]
        }
        config_path = os.path.join(test_workspace, "mstl_config.json")
        with open(config_path, "w") as f:
            json.dump(config, f, indent=2)

        print_green(f"Running mstl switch {branch_name}...")

        cmd = ["go", "run", main_go_path, "switch", branch_name, "-f", config_path, "--ignore-stdin", "-v"]

        result = subprocess.run(cmd, cwd=test_workspace, capture_output=True, text=True)

        print("STDOUT:", result.stdout)
        if result.returncode != 0:
            print("STDERR:", result.stderr)
            raise Exception("Switch command failed. (This is expected before the fix).")

        print_green("Switch command succeeded.")

        # Verify we are on the branch
        res = subprocess.run(["git", "symbolic-ref", "--short", "HEAD"], cwd=local_dir, capture_output=True, text=True)
        current_branch = res.stdout.strip()
        if current_branch != branch_name:
            raise Exception(f"Current branch is {current_branch}, expected {branch_name}")

        print_green("Verified: Branch checked out correctly.")

    finally:
        if os.path.exists(test_workspace):
             # cleanup
             shutil.rmtree(test_workspace)

def main():
    runner = InteractiveRunner("Switch Remote Fallback Test")
    runner.parse_args()

    description = (
        "This test verifies that 'mstl switch <branch>' works when the branch\n"
        "does not exist locally but exists on the remote."
    )

    runner.execute_scenario(
        "Switch Remote Fallback Verification",
        description,
        run_test_logic
    )

if __name__ == "__main__":
    main()
