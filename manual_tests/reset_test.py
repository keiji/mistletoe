import os
import shutil
import subprocess
import sys
import json

# Adjust path to import test_env
sys.path.append(os.path.dirname(__file__))
from gh_test_env import GhTestEnv

def setup_repo(env, repo_name):
    os.makedirs(env.test_dir, exist_ok=True)
    repo_path = os.path.join(env.test_dir, repo_name)
    if os.path.exists(repo_path):
        shutil.rmtree(repo_path)
    os.makedirs(repo_path)
    subprocess.check_call(["git", "init"], cwd=repo_path)

    # Commit C1
    with open(os.path.join(repo_path, "file1.txt"), "w") as f:
        f.write("v1\n")
    subprocess.check_call(["git", "add", "."], cwd=repo_path)
    subprocess.check_call(["git", "commit", "-m", "C1"], cwd=repo_path)
    c1_hash = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo_path).decode().strip()

    # Commit C2
    with open(os.path.join(repo_path, "file1.txt"), "w") as f:
        f.write("v2\n")
    subprocess.check_call(["git", "add", "."], cwd=repo_path)
    subprocess.check_call(["git", "commit", "-m", "C2"], cwd=repo_path)

    # Uncommitted change M1
    with open(os.path.join(repo_path, "file1.txt"), "w") as f:
        f.write("v3-modified\n")

    return repo_path, c1_hash

def test_reset_keep_changes():
    env = GhTestEnv()
    # env.setup() # GhTestEnv doesn't have setup, it does setup in init?
    # Actually GhTestEnv doesn't seem to have explicit setup in the file I read.
    # It sets up git auth and user in __init__.
    # But it does not create test_dir until create_config_and_graph is called?
    # No, test_dir is defined in __init__.
    # But checking source code: create_config_and_graph does os.makedirs(self.test_dir).
    # So we don't need setup().
    try:
        print("=== Test Reset Keep Changes ===")
        repo_name = "repo-reset-test"
        repo_path, c1_hash = setup_repo(env, repo_name)

        # Create config
        config = {
            "repositories": [
                {
                    "id": repo_name,
                    "url": "https://example.com/dummy.git",
                    "revision": c1_hash
                }
            ]
        }

        # Set remote origin in repo to match config
        subprocess.check_call(["git", "remote", "add", "origin", "https://example.com/dummy.git"], cwd=repo_path)

        config_path = os.path.join(env.test_dir, "mistletoe.json")
        with open(config_path, "w") as f:
            json.dump(config, f)

        # Run mstl reset
        print(f"Resetting {repo_name} to {c1_hash}...")
        # env.run_mstl_cmd takes args list
        env.run_mstl_cmd(["reset", "-f", config_path, "--verbose", "--ignore-stdin"])

        # Verify HEAD is C1
        current_head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo_path).decode().strip()
        if current_head != c1_hash:
            print(f"FAILURE: HEAD is {current_head}, expected {c1_hash}")
            sys.exit(1)

        # Verify file content is still v3-modified (working dir kept)
        with open(os.path.join(repo_path, "file1.txt"), "r") as f:
            content = f.read()

        if "v3-modified" not in content:
            print(f"FAILURE: File content is '{content.strip()}', expected 'v3-modified'")
            sys.exit(1)

        print("SUCCESS: HEAD reset to C1 and changes kept.")

    finally:
        env.cleanup()

if __name__ == "__main__":
    test_reset_keep_changes()
