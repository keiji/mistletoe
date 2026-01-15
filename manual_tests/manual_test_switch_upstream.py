import os
import sys
import shutil
import tempfile
import subprocess
import json
import atexit

# Add current directory to sys.path to import interactive_runner
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from interactive_runner import InteractiveRunner, print_green, print_red

def log(msg):
    print_green(f"[TEST] {msg}")

def fail(msg):
    print_red(f"[FAIL] {msg}")
    sys.exit(1)

class SwitchUpstreamTest:
    def __init__(self, runner):
        self.root_dir = os.getcwd()
        self.runner = runner
        self.test_dir = None
        self.bin_path = None
        self.repos_dir = None
        self.remote_dir = None
        self.config_file = None

    def setup(self):
        self.test_dir = tempfile.mkdtemp(prefix="mstl_switch_upstream_")
        # Use pre-built binary relative to this script
        script_dir = os.path.dirname(os.path.abspath(__file__))
        self.bin_path = os.path.abspath(os.path.join(script_dir, "../bin/mstl"))
        if sys.platform == "win32":
            self.bin_path += ".exe"

        self.repos_dir = os.path.join(self.test_dir, "repos")
        self.remote_dir = os.path.join(self.test_dir, "remotes")
        self.config_file = os.path.join(self.test_dir, "mstl_config.json")

        atexit.register(self.cleanup)

        # Setup git env
        os.environ["GIT_AUTHOR_NAME"] = "Test User"
        os.environ["GIT_AUTHOR_EMAIL"] = "test@example.com"
        os.environ["GIT_COMMITTER_NAME"] = "Test User"
        os.environ["GIT_COMMITTER_EMAIL"] = "test@example.com"

        log(f"Test Directory: {self.test_dir}")

    def cleanup(self):
        if self.test_dir and os.path.exists(self.test_dir):
            if self.runner.ask_yes_no("Delete temporary directory?", default="yes"):
                log("Cleaning up temporary directory...")
                try:
                    shutil.rmtree(self.test_dir)
                except Exception as e:
                    print(f"Cleanup failed: {e}")
            else:
                log(f"Skipped cleanup. Directory: {self.test_dir}")

    def run_cmd(self, cmd, cwd=None, check=True):
        if self.runner.args and self.runner.args.yes and cmd[0] == self.bin_path and "--yes" not in cmd:
            cmd = list(cmd) + ["--yes"]

        print_green(f"[CMD] {' '.join(cmd)}")
        try:
            result = subprocess.run(
                cmd,
                cwd=cwd,
                check=check,
                text=True,
                capture_output=True,
                env=os.environ
            )
            print(result.stdout)
            if result.stderr:
                print(result.stderr)
            return result
        except subprocess.CalledProcessError as e:
            if check:
                fail(f"Command failed: {' '.join(cmd)}\nStderr: {e.stderr}\nStdout: {e.stdout}")
            return e

    def setup_repo(self):
        log("Setting up remote and local repositories...")
        os.makedirs(self.remote_dir, exist_ok=True)
        os.makedirs(self.repos_dir, exist_ok=True)

        # 1. Init Bare Remote
        remote_path = os.path.join(self.remote_dir, "repo1.git")
        self.run_cmd(["git", "init", "--bare", remote_path])

        # 2. Seed Remote (via temp clone)
        seed_path = os.path.join(self.test_dir, "seed")
        self.run_cmd(["git", "clone", remote_path, seed_path])
        self.run_cmd(["git", "commit", "--allow-empty", "-m", "init"], cwd=seed_path)
        self.run_cmd(["git", "branch", "-M", "master"], cwd=seed_path)
        self.run_cmd(["git", "push", "origin", "master"], cwd=seed_path)

        # 3. Create 'feature/upstream-test' on remote
        self.run_cmd(["git", "checkout", "-b", "feature/upstream-test"], cwd=seed_path)
        self.run_cmd(["git", "push", "origin", "feature/upstream-test"], cwd=seed_path)

        # 4. Clone to local 'repo1' using file:// URL to match config expectations
        repo1_url = "file://" + remote_path
        self.run_cmd(["git", "clone", repo1_url, os.path.join(self.repos_dir, "repo1")])

        # 5. Create Config
        config = {
            "repositories": [
                {"url": repo1_url, "id": "repo1"}
            ]
        }
        with open(self.config_file, "w") as f:
            json.dump(config, f, indent=2)

    def test_upstream_setting(self):
        log("Testing Upstream Setting...")

        # Verify local does not have the branch yet
        repo1_dir = os.path.join(self.repos_dir, "repo1")
        res = self.run_cmd(["git", "branch", "--list", "feature/upstream-test"], cwd=repo1_dir)
        if "feature/upstream-test" in res.stdout:
            fail("Branch feature/upstream-test should not exist locally yet")

        # Run mstl switch -c feature/upstream-test
        # Note: We must use -c (create) because mstl switch (without -c) fails if local branch is missing.
        self.run_cmd([self.bin_path, "switch", "-c", "feature/upstream-test", "-f", self.config_file, "--ignore-stdin", "--verbose"], cwd=self.repos_dir)

        # Verify Upstream
        res_remote = self.run_cmd(["git", "config", "branch.feature/upstream-test.remote"], cwd=repo1_dir, check=False)
        res_merge = self.run_cmd(["git", "config", "branch.feature/upstream-test.merge"], cwd=repo1_dir, check=False)

        if res_remote.stdout.strip() != "origin":
            fail(f"Upstream remote not set to origin. Got: {res_remote.stdout.strip()}")

        if res_merge.stdout.strip() != "refs/heads/feature/upstream-test":
             fail(f"Upstream merge ref not set correctly. Got: {res_merge.stdout.strip()}")

        log("Success: Upstream set correctly.")

    def test_no_upstream(self):
        log("Testing No Upstream (Non-existent remote branch)...")

        self.run_cmd([self.bin_path, "switch", "-c", "feature/no-remote", "-f", self.config_file, "--ignore-stdin", "--verbose"], cwd=self.repos_dir)

        repo1_dir = os.path.join(self.repos_dir, "repo1")
        res_remote = self.run_cmd(["git", "config", "branch.feature/no-remote.remote"], cwd=repo1_dir, check=False)

        if res_remote.returncode == 0 and res_remote.stdout.strip() != "":
             fail(f"Upstream should NOT be set for feature/no-remote. Got: {res_remote.stdout.strip()}")

        log("Success: Upstream not set for local-only branch.")

    def run(self):
        self.setup()
        self.setup_repo()
        self.test_upstream_setting()
        self.test_no_upstream()
        log("All tests passed!")

def main():
    runner = InteractiveRunner("mstl Switch Upstream Test")
    runner.parse_args()
    test = SwitchUpstreamTest(runner)

    description = "Verify that 'mstl switch' automatically sets the upstream branch if a matching remote branch exists."
    runner.execute_scenario("Switch Upstream", description, test.run)
    runner.run_cleanup(test.cleanup)

if __name__ == "__main__":
    main()
