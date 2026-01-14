#!/usr/bin/env python3
import os
import sys
# Ensure manual_tests directory is in python path
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from gh_test_env import GhTestEnv
from interactive_runner import InteractiveRunner, print_green

def main():
    runner = InteractiveRunner("Multi-Repo Pull Request Creation Test")
    runner.parse_args()

    env = GhTestEnv()
    if runner.args and runner.args.yes:
        env.auto_yes = True

    # 1. Setup Phase (Automated)
    print_green("[-] Setting up test environment (generating names)...")
    try:
        env.generate_repo_names(4)
    except Exception as e:
        print_green(f"[FATAL] Setup failed: {e}")
        runner.log("Setup failed", status="FAILED")
        sys.exit(1)

    repo_a = env.repo_names[0]
    repo_b = env.repo_names[1]

    # Define the scenario logic
    def scenario_logic():
        # Create Repositories (Deferred until user confirmation)
        print_green(f"[-] Creating temporary repositories: {', '.join(env.repo_names)}...")
        env.setup_repos()
        env.create_config_and_graph()

        # Initialize
        print_green(f"[-] Initializing in {env.test_dir}...")
        env.run_mstl_cmd(["init", "-f", "mistletoe.json", "--verbose"])

        # Configure git user for the cloned repos (required for subsequent commits)
        print_green("[-] Configuring dummy git user for cloned repositories...")
        import subprocess
        for repo in env.repo_names:
             r_dir = os.path.join(env.test_dir, repo)
             subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
             subprocess.run(["git", "config", "user.name", "Test User"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)

        # Switch branch
        print_green("[-] Switching to feature/interactive-test...")
        env.run_mstl_cmd(["switch", "-c", "feature/interactive-test", "--verbose"])

        # Make changes
        print_green("[-] Making commits to repositories...")
        for repo in env.repo_names:
            r_dir = os.path.join(env.test_dir, repo)
            with open(os.path.join(r_dir, "test.txt"), "w") as f:
                f.write("test content")
            # We assume git is in path
            import subprocess
            subprocess.run(["git", "add", "."], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
            subprocess.run(["git", "commit", "-m", "Add test.txt"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
            # Push logic needs input "yes" because mstl push prompts
            # But wait, pr create also prompts.
            # We will run pr create directly, which handles push if ahead.

        print_green("[-] Running 'pr create'...")
        print_green("    (Please type 'yes' when prompted by the tool to create PRs)")

        # Execute pr create interactively
        # We allow stdin to pass through to the user
        cmd = ["pr", "create", "-t", "Interactive Test PR", "-b", "Testing interactive script", "--dependencies", "dependency-graph.md", "--verbose"]
        env.run_mstl_cmd(cmd, cwd=env.test_dir)

    # Expected result text
    expected = (
        f"This test will create Pull Requests in:\n"
        f"  - {repo_a}\n"
        f"  - {repo_b}\n"
        f"  ... and others (Total 4).\n\n"
        f"The Pull Request bodies should contain a dependency graph where:\n"
        f"  - {repo_a} depends on {repo_b}\n"
        f"  - {repo_b} depends on the third repo.\n"
        f"  - The fourth repo is isolated (no dependencies)."
    )

    # Run the interactive scenario
    runner.execute_scenario(
        "Create Pull Requests",
        expected,
        scenario_logic
    )

    # Cleanup
    runner.run_cleanup(env.cleanup)

if __name__ == "__main__":
    main()
