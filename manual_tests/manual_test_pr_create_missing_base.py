#!/usr/bin/env python3
import os
import sys
# Ensure manual_tests directory is in python path
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from gh_test_env import GhTestEnv
from interactive_runner import InteractiveRunner, print_green
import subprocess
import json

def main():
    runner = InteractiveRunner("PR Create Missing Base Branch Test")
    runner.parse_args()

    env = GhTestEnv()
    if runner.args and runner.args.yes:
        env.auto_yes = True

    # 1. Setup Phase
    print_green("[-] Setting up test environment...")
    try:
        env.generate_repo_names(2)
    except Exception as e:
        print_green(f"[FATAL] Setup failed: {e}")
        runner.log("Setup failed", status="FAILED")
        sys.exit(1)

    repo_valid = env.repo_names[0]
    repo_missing_base = env.repo_names[1]

    def scenario_logic():
        # Create Repositories
        print_green(f"[-] Creating temporary repositories...")
        env.setup_repos()

        # Override config generation to specify a non-existent base branch for repo_missing_base
        config_data = {
            "version": "1.0",
            "repositories": [
                {
                    "id": repo_valid,
                    "url": f"https://github.com/{env.user}/{repo_valid}.git",
                    "base-branch": "main"
                },
                {
                    "id": repo_missing_base,
                    "url": f"https://github.com/{env.user}/{repo_missing_base}.git",
                    "base-branch": "non-existent-branch" # This branch doesn't exist on remote
                }
            ]
        }

        with open(os.path.join(env.test_dir, "mistletoe.json"), "w") as f:
            json.dump(config_data, f, indent=2)

        # Initialize
        print_green(f"[-] Initializing in {env.test_dir}...")
        env.run_mstl_cmd(["init", "-f", "mistletoe.json", "--ignore-stdin", "--verbose"])

        # Configure dummy git user
        for repo in env.repo_names:
             r_dir = os.path.join(env.test_dir, repo)
             subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
             subprocess.run(["git", "config", "user.name", "Test User"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)

        # Switch branch
        print_green("[-] Switching to feature/missing-base-test...")
        env.run_mstl_cmd(["switch", "-c", "feature/missing-base-test", "--verbose"])

        # Make changes
        print_green("[-] Making commits...")
        for repo in env.repo_names:
            r_dir = os.path.join(env.test_dir, repo)
            with open(os.path.join(r_dir, "test.txt"), "w") as f:
                f.write("test content")
            subprocess.run(["git", "add", "."], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
            subprocess.run(["git", "commit", "-m", "Add test.txt"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)

        print_green("[-] Running 'pr create'...")

        # We expect this command to SUCCEED (exit 0) but print a warning/skip message for repo_missing_base
        cmd = ["pr", "create", "-t", "Test PR", "-b", "Test Body", "--yes", "--verbose"]

        result_code = 0
        output = ""
        try:
            # We capture output to verify the behavior
            process = subprocess.run(
                [env.mstl_bin] + cmd,
                cwd=env.test_dir,
                check=False, # We want to check code manually
                capture_output=True,
                text=True,
                env=os.environ.copy()
            )
            result_code = process.returncode
            output = process.stdout + process.stderr
            print(output)

            if result_code != 0:
                print_green(f"[FAIL] 'pr create' failed with exit code {result_code}")
                # For reproduction, we might expect failure, but for the test itself, we want to know if it passed the criteria.
                # In reproduction mode (before fix), this should fail or error out loudly.
            else:
                print_green(f"[SUCCESS] 'pr create' finished with exit code 0")

        except Exception as e:
             print_green(f"[FAIL] Exception running command: {e}")
             raise

        # Verification Logic
        if result_code == 0:
            if f"base branch 'non-existent-branch' does not exist on remote" in output and "skipping" in output.lower():
                 print_green("[PASS] Logic confirmed: Repository was skipped due to missing base branch.")
            else:
                 print_green("[WARN] Command succeeded but did not find expected skip message. Check output.")
        else:
            if f"base branch 'non-existent-branch' does not exist on remote" in output:
                 print_green("[FAIL-EXPECTED] Logic confirmed: Repository caused failure due to missing base branch (Current Behavior).")
            else:
                 print_green(f"[FAIL] Command failed for unexpected reason.")

    runner.execute_scenario(
        "Verify PR Create Skips Missing Base Branch",
        "The tool should skip the repo with missing base branch and process the valid one.",
        scenario_logic
    )

    runner.run_cleanup(env.cleanup)

if __name__ == "__main__":
    main()
