#!/usr/bin/env python3
import os
import sys
# Ensure scripts directory is in python path
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from gh_test_env import GhTestEnv
from interactive_runner import InteractiveRunner

def main():
    runner = InteractiveRunner("Multi-Repo Pull Request Creation Test")
    runner.parse_args()

    env = GhTestEnv()

    # 1. Setup Phase (Automated)
    print("[-] Setting up test environment (Creating repos, config, building binary)...")
    try:
        env.generate_repo_names(3)
        # Check for existing repos and warn (simple check, full check is in manual_test_gh.py but we do a basic one here)
        # Assuming generate_repo_names handles uniqueness.

        env.build_mstl_gh()
        env.setup_repos()
        env.create_config_and_graph()
    except Exception as e:
        print(f"[FATAL] Setup failed: {e}")
        runner.log("Setup failed", status="FAILED")
        sys.exit(1)

    repo_a = env.repo_names[0]
    repo_b = env.repo_names[1]

    # Define the scenario logic
    def scenario_logic():
        # Initialize
        print(f"[-] Initializing in {env.test_dir}...")
        env.run_mstl_cmd(["init", "-f", "mistletoe.json"])

        # Switch branch
        print("[-] Switching to feature/interactive-test...")
        env.run_mstl_cmd(["switch", "-c", "feature/interactive-test"])

        # Make changes
        print("[-] Making commits to repositories...")
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

        print("[-] Running 'pr create'...")
        print("    (Please type 'yes' when prompted by the tool to create PRs)")

        # Execute pr create interactively
        # We allow stdin to pass through to the user
        cmd = [env.mstl_bin, "pr", "create", "-t", "Interactive Test PR", "-b", "Testing interactive script", "-d", "dependencies.mmd"]
        import subprocess
        subprocess.run(cmd, cwd=env.test_dir)

    # Expected result text
    expected = (
        f"This test will create Pull Requests in:\n"
        f"  - {repo_a}\n"
        f"  - {repo_b}\n"
        f"  ... and others.\n\n"
        f"The Pull Request bodies should contain a dependency graph where:\n"
        f"  - {repo_a} depends on {repo_b}\n"
        f"  - {repo_b} depends on the third repo."
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
