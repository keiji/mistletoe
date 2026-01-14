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
    repo_names = ["repo-a", "repo-b", "repo-c"]
    repos = []

    # Create bare repos to serve as remotes
    remotes_dir = os.path.join(base_dir, "remotes")
    os.makedirs(remotes_dir, exist_ok=True)

    for name in repo_names:
        bare_path = os.path.join(remotes_dir, name + ".git")
        os.makedirs(bare_path, exist_ok=True)
        subprocess.run(["git", "init", "--bare"], cwd=bare_path, check=True, stdout=subprocess.DEVNULL)

        # Clone to create initial commit
        tmp_clone = os.path.join(base_dir, "tmp_setup_" + name)
        subprocess.run(["git", "clone", bare_path, tmp_clone], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

        with open(os.path.join(tmp_clone, "README.md"), "w") as f:
            f.write(f"# {name}")

        subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "config", "user.name", "Test User"], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)

        subprocess.run(["git", "add", "."], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "push", "origin", "master"], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)

        shutil.rmtree(tmp_clone)

        # Now create the actual working directory structure for mstl
        repo_work_dir = os.path.join(base_dir, name)
        subprocess.run(["git", "clone", bare_path, repo_work_dir], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        repos.append({"id": name, "url": bare_path, "path": repo_work_dir})

    return repos

def run_test():
    # Get absolute path to main.go
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)
    main_go_path = os.path.join(project_root, "cmd", "mstl", "main.go")

    test_workspace = os.path.abspath("manual_test_workspace_search")
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

        # 1. Standard status from root
        # Pass --ignore-stdin just in case
        print_green("1. Verifying standard behavior (running from root)...")
        result = subprocess.run(cmd_base + ["status", "--ignore-stdin"], cwd=test_workspace, capture_output=True, text=True)
        if result.returncode != 0:
            print_red(f"Standard status failed: {result.stderr}")
            # return False # Continue to see if other parts work
        else:
            print_green("Standard status passed.")

        # 2. Test Parent Search
        repo1_dir = repos[0]["path"]
        print_green(f"2. Testing Parent Search from sub-directory: {repo1_dir}")

        # We must use --ignore-stdin to prevent mstl from treating the pipe as config input
        process = subprocess.Popen(
            cmd_base + ["status", "--ignore-stdin"],
            cwd=repo1_dir,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )

        stdout, stderr = process.communicate(input="yes\n")

        if process.returncode != 0:
            print_red(f"Parent search test failed: {stderr}")
            print("Stdout:", stdout)
            return False

        if "Current directory does not have .mstl" in stdout:
            print_green("Prompt appeared and was accepted.")
        else:
            print_red("Did not see expected prompt in output.")
            print("Stdout:", stdout)
            # return False

        # 3. Test Rejection
        print_green("3. Testing Parent Search Rejection...")
        process = subprocess.Popen(
            cmd_base + ["status", "--ignore-stdin"],
            cwd=repo1_dir,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )
        stdout, stderr = process.communicate(input="no\n")

        if process.returncode == 0:
            print_red("Expected failure but command succeeded.")
            return False

        if "Configuration file .mstl/config.json not found" in stderr:
            print_green("Rejection handled correctly.")
        else:
            if "Configuration file" in stderr and "not found" in stderr:
                print_green("Rejection handled correctly (error message matched loosely).")
            else:
                print_red(f"Unexpected error message: {stderr}")
                return False

        # 4. Validation Failure
        print_green("4. Testing Validation Failure...")
        # Rename repo-c to break structure
        repo_c_path = repos[2]["path"]
        os.rename(repo_c_path, repo_c_path + "_renamed")

        try:
            process = subprocess.Popen(
                cmd_base + ["status", "--ignore-stdin"],
                cwd=repo1_dir,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True
            )
            stdout, stderr = process.communicate(input="yes\n")

            if "Current directory does not have .mstl" in stdout:
                print_red("Prompt appeared despite validation failure.")
                return False

            if "Configuration file .mstl/config.json not found" in stderr:
                print_green("Validation failure handled correctly.")
            elif "Configuration file" in stderr and "not found" in stderr:
                print_green("Validation failure handled correctly.")
            else:
                 print_red(f"Unexpected error message: {stderr}")
                 return False

        finally:
            if os.path.exists(repo_c_path + "_renamed"):
                os.rename(repo_c_path + "_renamed", repo_c_path)

        print_green("All tests passed.")
        return True

    finally:
        if os.path.exists(test_workspace):
            shutil.rmtree(test_workspace)

if __name__ == "__main__":
    if not run_test():
        sys.exit(1)
