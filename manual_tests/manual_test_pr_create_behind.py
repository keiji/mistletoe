#!/usr/bin/env python3
import os
import sys
# Ensure manual_tests directory is in python path
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from gh_test_env import GhTestEnv
from interactive_runner import InteractiveRunner, print_green

def main():
    runner = InteractiveRunner("Pull Request Create - Behind/Diverged Status Test")
    runner.parse_args()

    env = GhTestEnv()
    if runner.args.yes:
        env.auto_yes = True

    # 1. Setup Phase
    print_green("[-] Setting up test environment (generating names)...")
    try:
        env.generate_repo_names(1)
    except Exception as e:
        print_green(f"[FATAL] Setup failed: {e}")
        runner.log("Setup failed", status="FAILED")
        sys.exit(1)

    repo_a = env.repo_names[0]

    def scenario_logic():
        # Create Repositories
        print_green(f"[-] Creating temporary repository: {repo_a}...")
        env.setup_repos()
        env.create_config_and_graph()

        # Initialize
        print_green(f"[-] Initializing in {env.test_dir}...")
        env.run_mstl_cmd(["init", "-f", "mistletoe.json", "--ignore-stdin"])

        # Configure git user
        import subprocess
        r_a = os.path.join(env.test_dir, repo_a)
        subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=r_a, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "config", "user.name", "Test User"], cwd=r_a, check=True, stdout=subprocess.DEVNULL)

        # --------------------------------------------------------------------------------
        # Prepare "Behind" Scenario
        # --------------------------------------------------------------------------------
        print_green("[-] Preparing diverged/behind state on feature branch...")

        branch_name = "feature/behind-test"

        # 1. Create feature branch and push it
        subprocess.run(["git", "checkout", "-b", branch_name], cwd=r_a, check=True, stdout=subprocess.DEVNULL)
        with open(os.path.join(r_a, "initial.txt"), "w") as f: f.write("initial")
        subprocess.run(["git", "add", "."], cwd=r_a, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "commit", "-m", "Initial feature commit"], cwd=r_a, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "push", "-u", "origin", branch_name], cwd=r_a, check=True, stdout=subprocess.DEVNULL)

        # 2. Simulate Remote Activity (Someone else pushed to the same branch)
        # We do this by cloning to another directory, committing, and pushing.
        temp_clone_dir = os.path.join(env.test_dir, f"{repo_a}_temp_clone")

        # Determine remote URL:
        # If using MOCK_GH_USER, repo is at root cwd.
        # If real, it's a github url.
        # We can read 'git remote get-url origin' from the initialized repo to be safe.
        res = subprocess.run(["git", "remote", "get-url", "origin"], cwd=r_a, capture_output=True, text=True, check=True)
        remote_url = res.stdout.strip()

        subprocess.run(["git", "clone", remote_url, temp_clone_dir], check=True, stdout=subprocess.DEVNULL)

        # Checkout the branch in temp clone
        subprocess.run(["git", "checkout", branch_name], cwd=temp_clone_dir, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "config", "user.email", "other@example.com"], cwd=temp_clone_dir, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "config", "user.name", "Other User"], cwd=temp_clone_dir, check=True, stdout=subprocess.DEVNULL)

        # Add a commit and push
        with open(os.path.join(temp_clone_dir, "remote_change.txt"), "w") as f: f.write("remote change")
        subprocess.run(["git", "add", "."], cwd=temp_clone_dir, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "commit", "-m", "Remote change"], cwd=temp_clone_dir, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "push"], cwd=temp_clone_dir, check=True, stdout=subprocess.DEVNULL)

        # 3. Create Local Divergence (Optional, but "Behind" is sufficient to trigger error)
        # Let's make it strictly "Behind" first (fast-forward possible but we are behind).
        # Actually, let's make it Diverged (Ahead and Behind) just to be sure.
        # Add local commit
        with open(os.path.join(r_a, "local_change.txt"), "w") as f: f.write("local change")
        subprocess.run(["git", "add", "."], cwd=r_a, check=True, stdout=subprocess.DEVNULL)
        subprocess.run(["git", "commit", "-m", "Local change"], cwd=r_a, check=True, stdout=subprocess.DEVNULL)

        # Now Local is Ahead 1, Behind 1.
        # Important: We must FETCH in the local repo so it knows it is behind.
        # 'mstl pr create' does NOT fetch by default if we passed noFetch=true to CollectStatus?
        # Let's check pr_create.go.
        #   rows := CollectStatus(config, jobs, opts.GitPath, verbose, true)
        #   The last arg is `noFetch`. It is passed as `true`.
        #   This means 'pr create' relies on the user having fetched?
        #   Or does it rely on previous 'sync'?
        #   If noFetch is true, CollectStatus won't see the new remote commit unless we fetch manually here.

        print_green("[-] Fetching origin in local repo to ensure it sees the remote changes...")
        subprocess.run(["git", "fetch", "origin"], cwd=r_a, check=True, stdout=subprocess.DEVNULL)

        # --------------------------------------------------------------------------------
        # Run Test
        # --------------------------------------------------------------------------------
        print_green("[-] Running 'pr create'...")
        print_green("    Verify the command FAILS with an error about the repository being behind/pull needed.")

        cmd = [env.mstl_bin, "pr", "create", "-f", "mistletoe.json", "--verbose", "--ignore-stdin"]
        # Supply title/body to avoid editor if it mistakenly proceeds
        cmd.extend(["--title", "Should Fail", "--body", "Should Fail"])

        if runner.args.yes:
            cmd.append("--yes")

        # We expect this command to FAIL (return non-zero exit code)
        result = subprocess.run(cmd, cwd=env.test_dir, capture_output=True, text=True)

        print(result.stdout)
        print(result.stderr)

        if result.returncode != 0 and ("behind remote" in result.stdout or "behind remote" in result.stderr or "require a pull" in result.stderr or "require a pull" in result.stdout):
             print_green("[SUCCESS] Command failed as expected with 'behind' error.")
        else:
             print_green("[FAILURE] Command did not fail as expected or gave wrong error.")
             if result.returncode == 0:
                 print_green("Command succeeded but should have failed.")
             sys.exit(1)

    # Expected result text
    expected = (
        f"The tool should FAIL and display:\n"
        f"  error: the following repositories are behind remote and require a pull:\n"
        f"   - {repo_a}\n"
    )

    # Run the interactive scenario
    runner.execute_scenario(
        "Verify Behind Status Check",
        expected,
        scenario_logic
    )

    # Cleanup
    runner.run_cleanup(env.cleanup)

if __name__ == "__main__":
    main()
