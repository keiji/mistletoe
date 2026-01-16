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
            text=True,
            stdin=subprocess.DEVNULL
        )
        return result
    except subprocess.CalledProcessError as e:
        print(f"Command failed: {e.cmd}")
        print(f"Stdout: {e.stdout}")
        print(f"Stderr: {e.stderr}")
        raise

def setup_test_env(base_dir):
    remote_dir = os.path.join(base_dir, "remote")
    if os.path.exists(remote_dir):
        shutil.rmtree(remote_dir)
    os.makedirs(remote_dir, exist_ok=True)
    run_command(["git", "init", "--bare"], cwd=remote_dir)

    # Initial content for remote
    temp_clone = os.path.join(base_dir, "temp_clone")
    if os.path.exists(temp_clone):
        shutil.rmtree(temp_clone)

    run_command(["git", "clone", remote_dir, temp_clone])

    with open(os.path.join(temp_clone, "README.md"), "w") as f:
        f.write("Initial")

    run_command(["git", "add", "."], cwd=temp_clone)
    run_command(["git", "config", "user.email", "you@example.com"], cwd=temp_clone)
    run_command(["git", "config", "user.name", "Your Name"], cwd=temp_clone)
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
    base_dir = os.path.abspath("test_repro_dir_v2")
    if os.path.exists(base_dir):
        shutil.rmtree(base_dir)
    os.makedirs(base_dir)

    print(f"Setting up test environment in {base_dir}")
    remote_dir = setup_test_env(base_dir)

    # Build mstl
    mstl_bin = os.path.join(os.getcwd(), "mstl_bin_v2")
    run_command(["go", "build", "-o", mstl_bin, "./cmd/mstl/main.go"])

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

    print(f"Config 1 content: {open(config_file_1).read()}")

    print("Running mstl init (1)...")
    run_command([mstl_bin, "init", "-f", config_file_1, "--dest", local_dir])

    # Verify we are on master
    repo_dir = os.path.join(local_dir, "repo")
    res = run_command(["git", "rev-parse", "--abbrev-ref", "HEAD"], cwd=repo_dir)
    print(f"Current branch: {res.stdout.strip()}")
    assert res.stdout.strip() == "master"

    # Create new feature branch on remote that local doesn't know about
    temp_clone = os.path.join(base_dir, "temp_clone_2")
    run_command(["git", "clone", remote_dir, temp_clone])
    run_command(["git", "config", "user.email", "you@example.com"], cwd=temp_clone)
    run_command(["git", "config", "user.name", "Your Name"], cwd=temp_clone)
    run_command(["git", "checkout", "-b", "new-feature"], cwd=temp_clone)
    run_command(["git", "push", "origin", "new-feature"], cwd=temp_clone)
    shutil.rmtree(temp_clone)

    # Step 2: Change config to use new-feature
    config_2 = {
        "repositories": [
            {
                "url": remote_dir,
                "id": "repo",
                "branch": "new-feature",
                "base-branch": "feature-branch"
            }
        ]
    }
    # Note: 'feature-branch' exists on remote but we haven't fetched it explicitly?
    # Actually when we cloned master, we likely got 'origin/feature-branch' too.
    # But 'new-feature' is definitely new.

    config_file_2 = os.path.join(base_dir, "config2.json")
    with open(config_file_2, "w") as f:
        json.dump(config_2, f)

    print(f"Config 2 content: {open(config_file_2).read()}")

    print("Running mstl init (2) with new-feature branch...")

    # This should fail to switch branch with current code
    result = run_command([mstl_bin, "init", "-f", config_file_2, "--dest", local_dir, "-v"], check=False)

    print("Init (2) Output:")
    print(result.stdout)
    print(result.stderr)

    # Check if switched
    res = run_command(["git", "rev-parse", "--abbrev-ref", "HEAD"], cwd=repo_dir)
    current_branch = res.stdout.strip()
    print(f"Current branch after init (2): {current_branch}")

    if current_branch == "new-feature":
        print("UNEXPECTED SUCCESS: Switched to new-feature (Did you already fix it?)")
    else:
        print("EXPECTED FAILURE: Did not switch to new-feature")

    # Check if 'new-feature' is available locally?
    res = run_command(["git", "show-ref", "new-feature"], cwd=repo_dir, check=False)
    if res.returncode != 0:
        print("new-feature ref does not exist locally.")

if __name__ == "__main__":
    main()
