import os
import sys
import shutil
import tempfile
import subprocess
import json
import atexit

# Add current directory to sys.path to import interactive_runner
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from interactive_runner import InteractiveRunner, print_green

# Colors
GREEN = '\033[0;32m'
RED = '\033[0;31m'
NC = '\033[0m'

def log(msg):
    print_green(f"[TEST] {msg}")

def fail(msg):
    print(f"{RED}[FAIL]{NC} {msg}")
    sys.exit(1)

class MstlManualTest:
    def __init__(self):
        self.root_dir = os.getcwd()
        self.test_dir = None
        self.bin_path = None
        self.repos_dir = None
        self.remote_dir = None
        self.config_file = None
        self.seed_dir = None

    def setup(self):
        self.test_dir = tempfile.mkdtemp(prefix="mstl_test_")
        self.bin_path = os.path.join(self.test_dir, "bin", "mstl")
        self.repos_dir = os.path.join(self.test_dir, "repos")
        self.remote_dir = os.path.join(self.test_dir, "remotes")
        self.config_file = os.path.join(self.test_dir, "mstl_config.json")
        self.seed_dir = os.path.join(self.test_dir, "seed")

        atexit.register(self.cleanup)

        # Setup git env
        os.environ["GIT_AUTHOR_NAME"] = "Test User"
        os.environ["GIT_AUTHOR_EMAIL"] = "test@example.com"
        os.environ["GIT_COMMITTER_NAME"] = "Test User"
        os.environ["GIT_COMMITTER_EMAIL"] = "test@example.com"

        log(f"Test Directory: {self.test_dir}")

    def cleanup(self):
        if self.test_dir and os.path.exists(self.test_dir):
            log("Cleaning up temporary directory...")
            try:
                shutil.rmtree(self.test_dir)
            except Exception as e:
                print(f"Cleanup failed: {e}")

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

        # Verify version
        res = self.run_cmd([self.bin_path, "version"])
        if "mstl version" not in res.stdout:
            fail("mstl version command failed")

        # Verify help
        res = self.run_cmd([self.bin_path, "help"])
        if "Usage:" not in res.stdout:
            fail("mstl help command failed")

        log("Build success")

    def setup_remotes(self):
        log("Setting up remote repositories...")
        os.makedirs(self.remote_dir, exist_ok=True)
        self.run_cmd(["git", "init", "--bare", os.path.join(self.remote_dir, "repo1.git")])
        self.run_cmd(["git", "init", "--bare", os.path.join(self.remote_dir, "repo2.git")])

        log("Seeding remotes...")
        os.makedirs(self.seed_dir, exist_ok=True)

        # Repo 1
        self.run_cmd(["git", "clone", os.path.join(self.remote_dir, "repo1.git"), os.path.join(self.seed_dir, "repo1")])
        repo1_seed = os.path.join(self.seed_dir, "repo1")
        self.run_cmd(["git", "checkout", "-b", "main"], cwd=repo1_seed, check=False)

        with open(os.path.join(repo1_seed, "README.md"), "w") as f:
            f.write("# Repo 1")

        self.run_cmd(["git", "add", "README.md"], cwd=repo1_seed)
        self.run_cmd(["git", "commit", "-m", "Initial commit repo1"], cwd=repo1_seed)
        self.run_cmd(["git", "push", "origin", "main"], cwd=repo1_seed)
        self.run_cmd(["git", "--git-dir", os.path.join(self.remote_dir, "repo1.git"), "symbolic-ref", "HEAD", "refs/heads/main"])

        # Repo 2
        self.run_cmd(["git", "clone", os.path.join(self.remote_dir, "repo2.git"), os.path.join(self.seed_dir, "repo2")])
        repo2_seed = os.path.join(self.seed_dir, "repo2")
        self.run_cmd(["git", "checkout", "-b", "main"], cwd=repo2_seed, check=False)

        with open(os.path.join(repo2_seed, "README.md"), "w") as f:
            f.write("# Repo 2")

        self.run_cmd(["git", "add", "README.md"], cwd=repo2_seed)
        self.run_cmd(["git", "commit", "-m", "Initial commit repo2"], cwd=repo2_seed)
        self.run_cmd(["git", "push", "origin", "main"], cwd=repo2_seed)
        self.run_cmd(["git", "--git-dir", os.path.join(self.remote_dir, "repo2.git"), "symbolic-ref", "HEAD", "refs/heads/main"])

    def create_config(self):
        log("Creating mstl configuration...")
        config = {
            "repositories": [
                {"url": os.path.join(self.remote_dir, "repo1.git")},
                {"url": os.path.join(self.remote_dir, "repo2.git")}
            ]
        }
        with open(self.config_file, "w") as f:
            json.dump(config, f, indent=2)

    def test_init(self):
        log("Testing 'init'...")
        os.makedirs(self.repos_dir, exist_ok=True)
        self.run_cmd([self.bin_path, "init", "-f", self.config_file, "--ignore-stdin"], cwd=self.repos_dir)

        if not os.path.isdir(os.path.join(self.repos_dir, "repo1")) or not os.path.isdir(os.path.join(self.repos_dir, "repo2")):
            fail("Repositories not cloned")
        log("Success: mstl init")

    def test_status_clean(self):
        log("Testing 'status' (Clean)...")
        res = self.run_cmd([self.bin_path, "status", "-f", self.config_file, "--ignore-stdin"], cwd=self.repos_dir)

        # Filter out legend
        output_lines = [line for line in res.stdout.splitlines() if "Status Legend" not in line]
        clean_output = "\n".join(output_lines)
        if "!" in clean_output:
            fail("Status showed conflict on clean repo")
        log("Success: mstl status clean")

    def test_switch(self):
        log("Testing 'switch'...")
        self.run_cmd([self.bin_path, "switch", "-f", self.config_file, "-c", "feature/test-branch", "--ignore-stdin"], cwd=self.repos_dir)

        # Verify
        res = self.run_cmd(["git", "symbolic-ref", "--short", "HEAD"], cwd=os.path.join(self.repos_dir, "repo1"))
        if res.stdout.strip() != "feature/test-branch":
            fail(f"repo1 not on feature/test-branch (was {res.stdout.strip()})")
        log("Success: mstl switch -c")

    def test_push(self):
        log("Testing 'push'...")
        repo1_path = os.path.join(self.repos_dir, "repo1")
        with open(os.path.join(repo1_path, "README.md"), "a") as f:
            f.write("\nChange in repo1")

        self.run_cmd(["git", "add", "README.md"], cwd=repo1_path)
        self.run_cmd(["git", "commit", "-m", "Update repo1"], cwd=repo1_path)

        # Verify status shows unpushed (>)
        res = self.run_cmd([self.bin_path, "status", "-f", self.config_file, "--ignore-stdin"], cwd=self.repos_dir)
        if ">" not in res.stdout:
            fail("Status did not show unpushed commit (>)")

        log("Running push (with input 'yes')...")
        self.run_cmd([self.bin_path, "push", "-f", self.config_file, "--ignore-stdin"], cwd=self.repos_dir, input_str="yes\n")

        # Verify remote
        self.run_cmd(["git", "fetch", "origin"], cwd=os.path.join(self.seed_dir, "repo1")) # Use seed dir to check remote
        res = self.run_cmd(["git", "log", "origin/feature/test-branch", "--oneline"], cwd=os.path.join(self.seed_dir, "repo1"))
        if "Update repo1" not in res.stdout:
            fail("Remote repo1 does not have the pushed commit")
        log("Success: mstl push")

    def test_sync(self):
        log("Testing 'sync'...")
        # Switch back to main
        log("Switching back to main for sync test...")
        self.run_cmd([self.bin_path, "switch", "-f", self.config_file, "main", "--ignore-stdin"], cwd=self.repos_dir)

        # Update remote repo2
        repo2_seed = os.path.join(self.seed_dir, "repo2")
        self.run_cmd(["git", "checkout", "main"], cwd=repo2_seed)
        with open(os.path.join(repo2_seed, "README.md"), "a") as f:
            f.write("\nRemote Change repo2")
        self.run_cmd(["git", "add", "README.md"], cwd=repo2_seed)
        self.run_cmd(["git", "commit", "-m", "Remote update repo2"], cwd=repo2_seed)
        self.run_cmd(["git", "push", "origin", "main"], cwd=repo2_seed)

        # Verify status shows pullable (<)
        res = self.run_cmd([self.bin_path, "status", "-f", self.config_file, "--ignore-stdin"], cwd=self.repos_dir)
        if "<" not in res.stdout:
            fail("Status did not show pullable commit (<)")

        log("Running sync...")
        self.run_cmd([self.bin_path, "sync", "-f", self.config_file, "--ignore-stdin"], cwd=self.repos_dir)

        # Verify local repo2
        with open(os.path.join(self.repos_dir, "repo2", "README.md"), "r") as f:
            content = f.read()
        if "Remote Change repo2" not in content:
            fail("repo2 did not receive remote changes")
        log("Success: mstl sync")

    def test_snapshot(self):
        log("Testing 'snapshot'...")
        self.run_cmd([self.bin_path, "snapshot", "--ignore-stdin"], cwd=self.repos_dir)

        # Check file
        files = os.listdir(self.repos_dir)
        snapshot_files = [f for f in files if f.startswith("mistletoe-snapshot-") and f.endswith(".json")]
        if not snapshot_files:
            fail("Snapshot file not created")

        snapshot_file = os.path.join(self.repos_dir, snapshot_files[0])
        log(f"Checking snapshot content in {snapshot_files[0]}...")

        with open(snapshot_file, "r") as f:
            content = f.read()
            if "repo1" not in content:
                fail("Snapshot missing repo1")
            if "main" not in content:
                fail("Snapshot missing main branch info")

        log("Success: mstl snapshot")

    def run_logic(self):
        self.setup()
        self.build_mstl()
        self.setup_remotes()
        self.create_config()
        self.test_init()
        self.test_status_clean()
        self.test_switch()
        self.test_push()
        self.test_sync()
        self.test_snapshot()
        log("All tests passed!")

def main():
    runner = InteractiveRunner("mstl Core Functionality Test")
    runner.parse_args()
    test = MstlManualTest()

    description = (
        "This test verifies the core functionality of 'mstl' (init, status, switch, push, sync, snapshot).\n"
        "It will create temporary local bare repositories to simulate remotes."
    )

    runner.execute_scenario(
        "Core Feature Check",
        description,
        test.run_logic
    )

    runner.run_cleanup(test.cleanup)

if __name__ == "__main__":
    main()
