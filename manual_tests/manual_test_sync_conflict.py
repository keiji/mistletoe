import os
import sys
import shutil
import tempfile
import subprocess
import json
import atexit

# Add current directory to sys.path to import interactive_runner
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from interactive_runner import print_green

# Colors
GREEN = '\033[0;32m'
RED = '\033[0;31m'
YELLOW = '\033[1;33m'
NC = '\033[0m'

def log(msg):
    print_green(f"[TEST] {msg}")

def fail(msg):
    print(f"{RED}[FAIL]{NC} {msg}")
    sys.exit(1)

class MstlManualTestSyncConflict:
    def __init__(self):
        self.root_dir = os.getcwd()
        self.test_dir = tempfile.mkdtemp(prefix="mstl_test_sync_")
        self.bin_path = os.path.join(self.test_dir, "bin", "mstl")
        self.repos_dir = os.path.join(self.test_dir, "repos")
        self.remote_dir = os.path.join(self.test_dir, "remotes")
        self.config_file = os.path.join(self.test_dir, "mstl_config.json")
        self.seed_dir = os.path.join(self.test_dir, "seed")

        atexit.register(self.cleanup)

        os.environ["GIT_AUTHOR_NAME"] = "Test User"
        os.environ["GIT_AUTHOR_EMAIL"] = "test@example.com"
        os.environ["GIT_COMMITTER_NAME"] = "Test User"
        os.environ["GIT_COMMITTER_EMAIL"] = "test@example.com"
        # Disable pager
        os.environ["GIT_PAGER"] = "cat"

        log(f"Test Directory: {self.test_dir}")

    def cleanup(self):
        if os.path.exists(self.test_dir):
            log("Cleaning up temporary directory...")
            shutil.rmtree(self.test_dir)

    def run_cmd(self, cmd, cwd=None, check=True, input_str=None):
        try:
            result = subprocess.run(
                cmd,
                cwd=cwd,
                check=check,
                text=True,
                capture_output=True,
                input=input_str,
                env=os.environ
            )
            return result
        except subprocess.CalledProcessError as e:
            if check:
                fail(f"Command failed: {' '.join(cmd)}\nStderr: {e.stderr}\nStdout: {e.stdout}")
            return e

    def build_mstl(self):
        log("Building mstl...")
        os.makedirs(os.path.dirname(self.bin_path), exist_ok=True)
        self.run_cmd(["go", "build", "-o", self.bin_path, "./cmd/mstl"], cwd=self.root_dir)

    def setup_repo(self):
        log("Setting up remote repository...")
        os.makedirs(self.remote_dir, exist_ok=True)
        self.run_cmd(["git", "init", "--bare", os.path.join(self.remote_dir, "repo1.git")])

        log("Seeding remote...")
        os.makedirs(self.seed_dir, exist_ok=True)

        # Clone to seed dir
        self.run_cmd(["git", "clone", os.path.join(self.remote_dir, "repo1.git"), os.path.join(self.seed_dir, "repo1")])
        repo1_seed = os.path.join(self.seed_dir, "repo1")
        self.run_cmd(["git", "checkout", "-b", "main"], cwd=repo1_seed, check=False)

        with open(os.path.join(repo1_seed, "README.md"), "w") as f:
            f.write("# Repo 1\nLine 1\nLine 2\n")

        self.run_cmd(["git", "add", "README.md"], cwd=repo1_seed)
        self.run_cmd(["git", "commit", "-m", "Initial commit"], cwd=repo1_seed)
        self.run_cmd(["git", "push", "origin", "main"], cwd=repo1_seed)

        # Ensure HEAD matches
        self.run_cmd(["git", "--git-dir", os.path.join(self.remote_dir, "repo1.git"), "symbolic-ref", "HEAD", "refs/heads/main"])

    def create_config(self):
        config = {
            "repositories": [
                {"url": os.path.join(self.remote_dir, "repo1.git"), "branch": "main"}
            ]
        }
        with open(self.config_file, "w") as f:
            json.dump(config, f, indent=2)

    def run_test(self):
        self.build_mstl()
        self.setup_repo()
        self.create_config()

        log("Initializing local environment...")
        os.makedirs(self.repos_dir, exist_ok=True)
        # Use --ignore-stdin to prevent mstl from treating piped env/execution as config input
        self.run_cmd([self.bin_path, "init", "-f", self.config_file, "--ignore-stdin"], cwd=self.repos_dir)

        # 1. Create a conflict scenario
        log("Creating conflict scenario...")

        # Modify Remote
        repo1_seed = os.path.join(self.seed_dir, "repo1")
        with open(os.path.join(repo1_seed, "README.md"), "w") as f:
            f.write("# Repo 1\nLine 1 (Remote)\nLine 2\n")
        self.run_cmd(["git", "add", "README.md"], cwd=repo1_seed)
        self.run_cmd(["git", "commit", "-m", "Remote Update"], cwd=repo1_seed)
        self.run_cmd(["git", "push", "origin", "main"], cwd=repo1_seed)

        # Modify Local (same line)
        repo1_local = os.path.join(self.repos_dir, "repo1")
        with open(os.path.join(repo1_local, "README.md"), "w") as f:
            f.write("# Repo 1\nLine 1 (Local)\nLine 2\n")
        self.run_cmd(["git", "add", "README.md"], cwd=repo1_local)
        self.run_cmd(["git", "commit", "-m", "Local Update"], cwd=repo1_local)

        # 2. Check Status
        log("Checking status (should show conflict or divergence)...")
        # Use --ignore-stdin just in case
        res = self.run_cmd([self.bin_path, "status", "-f", self.config_file, "--ignore-stdin"], cwd=self.repos_dir)
        print(res.stdout)

        if "!" not in res.stdout:
            fail("Status failed to detect conflict (expected '!')")

        # 3. Attempt Sync
        log("Running sync (expecting failure or conflict)...")

        # Pass "merge" to prompts, but use --ignore-stdin so ResolveCommonValues doesn't eat it as config
        res = self.run_cmd([self.bin_path, "sync", "-f", self.config_file, "--ignore-stdin"], cwd=self.repos_dir, input_str="merge\n", check=False)

        # It should NOT succeed. 'git pull --no-rebase' returns 1 on conflict.
        if res.returncode == 0:
            fail("Sync command succeeded unexpectedly despite merge conflict.")

        # Output should mention CONFLICT
        if "CONFLICT" not in res.stdout and "CONFLICT" not in res.stderr:
             log("Output:\n" + res.stdout + "\n" + res.stderr)
             fail("Sync failed but did not output standard git conflict message.")

        log("Success: Sync correctly failed on conflict.")

if __name__ == "__main__":
    test = MstlManualTestSyncConflict()
    test.run_test()
