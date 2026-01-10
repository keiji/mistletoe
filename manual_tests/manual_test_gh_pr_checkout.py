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
    print_green("[-] Setting up test environment (Building binary, generating names)...")
    try:
        env.generate_repo_names(4)
        env.build_mstl_gh()
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
        env.run_mstl_cmd(["init", "-f", "mistletoe.json"])

        print_green("[-] Configuring dummy git user...")
        for repo in env.repo_names:
             r_dir = os.path.join(env.test_dir, repo)
             subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
             subprocess.run(["git", "config", "user.name", "Test User"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)

        print_green("[-] Switching to feature/checkout-test...")
        env.run_mstl_cmd(["switch", "-c", "feature/checkout-test"])

        print_green("[-] Making commits...")
        for repo in env.repo_names:
            r_dir = os.path.join(env.test_dir, repo)
            with open(os.path.join(r_dir, "test.txt"), "w") as f:
                f.write("test content")
            subprocess.run(["git", "add", "."], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
            subprocess.run(["git", "commit", "-m", "Add test.txt"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)

        print_green("[-] Running 'pr create' to setup PRs...")
        # Automate yes input for creation
        env.run_mstl_cmd(["pr", "create", "-t", "Checkout Test PR", "-b", "Body", "--dependencies", "dependencies.md", "--ignore-stdin"], input_str="yes\n")

        # Retrieve PR URL for Repo A
        print_green(f"[-] retrieving PR URL for {repo_a}...")
        res = subprocess.run(
            ["gh", "pr", "list", "--repo", f"{env.user}/{repo_a}", "--head", "feature/checkout-test", "--json", "url", "--jq", ".[0].url"],
            capture_output=True, text=True, check=True
        )
        pr_url = res.stdout.strip()
        print_green(f"    PR URL: {pr_url}")

        # Checkout
        checkout_dest = os.path.join(env.cwd, "pr_checkout")
        if os.path.exists(checkout_dest):
            shutil.rmtree(checkout_dest)

        print_green(f"[-] Running 'pr checkout' to {checkout_dest}...")
        # We run this from env.cwd (root) so --dest ./pr_checkout works as expected
        env.run_mstl_cmd(["pr", "checkout", "-u", pr_url, "--dest", checkout_dest, "--verbose"], cwd=env.cwd)

        # Verification logic could be added here, but InteractiveRunner relies on user saying "Yes"
        # We can print what to check
        print_green(f"[-] Verified checkout destination: {checkout_dest}")
        if os.path.exists(checkout_dest) and len(os.listdir(checkout_dest)) >= 4:
             print_green(f"    Success: Directory exists and contains files.")
        else:
             print_green(f"    WARNING: Directory seems empty or missing!")

    expected = (
        "1. Repositories A, B, C, D are created and PRs are opened.\n"
        "2. 'mstl-gh pr checkout' downloads the state from the PR snapshot.\n"
        "3. A new directory './pr_checkout' is created.\n"
        "4. Inside './pr_checkout', all 4 repositories exist and are checked out to the PR branch/commit.\n"
        "5. The .mstl/dependencies.md in the checked out dir should match the original."
    )

    runner.execute_scenario(
        "Pull Request Checkout",
        expected,
        scenario_logic
    )

    # Cleanup
    def cleanup_with_dest():
        env.cleanup()
        dest_dir = os.path.join(env.cwd, "pr_checkout")
        if os.path.exists(dest_dir):
            shutil.rmtree(dest_dir)
            print_green("[-] Deleted ./pr_checkout")

    runner.run_cleanup(cleanup_with_dest)

if __name__ == "__main__":
    main()
