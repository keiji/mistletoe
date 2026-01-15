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

class UpstreamSafetyTest:
    def __init__(self, runner):
        self.runner = runner
        self.test_dir = None
        self.bin_path = None
        self.repos_dir = None
        self.remotes_dir = None
        self.config_file = None

    def setup(self):
        self.test_dir = tempfile.mkdtemp(prefix="mstl_upstream_test_")
        # Use pre-built binary relative to this script
        script_dir = os.path.dirname(os.path.abspath(__file__))
        self.bin_path = os.path.abspath(os.path.join(script_dir, "../bin/mstl"))
        if sys.platform == "win32":
            self.bin_path += ".exe"

        self.remotes_dir = os.path.join(self.test_dir, "remotes")
        self.repos_dir = os.path.join(self.test_dir, "repos")
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
            print(f"\n[INFO] Temporary directory: {self.test_dir}")
            if self.runner.ask_yes_no("Delete temporary directory?", default="yes"):
                log("Cleaning up temporary directory...")
                try:
                    shutil.rmtree(self.test_dir)
                except Exception as e:
                    print(f"Cleanup failed: {e}")
            else:
                log("Skipped cleanup.")

    def run_cmd(self, cmd, cwd=None, check=True):
        if self.runner.args and self.runner.args.yes and cmd[0] == self.bin_path and "--yes" not in cmd:
            cmd = list(cmd) + ["--yes"]

        # Always add --ignore-stdin for mstl commands in automation to avoid pipe issues
        if cmd[0] == self.bin_path and "--ignore-stdin" not in cmd:
             cmd = list(cmd) + ["--ignore-stdin"]

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
            # print(result.stdout) # Clean output usually desired
            return result
        except subprocess.CalledProcessError as e:
            if check:
                fail(f"Command failed: {' '.join(cmd)}\nStderr: {e.stderr}\nStdout: {e.stdout}")
            return e

    def prepare_environment(self):
        log("Setting up remotes and local repos...")
        os.makedirs(self.remotes_dir, exist_ok=True)
        os.makedirs(self.repos_dir, exist_ok=True)

        # Repo 1
        self.run_cmd(["git", "init", "--bare", os.path.join(self.remotes_dir, "repo1.git")])
        repo1_remote_path = os.path.abspath(os.path.join(self.remotes_dir, "repo1.git"))
        self.run_cmd(["git", "clone", repo1_remote_path, os.path.join(self.repos_dir, "repo1")])

        repo1_local = os.path.join(self.repos_dir, "repo1")
        self.run_cmd(["git", "commit", "--allow-empty", "-m", "init"], cwd=repo1_local)
        self.run_cmd(["git", "push", "origin", "master"], cwd=repo1_local)

        # Repo 2
        self.run_cmd(["git", "init", "--bare", os.path.join(self.remotes_dir, "repo2.git")])
        repo2_remote_path = os.path.abspath(os.path.join(self.remotes_dir, "repo2.git"))
        self.run_cmd(["git", "clone", repo2_remote_path, os.path.join(self.repos_dir, "repo2")])

        repo2_local = os.path.join(self.repos_dir, "repo2")
        self.run_cmd(["git", "commit", "--allow-empty", "-m", "init"], cwd=repo2_local)
        self.run_cmd(["git", "push", "origin", "master"], cwd=repo2_local)

        # Config
        config = {
            "repositories": [
                {"url": repo1_remote_path},
                {"url": repo2_remote_path}
            ]
        }
        with open(self.config_file, "w") as f:
            json.dump(config, f, indent=2)

    def test_mismatch(self):
        log("Scenario 1: Testing mismatch upstream name...")
        repo1_local = os.path.join(self.repos_dir, "repo1")
        self.run_cmd(["git", "checkout", "-b", "feature-mismatch"], cwd=repo1_local)
        self.run_cmd(["git", "branch", "-u", "origin/master"], cwd=repo1_local)

        # Run Status
        log("Running mstl status...")
        self.run_cmd([self.bin_path, "status", "-f", self.config_file], cwd=self.repos_dir)

        # Verify
        res = subprocess.run(["git", "rev-parse", "--abbrev-ref", "@{u}"], cwd=repo1_local, capture_output=True, text=True)
        if res.returncode == 0:
            fail(f"Upstream was NOT unset for mismatched branch. Upstream: {res.stdout.strip()}")
        log("Success: Upstream unset for mismatch.")

    def test_missing_remote(self):
        log("Scenario 2: Testing missing remote branch...")
        repo2_local = os.path.join(self.repos_dir, "repo2")
        self.run_cmd(["git", "checkout", "-b", "feature-gone"], cwd=repo2_local)
        self.run_cmd(["git", "push", "-u", "origin", "feature-gone"], cwd=repo2_local)

        # Delete remote branch via another client (or direct push delete)
        # To avoid repo2 local update during push delete, clone a temp one
        tmp_repo = os.path.join(self.test_dir, "repo2_tmp")
        self.run_cmd(["git", "clone", os.path.join(self.remotes_dir, "repo2.git"), tmp_repo])
        self.run_cmd(["git", "push", "origin", "--delete", "feature-gone"], cwd=tmp_repo)

        # Run Status
        log("Running mstl status...")
        self.run_cmd([self.bin_path, "status", "-f", self.config_file], cwd=self.repos_dir)

        # Verify
        res = subprocess.run(["git", "rev-parse", "--abbrev-ref", "@{u}"], cwd=repo2_local, capture_output=True, text=True)
        if res.returncode == 0:
             fail(f"Upstream was NOT unset for missing remote branch. Upstream: {res.stdout.strip()}")
        log("Success: Upstream unset for missing remote.")

    def run_logic(self):
        self.setup()
        self.prepare_environment()
        self.test_mismatch()
        self.test_missing_remote()
        log("All upstream safety tests passed!")

def main():
    runner = InteractiveRunner("mstl Upstream Safety Test")
    runner.parse_args()
    test = UpstreamSafetyTest(runner)

    description = (
        "This test verifies that 'mstl status' correctly unsets upstream configuration when:\n"
        "1. The local branch name does not match the upstream branch name.\n"
        "2. The upstream remote branch no longer exists."
    )

    runner.execute_scenario(
        "Upstream Validation",
        description,
        test.run_logic
    )

    runner.run_cleanup(test.cleanup)

if __name__ == "__main__":
    main()
