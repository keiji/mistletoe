import argparse
import os
import shutil
import subprocess
import sys
import json
import time

def run_command(command, cwd=None, capture_output=True, check=True):
    try:
        result = subprocess.run(
            command,
            cwd=cwd,
            check=check,
            capture_output=capture_output,
            text=True
        )
        return result
    except subprocess.CalledProcessError as e:
        print(f"Command failed: {e.cmd}")
        print(f"Stdout: {e.stdout}")
        print(f"Stderr: {e.stderr}")
        raise

def setup_test_env(base_dir):
    remote_dir = os.path.join(base_dir, "remote")
    os.makedirs(remote_dir, exist_ok=True)
    run_command(["git", "init", "--bare"], cwd=remote_dir)

    # Initial content for remote
    temp_clone = os.path.join(base_dir, "temp_clone")
    run_command(["git", "clone", remote_dir, temp_clone])

    with open(os.path.join(temp_clone, "README.md"), "w") as f:
        f.write("Initial")

    run_command(["git", "add", "."], cwd=temp_clone)
    run_command(["git", "commit", "-m", "Initial commit"], cwd=temp_clone)
    run_command(["git", "push", "origin", "master"], cwd=temp_clone)

    # Create feature branch on remote
    run_command(["git", "checkout", "-b", "feature-branch"], cwd=temp_clone)
    with open(os.path.join(temp_clone, "feature.txt"), "w") as f:
        f.write("Feature")
    run_command(["git", "add", "."], cwd=temp_clone)
    run_command(["git", "commit", "-m", "Feature commit"], cwd=temp_clone)
    run_command(["git", "push", "origin", "feature-branch"], cwd=temp_clone)

    shutil.rmtree(temp_clone)
    return remote_dir

def main():
    base_dir = os.path.abspath("test_repro_dir")
    if os.path.exists(base_dir):
        shutil.rmtree(base_dir)
    os.makedirs(base_dir)

    print(f"Setting up test environment in {base_dir}")
    remote_dir = setup_test_env(base_dir)

    # Step 1: Initialize local repo with master
    local_dir = os.path.join(base_dir, "local")

    config_1 = {
        "repositories": [
            {
                "url": remote_dir,
                "id": "repo",
                "branch": "master"
            }
        ]
    }

    config_file_1 = os.path.join(base_dir, "config1.json")
    with open(config_file_1, "w") as f:
        json.dump(config_1, f)

    print("Running mstl init (1)...")
    # Using 'mstl' command assuming it's in path or built.
    # I'll use 'go run cmd/mstl/main.go' for safety if needed, but 'mstl' is standard here?
    # Usually in this environment I need to run 'go run ...' or build it.
    # I'll try to build it first.

    mstl_bin = os.path.join(os.getcwd(), "mstl_bin")
    run_command(["go", "build", "-o", mstl_bin, "./cmd/mstl/main.go"])

    run_command([mstl_bin, "init", "-f", config_file_1, "--dest", local_dir])

    # Verify we are on master
    repo_dir = os.path.join(local_dir, "repo")
    res = run_command(["git", "rev-parse", "--abbrev-ref", "HEAD"], cwd=repo_dir)
    print(f"Current branch: {res.stdout.strip()}")
    assert res.stdout.strip() == "master"

    # Step 2: Change config to use feature-branch
    # Note: local repo does NOT have feature-branch fetched yet if we cloned with default (usually fetches all heads? yes. but 'clone' fetches all heads)
    # Wait, 'git clone' fetches all remote refs. So 'origin/feature-branch' SHOULD exist.

    # Let's verify if origin/feature-branch exists in local
    res = run_command(["git", "branch", "-r"], cwd=repo_dir)
    print(f"Remote branches:\n{res.stdout}")

    # If I want to simulate the failure, I must make sure the local repo is unaware of the new branch?
    # Or maybe the user scenario is: someone added a branch to remote AFTER I cloned?

    # Let's Add another branch to remote NOW.
    temp_clone = os.path.join(base_dir, "temp_clone_2")
    run_command(["git", "clone", remote_dir, temp_clone])
    run_command(["git", "checkout", "-b", "new-feature"], cwd=temp_clone)
    run_command(["git", "push", "origin", "new-feature"], cwd=temp_clone)
    shutil.rmtree(temp_clone)

    # Now local 'repo' does NOT know about 'new-feature'.

    config_2 = {
        "repositories": [
            {
                "url": remote_dir,
                "id": "repo",
                "branch": "new-feature"
            }
        ]
    }
    config_file_2 = os.path.join(base_dir, "config2.json")
    with open(config_file_2, "w") as f:
        json.dump(config_2, f)

    print("Running mstl init (2) with new-feature branch...")
    try:
        run_command([mstl_bin, "init", "-f", config_file_2, "--dest", local_dir])
    except Exception as e:
        print("mstl init failed as expected (maybe?)")
        # Check output if possible? run_command prints it.

    # Check if switched
    res = run_command(["git", "rev-parse", "--abbrev-ref", "HEAD"], cwd=repo_dir)
    current_branch = res.stdout.strip()
    print(f"Current branch after init (2): {current_branch}")

    if current_branch == "new-feature":
        print("SUCCESS: Switched to new-feature")
        sys.exit(0) # Should fail to reproduce?
    else:
        print("FAILURE: Did not switch to new-feature")
        sys.exit(1)

if __name__ == "__main__":
    main()
