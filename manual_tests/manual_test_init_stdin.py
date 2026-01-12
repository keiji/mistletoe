#!/usr/bin/env python3
"""
Test script for `mstl init` using standard input for configuration.
"""
import os
import sys
import subprocess
import shutil
import json
import time

# Ensure we can find the modules in the current directory
sys.path.append(os.getcwd())

def print_green(text):
    print(f"\033[32m{text}\033[0m")

def print_red(text):
    print(f"\033[31m{text}\033[0m")

def run_command(cmd, cwd=None, stdin_input=None):
    """Runs a shell command."""
    try:
        # Note: If cmd contains pipes, use shell=True, but here we run list of args
        # and pass stdin via input parameter.
        result = subprocess.run(
            cmd,
            cwd=cwd,
            input=stdin_input,
            text=True,
            capture_output=True,
            check=True
        )
        # Verify stderr output for verbose logging if needed, but return stdout
        # mstl commands might output to stderr for logs.
        if result.stderr:
            # Check if it's just info logs or actual errors.
            # mstl --verbose outputs to stderr.
            pass
        return result.stdout.strip(), result.stderr
    except subprocess.CalledProcessError as e:
        print_red(f"Command failed: {' '.join(cmd)}")
        print_red(f"STDOUT: {e.stdout}")
        print_red(f"STDERR: {e.stderr}")
        raise

def main():
    # Setup directories
    base_dir = os.path.abspath("manual_tests_tmp_init_stdin")
    if os.path.exists(base_dir):
        shutil.rmtree(base_dir)
    os.makedirs(base_dir)

    remote_dir = os.path.join(base_dir, "remote")
    os.makedirs(remote_dir)

    dest_dir = os.path.join(base_dir, "dest")
    # Dest dir will be created by init

    try:
        # 1. Create a dummy bare repository
        repo_name = "repo-stdin"
        repo_path = os.path.join(remote_dir, repo_name)
        os.makedirs(repo_path)
        run_command(["git", "init", "--bare"], cwd=repo_path)

        # 2. Create a seed repo to push content to the bare repo
        seed_path = os.path.join(base_dir, "seed")
        os.makedirs(seed_path)
        run_command(["git", "init"], cwd=seed_path)
        run_command(["git", "config", "user.email", "test@example.com"], cwd=seed_path)
        run_command(["git", "config", "user.name", "Test User"], cwd=seed_path)

        readme_path = os.path.join(seed_path, "README.md")
        with open(readme_path, "w") as f:
            f.write("# Stdin Test Repo")

        run_command(["git", "add", "README.md"], cwd=seed_path)
        run_command(["git", "commit", "-m", "Initial commit"], cwd=seed_path)
        run_command(["git", "remote", "add", "origin", repo_path], cwd=seed_path)
        run_command(["git", "push", "origin", "master"], cwd=seed_path)

        # Update HEAD in bare repo
        run_command(["git", "symbolic-ref", "HEAD", "refs/heads/master"], cwd=repo_path)

        # 3. Prepare Configuration JSON
        # Note: Use file:// URL for local test
        repo_url = f"file://{repo_path}"
        config = {
            "repositories": [
                {
                    "id": repo_name,
                    "url": repo_url,
                    "branch": "master"
                }
            ]
        }
        config_json = json.dumps(config)

        # 4. Run mstl init with piped input
        # Assuming 'mstl' is in the path or we use the build command.

        mstl_bin = os.path.abspath("mstl")
        print_green(f"Building mstl...")
        run_command(["go", "build", "-o", mstl_bin, "./cmd/mstl"])

        print_green(f"Running mstl init with stdin input...")

        # Note: --dest must be specified or it defaults to current dir.
        # We use a fresh dest dir.

        cmd = [mstl_bin, "init", "--dest", dest_dir]

        stdout, stderr = run_command(cmd, stdin_input=config_json)
        print(stdout)
        if stderr:
            print(stderr)

        # 5. Verify Result
        target_repo_path = os.path.join(dest_dir, repo_name)
        target_readme = os.path.join(target_repo_path, "README.md")

        if os.path.exists(target_readme):
            print_green(f"Success! {target_readme} exists.")
        else:
            print_red(f"Failure! {target_readme} not found.")
            sys.exit(1)

        # Check .mstl/config.json
        mstl_config_path = os.path.join(dest_dir, ".mstl", "config.json")
        if os.path.exists(mstl_config_path):
            print_green(f"Success! {mstl_config_path} exists.")
        else:
            print_red(f"Failure! {mstl_config_path} not found.")
            sys.exit(1)

        print_green("Test Passed.")

    finally:
        # Cleanup
        if os.path.exists(base_dir):
            shutil.rmtree(base_dir)
        if os.path.exists("mstl"):
            os.remove("mstl")

if __name__ == "__main__":
    main()
