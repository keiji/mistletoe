#!/usr/bin/env python3
import os
import sys
import subprocess

# Ensure manual_tests directory is in python path
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from gh_test_env import GhTestEnv
from interactive_runner import InteractiveRunner, print_green

def main():
    runner = InteractiveRunner("Pull Request Update Test")
    runner.parse_args()

    env = GhTestEnv()

    print_green("[-] Setting up test environment...")
    try:
        env.generate_repo_names(4)
    except Exception as e:
        print_green(f"[FATAL] Setup failed: {e}")
        runner.log("Setup failed", status="FAILED")
        sys.exit(1)

    repo_a = env.repo_names[0]
    repo_d = env.repo_names[3]

    def scenario_logic():
        print_green(f"[-] Creating repositories...")
        env.setup_repos()
        env.create_config_and_graph() # Creates A->B->C

        print_green(f"[-] Initializing...")
        env.run_mstl_cmd(["init", "-f", "mistletoe.json"])

        print_green("[-] Configuring git user...")
        for repo in env.repo_names:
             r_dir = os.path.join(env.test_dir, repo)
             subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
             subprocess.run(["git", "config", "user.name", "Test User"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)

        print_green("[-] Switching to feature/update-test...")
        env.run_mstl_cmd(["switch", "-c", "feature/update-test"])

        print_green("[-] Making commits...")
        for repo in env.repo_names:
            r_dir = os.path.join(env.test_dir, repo)
            with open(os.path.join(r_dir, "test.txt"), "w") as f:
                f.write("test content")
            subprocess.run(["git", "add", "."], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
            subprocess.run(["git", "commit", "-m", "Add test.txt"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)

        print_green("[-] Running 'pr create'...")
        env.run_mstl_cmd(["pr", "create", "-t", "Update Test PR", "-b", "Body", "--dependencies", "dependency-graph.md"])

        print_green("[-] Modifying dependency-graph.md (Adding D --> A)...")
        # Read existing graph
        with open(env.dependency_file, "r") as f:
            content = f.read()

        # Insert D --> A before end of block
        # content looks like:
        # ```mermaid
        # graph TD
        #     A --> B
        #     B --> C
        # ```

        lines = content.splitlines()
        # Find the last line that starts with ```
        end_idx = -1
        for i in range(len(lines)-1, -1, -1):
            if lines[i].strip().startswith("```"):
                end_idx = i
                break

        if end_idx != -1:
             lines.insert(end_idx, f"    {repo_d} --> {repo_a}")
        else:
             lines.append(f"    {repo_d} --> {repo_a}")

        with open(env.dependency_file, "w") as f:
            f.write("\n".join(lines) + "\n")

        print_green("[-] Running 'pr update'...")
        # pr update updates existing PRs
        env.run_mstl_cmd(["pr", "update", "--dependencies", "dependency-graph.md", "--verbose"])

        # Display PR URLs for verification
        print_green(f"[-] Please verify the PR for Repo D ({repo_d}):")
        subprocess.run(["gh", "pr", "list", "--repo", f"{env.user}/{repo_d}", "--head", "feature/update-test"], check=True)

    expected = (
        f"1. PRs created for all 4 repos.\n"
        f"2. Initially D is isolated.\n"
        f"3. After update, the PR for {repo_d} should now show a dependency on {repo_a}.\n"
        f"4. The Mistletoe block in the PR body should be updated."
    )

    runner.execute_scenario(
        "Pull Request Update",
        expected,
        scenario_logic
    )

    runner.run_cleanup(env.cleanup)

if __name__ == "__main__":
    main()
