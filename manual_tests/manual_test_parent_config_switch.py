import os
import shutil
import subprocess
import sys
import json

GREEN = '\033[92m'
RED = '\033[91m'
RESET = '\033[0m'

def print_green(text):
    print(f"{GREEN}{text}{RESET}")

def print_red(text):
    print(f"{RED}{text}{RESET}")

def setup_local_repos(base_dir):
    repo_names = ["repo-a", "repo-b"]
    repos = []

    remotes_dir = os.path.join(base_dir, "remotes")
    os.makedirs(remotes_dir, exist_ok=True)

    for name in repo_names:
        bare_path = os.path.join(remotes_dir, name + ".git")
        os.makedirs(bare_path, exist_ok=True)
        subprocess.run(["git", "init", "--bare"], cwd=bare_path, check=True, stdout=subprocess.DEVNULL)

        tmp_clone = os.path.join(base_dir, "tmp_setup_" + name)
        subprocess.run(["git", "clone", bare_path, tmp_clone], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

        # Configure user to avoid git errors
        subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "config", "user.name", "Test User"], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)

        with open(os.path.join(tmp_clone, "README.md"), "w") as f:
            f.write(f"# {name}")

        subprocess.run(["git", "add", "."], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "branch", "-M", "master"], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "push", "origin", "master"], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)

        shutil.rmtree(tmp_clone)

        repo_work_dir = os.path.join(base_dir, name)
        subprocess.run(["git", "clone", bare_path, repo_work_dir], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        repos.append({"id": name, "url": bare_path, "path": repo_work_dir})

    return repos

def run_test():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)
    main_go_path = os.path.join(project_root, "cmd", "mstl", "main.go")

    test_workspace = os.path.abspath("manual_test_workspace_repro")
    if os.path.exists(test_workspace):
        shutil.rmtree(test_workspace)
    os.makedirs(test_workspace)

    print_green("Setting up local test environment...")
    repos = setup_local_repos(test_workspace)

    # Create .mstl config
    mstl_dir = os.path.join(test_workspace, ".mstl")
    os.makedirs(mstl_dir, exist_ok=True)

    config_repos = []
    for r in repos:
        config_repos.append({
            "id": r["id"],
            "url": r["url"],
            "branch": "master"
        })

    config = {"repositories": config_repos}
    config_path = os.path.join(mstl_dir, "config.json")
    with open(config_path, "w") as f:
        json.dump(config, f, indent=2)

    try:
        cmd_base = ["go", "run", main_go_path]

        # We run from repo-a.
        # Config is in parent.
        # repo-b is a sibling.
        repo1_dir = repos[0]["path"] # repo-a

        print_green(f"Testing Parent Search from sub-directory: {repo1_dir}")

        # We expect mstl to find config in parent, verify repo-b exists in parent, and then correctly status it.
        # If bug exists, it will try to find repo-b inside repo-a and fail to show its status.

        process = subprocess.Popen(
            cmd_base + ["status", "--ignore-stdin"],
            cwd=repo1_dir,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )

        stdout, stderr = process.communicate(input="yes\n")

        print("STDOUT:\n" + stdout)
        print("STDERR:\n" + stderr)

        # Check if repo-b is mentioned in stdout table
        if "repo-b" in stdout and "master" in stdout:
             print_green("repo-b found in status output.")
        else:
             print_red("repo-b NOT found in status output. FAILURE.")
             return False

        return True

    finally:
        if os.path.exists(test_workspace):
            shutil.rmtree(test_workspace)

if __name__ == "__main__":
    if not run_test():
        sys.exit(1)
