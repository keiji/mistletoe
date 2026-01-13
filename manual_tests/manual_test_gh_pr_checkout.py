#!/usr/bin/env python3
import os
import sys
import shutil
import subprocess

# Ensure manual_tests directory is in python path
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from gh_test_env import GhTestEnv
from interactive_runner import InteractiveRunner, print_green

def main():
    runner = InteractiveRunner("Pull Request Checkout Test")
    runner.parse_args()

    env = GhTestEnv()

    # 1. Setup Phase (Automated)
    print_green("[-] Setting up test environment (generating names)...")
    try:
        env.generate_repo_names(4)
    except Exception as e:
        print_green(f"[FATAL] Setup failed: {e}")
        runner.log("Setup failed", status="FAILED")
        sys.exit(1)

    repo_a = env.repo_names[0]

    # Define the scenario logic
    def scenario_logic():
        # Setup and Create PRs
        print_green(f"[-] Creating temporary repositories: {', '.join(env.repo_names)}...")
        env.setup_repos()
        env.create_config_and_graph()

        print_green(f"[-] Initializing in {env.test_dir}...")
        env.run_mstl_cmd(["init", "-f", "mistletoe.json", "--verbose"])

        print_green("[-] Configuring dummy git user...")
        for repo in env.repo_names:
             r_dir = os.path.join(env.test_dir, repo)
             subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
             subprocess.run(["git", "config", "user.name", "Test User"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)

        print_green("[-] Switching to feature/checkout-test...")
        env.run_mstl_cmd(["switch", "-c", "feature/checkout-test", "--verbose"])

        print_green("[-] Making commits...")
        for repo in env.repo_names:
            r_dir = os.path.join(env.test_dir, repo)
            with open(os.path.join(r_dir, "test.txt"), "w") as f:
                f.write("test content")
            subprocess.run(["git", "add", "."], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
            subprocess.run(["git", "commit", "-m", "Add test.txt"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)

            # Make a second commit so depth=1 is distinguishable
            with open(os.path.join(r_dir, "test2.txt"), "w") as f:
                f.write("test content 2")
            subprocess.run(["git", "add", "."], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
            subprocess.run(["git", "commit", "-m", "Add test2.txt"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)


        print_green("[-] Running 'pr create' to setup PRs...")
        # Manual input for creation
        env.run_mstl_cmd(["pr", "create", "-t", "Checkout Test PR", "-b", "Body", "--dependencies", "dependency-graph.md", "--verbose"])

        # Retrieve PR URL for Repo A
        print_green(f"[-] retrieving PR URL for {repo_a}...")
        res = subprocess.run(
            ["gh", "pr", "list", "--repo", f"{env.user}/{repo_a}", "--head", "feature/checkout-test", "--json", "url", "--jq", ".[0].url"],
            capture_output=True, text=True, check=True
        )
        pr_url = res.stdout.strip()
        print_green(f"    PR URL: {pr_url}")

        # Checkout Normal
        checkout_dest = os.path.join(env.cwd, "pr_checkout")
        if os.path.exists(checkout_dest):
            shutil.rmtree(checkout_dest)

        print_green(f"[-] Running 'pr checkout' to {checkout_dest}...")
        env.run_mstl_cmd(["pr", "checkout", "-u", pr_url, "--dest", checkout_dest, "--verbose"], cwd=env.cwd)

        # Checkout Shallow
        checkout_dest_shallow = os.path.join(env.cwd, "pr_checkout_shallow")
        if os.path.exists(checkout_dest_shallow):
            shutil.rmtree(checkout_dest_shallow)

        print_green(f"[-] Running 'pr checkout --depth 1' to {checkout_dest_shallow}...")
        env.run_mstl_cmd(["pr", "checkout", "-u", pr_url, "--dest", checkout_dest_shallow, "--depth", "1", "--verbose"], cwd=env.cwd)


        # Verification logic
        print_green(f"[-] Verified checkout destinations")

        # Verify Depth
        for repo in env.repo_names:
             shallow_repo = os.path.join(checkout_dest_shallow, repo)
             if os.path.exists(shallow_repo):
                 # Check commit count
                 res = subprocess.run(["git", "rev-list", "--count", "HEAD"], cwd=shallow_repo, capture_output=True, text=True)
                 count = res.stdout.strip()
                 if count == "1":
                     print_green(f"    Success: {repo} is shallow (depth=1).")
                 else:
                     print_green(f"    WARNING: {repo} has depth {count} (expected 1).")
             else:
                 print_green(f"    WARNING: {repo} missing in shallow checkout.")


    expected = (
        "1. Repositories A, B, C, D are created and PRs are opened.\n"
        "2. 'mstl-gh pr checkout' downloads the state from the PR snapshot to ./pr_checkout.\n"
        "3. 'mstl-gh pr checkout --depth 1' downloads the state to ./pr_checkout_shallow.\n"
        "4. Inside './pr_checkout_shallow', repositories should have only 1 commit history.\n"
    )

    runner.execute_scenario(
        "Pull Request Checkout (Normal & Shallow)",
        expected,
        scenario_logic
    )

    # Cleanup
    def cleanup_with_dest():
        env.cleanup()
        dest_dir = os.path.join(env.cwd, "pr_checkout")
        if os.path.exists(dest_dir):
            shutil.rmtree(dest_dir)
        dest_dir_shallow = os.path.join(env.cwd, "pr_checkout_shallow")
        if os.path.exists(dest_dir_shallow):
            shutil.rmtree(dest_dir_shallow)
            print_green("[-] Deleted ./pr_checkout*")

    runner.run_cleanup(cleanup_with_dest)

if __name__ == "__main__":
    main()
