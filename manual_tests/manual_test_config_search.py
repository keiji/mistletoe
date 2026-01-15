import os
import shutil
import subprocess
import sys
import json

# Add current directory to sys.path to import interactive_runner
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from interactive_runner import InteractiveRunner, print_green, print_red

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
        # Ensure we are on master or rename current branch to master
        subprocess.run(["git", "branch", "-M", "master"], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "push", "origin", "master"], cwd=tmp_clone, check=True, stdout=subprocess.DEVNULL)

        shutil.rmtree(tmp_clone)

        # Now create the actual working directory structure for mstl
        repo_work_dir = os.path.join(base_dir, name)
        subprocess.run(["git", "clone", bare_path, repo_work_dir], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        repos.append({"id": name, "url": bare_path, "path": repo_work_dir})

    return repos

def run_test_logic():
    # Get absolute path to main.go
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)
    main_go_path = os.path.join(project_root, "cmd", "mstl", "main.go")

    test_workspace = os.path.abspath("manual_test_workspace_search")
    if os.path.exists(test_workspace):
        shutil.rmtree(test_workspace)
    os.makedirs(test_workspace)

    try:
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

        cmd_base = ["go", "run", main_go_path]

        # 1. Standard status from root
        # Pass --ignore-stdin just in case
        print_green("1. Verifying standard behavior (running from root)...")
        result = subprocess.run(cmd_base + ["status", "--ignore-stdin"], cwd=test_workspace, capture_output=True, text=True)
        if result.returncode != 0:
            raise Exception(f"Standard status failed: {result.stderr}")
        else:
            print_green("Standard status passed.")

        # 2. Test Parent Search (Automatic Switch)
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

        stdout, stderr = process.communicate()

        if process.returncode != 0:
            print("Stdout:", stdout)
            raise Exception(f"Parent search test failed: {stderr}")

        if "Using that configuration" in stdout or "Using that configuration" in stderr: # Notification might be in stderr or stdout
            # Actually, memory says "notification in English". Stderr or Stdout?
            # Previous run output: "Stdout: No .mstl found ... Using that configuration."
            print_green("Automatic switch notification appeared.")
        else:
            # Let's check exactly what the output was in case my check is too strict
            if "No .mstl found in current directory, but found one in" in stdout:
                 print_green("Automatic switch notification appeared (partial match).")
            else:
                print("Stdout:", stdout)
                raise Exception("Did not see expected notification in output.")

        # 3. Test Validation Failure
        print_green("3. Testing Validation Failure...")
        # Rename repo-c to break structure
        repo_c_path = repos[2]["path"]
        os.rename(repo_c_path, repo_c_path + "_renamed")

        repo_c_renamed = True
        try:
            process = subprocess.Popen(
                cmd_base + ["status", "--ignore-stdin"],
                cwd=repo1_dir,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True
            )
            stdout, stderr = process.communicate()

            if "Using that configuration" in stdout:
                raise Exception("Switched to parent config despite validation failure.")

            # It should fail because it can't find config in CWD, and parent config is invalid so it ignores it.
            # So it effectively reports "config not found".
            if "Configuration file .mstl/config.json not found" in stderr:
                print_green("Validation failure handled correctly.")
            elif "Configuration file" in stderr and "not found" in stderr:
                print_green("Validation failure handled correctly.")
            else:
                 raise Exception(f"Unexpected error message: {stderr}")

        finally:
            if repo_c_renamed and os.path.exists(repo_c_path + "_renamed"):
                os.rename(repo_c_path + "_renamed", repo_c_path)

        print_green("All checks passed inside run_test_logic.")

    finally:
        if os.path.exists(test_workspace):
            shutil.rmtree(test_workspace)

def main():
    runner = InteractiveRunner("Configuration Search Logic Test")
    runner.parse_args()

    description = (
        "This test automatically verifies:\n"
        "1. Standard 'mstl status' behavior from root.\n"
        "2. Parent configuration search (automatic switch verification).\n"
        "3. Parent configuration validation failure handling (should not switch)."
    )

    runner.execute_scenario(
        "Config Search Verification",
        description,
        run_test_logic
    )

if __name__ == "__main__":
    main()
