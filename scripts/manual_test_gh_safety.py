import os
import subprocess
import time
import shutil
import json
import sys

def run_command(cmd, cwd=None, env=None):
    """Run a shell command and check for errors."""
    try:
        subprocess.run(cmd, check=True, cwd=cwd, env=env, shell=True, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
    except subprocess.CalledProcessError as e:
        print(f"Error running command: {cmd}")
        print(f"Stderr: {e.stderr.decode()}")
        sys.exit(1)

def main():
    print("Starting Manual Test for 'pr create' Safety Check...")

    # 1. Setup Environment
    test_dir = os.path.abspath("test_safety_env")
    if os.path.exists(test_dir):
        shutil.rmtree(test_dir)
    os.makedirs(test_dir)

    print(f"Test directory: {test_dir}")

    # Build mstl-gh
    print("Building mstl-gh...")
    mstl_gh_bin = os.path.join(test_dir, "mstl-gh")
    if sys.platform == "win32":
        mstl_gh_bin += ".exe"

    subprocess.run(f"go build -o {mstl_gh_bin} ./cmd/mstl-gh", shell=True, check=True)

    # Create Bare Remote Repo
    remote_dir = os.path.join(test_dir, "remote-a.git")
    os.makedirs(remote_dir)
    run_command("git init --bare", cwd=remote_dir)

    # Create Local Repo A
    repo_a_dir = os.path.join(test_dir, "repo-a")
    os.makedirs(repo_a_dir)
    run_command("git init", cwd=repo_a_dir)
    run_command("git config user.email 'test@example.com'", cwd=repo_a_dir)
    run_command("git config user.name 'Test User'", cwd=repo_a_dir)

    repo_url = "https://github.com/example/repo-a"
    run_command(f"git remote add origin {repo_url}", cwd=repo_a_dir)

    remote_url_file = "file://" + remote_dir.replace("\\", "/")

    # Use Local Config for insteadOf
    run_command(f"git config url.\"{remote_url_file}\".insteadOf \"{repo_url}\"", cwd=repo_a_dir)

    # Initial content and push to remote
    with open(os.path.join(repo_a_dir, "file.txt"), "w") as f:
        f.write("content 1")
    run_command("git add .", cwd=repo_a_dir)
    run_command("git branch -M main", cwd=repo_a_dir)
    run_command("git commit -m 'commit 1'", cwd=repo_a_dir)

    # Push initial state
    run_command("git push -u origin main", cwd=repo_a_dir)

    # Make Commit 2 (So we are Ahead)
    with open(os.path.join(repo_a_dir, "file.txt"), "a") as f:
        f.write("\ncontent 2")
    run_command("git add .", cwd=repo_a_dir)
    run_command("git commit -m 'commit 2'", cwd=repo_a_dir)

    # Create fake gh
    fake_gh = os.path.join(test_dir, "gh")
    gh_script_content = """#!/usr/bin/env python3
import sys
import json

args = sys.argv[1:]

if "auth" in args and "status" in args:
    print("Logged in to github.com as testuser")
    sys.exit(0)

if "--version" in args:
    print("gh version 2.0.0")
    sys.exit(0)

if "pr" in args and "list" in args:
    print("[]")
    sys.exit(0)

if "repo" in args and "view" in args:
    if "-q" in args:
        idx = args.index("-q")
        if idx + 1 < len(args):
            query = args[idx+1]
            if ".viewerPermission" in query:
                print("WRITE")
                sys.exit(0)
    print(json.dumps({"viewerPermission": "WRITE"}))
    sys.exit(0)

print("")
sys.exit(0)
"""
    with open(fake_gh, "w") as f:
        f.write(gh_script_content)
    run_command(f"chmod +x {fake_gh}")

    env = os.environ.copy()
    env["PATH"] = test_dir + os.pathsep + env["PATH"]
    env["HOME"] = test_dir

    # Create config
    config = {
        "repositories": [
            {
                "id": "repo-a",
                "url": repo_url,
                "branch": "main"
            }
        ]
    }

    config_path = os.path.join(test_dir, "mistletoe.json")
    with open(config_path, "w") as f:
        json.dump(config, f)

    # Run mstl-gh pr create
    print("Running mstl-gh pr create...")

    log_file_path = os.path.join(test_dir, "output.log")
    log_file = open(log_file_path, "w")

    proc = subprocess.Popen(
        [mstl_gh_bin, "pr", "create", "-f", config_path, "--title", "Test PR", "--body", "Body"],
        stdin=subprocess.PIPE,
        stdout=log_file,
        stderr=log_file,
        cwd=test_dir,
        env=env,
        text=True
    )

    # Monitor Log for Prompt
    print("Waiting for prompt...")
    found_prompt = False
    for i in range(20): # Wait up to 20s
        time.sleep(1)
        log_file.flush()
        with open(log_file_path, "r") as f:
            content = f.read()
            if "Proceed with Push" in content:
                found_prompt = True
                break

    if not found_prompt:
        print("Timeout waiting for prompt.")
        with open(log_file_path, "r") as f:
            print(f.read())
        proc.kill()
        sys.exit(1)

    # Inject Change! (Commit 3)
    print("Injecting change to repo-a...")
    with open(os.path.join(repo_a_dir, "file2.txt"), "w") as f:
        f.write("content 3")
    run_command("git add .", cwd=repo_a_dir)
    run_command("git commit -m 'commit 3'", cwd=repo_a_dir)

    # Send "yes"
    print("Sending 'yes' to mstl-gh...")
    proc.stdin.write("yes\n")
    proc.stdin.flush()

    proc.wait()
    log_file.close()

    with open(log_file_path, "r") as f:
        out = f.read()

    print("--- OUTPUT ---")
    print(out)

    # Check for specific error message
    expected_error = "has changed since status collection"
    if expected_error in out:
        print("\nSUCCESS: Safety check triggered correctly.")
        try:
            shutil.rmtree(test_dir)
        except:
            pass
        sys.exit(0)
    else:
        print("\nFAILURE: Safety check did NOT trigger.")
        sys.exit(1)

if __name__ == "__main__":
    main()
