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

# Colors - Use interactive_runner's where possible, but keep local for helpers if needed
# Actually, interactive_runner doesn't export colors, only print_green.
# We will rely on print_green for logs.

def log(msg):
    print_green(f"[TEST] {msg}")

def fail(msg):
    # Use ANSI codes for red directly since interactive_runner doesn't export print_red
    print(f"\033[0;31m[FAIL]\033[0m {msg}")
    sys.exit(1)

class MstlManualTestSyncConflict:
    def __init__(self):
        self.root_dir = os.getcwd()
        self.test_dir = None # Created in setup
        self.bin_path = None
        self.repos_dir = None
        self.remote_dir = None
        self.config_file = None
        self.seed_dir = None

    def setup(self):
        self.test_dir = tempfile.mkdtemp(prefix="mstl_test_sync_")

        script_dir = os.path.dirname(os.path.abspath(__file__))
        self.bin_path = os.path.abspath(os.path.join(script_dir, "../bin/mstl"))
        if sys.platform == "win32":
            self.bin_path += ".exe"

        self.repos_dir = os.path.join(self.test_dir, "repos")
        self.remote_dir = os.path.join(self.test_dir, "remotes")
        self.config_file = os.path.join(self.test_dir, "mstl_config.json")
        self.seed_dir = os.path.join(self.test_dir, "seed")

        # We can register cleanup here, but InteractiveRunner also handles it.
        # Let's keep atexit as a fallback.
        atexit.register(self.cleanup)

        os.environ["GIT_AUTHOR_NAME"] = "Test User"
        os.environ["GIT_AUTHOR_EMAIL"] = "test@example.com"
        os.environ["GIT_COMMITTER_NAME"] = "Test User"
        os.environ["GIT_COMMITTER_EMAIL"] = "test@example.com"
        # Disable pager
        os.environ["GIT_PAGER"] = "cat"

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

    def run_test_logic(self):
        self.setup() # Initialize dirs
        if not os.path.exists(self.bin_path):
            fail(f"mstl binary not found at {self.bin_path}. Please run build_all.sh first.")

        self.setup_repo()
        self.create_config()

        log("Initializing local environment...")
        os.makedirs(self.repos_dir, exist_ok=True)
        # Use --ignore-stdin to prevent mstl from treating piped env/execution as config input
        self.run_cmd([self.bin_path, "init", "-f", self.config_file, "--ignore-stdin"], cwd=self.repos_dir)

        # Copy config to root to workaround path resolution issue with .mstl/config.json in tests
        shutil.copy(os.path.join(self.repos_dir, ".mstl", "config.json"), os.path.join(self.repos_dir, "config.json"))

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
        res = self.run_cmd([self.bin_path, "status", "-f", "config.json", "--ignore-stdin"], cwd=self.repos_dir)
        print(res.stdout)

        if "!" not in res.stdout:
            fail("Status failed to detect conflict (expected '!')")

        # 3. Attempt Sync
        log("Running sync (expecting failure or conflict)...")

        # Pass "merge" to prompts, but use --ignore-stdin so ResolveCommonValues doesn't eat it as config
        res = self.run_cmd([self.bin_path, "sync", "-f", "config.json", "--ignore-stdin"], cwd=self.repos_dir, input_str="merge\n", check=False)

        # It should NOT succeed. 'git pull --no-rebase' returns 1 on conflict.
        if res.returncode == 0:
            fail("Sync command succeeded unexpectedly despite merge conflict.")

        # Output should mention CONFLICT
        if "CONFLICT" not in res.stdout and "CONFLICT" not in res.stderr:
             log("Output:\n" + res.stdout + "\n" + res.stderr)
             fail("Sync failed but did not output standard git conflict message.")

        log("Success: Sync correctly failed on conflict.")

def main():
    runner = InteractiveRunner("mstl sync Conflict Test")
    runner.parse_args()
    test = MstlManualTestSyncConflict()

    description = (
        "This test verifies that 'mstl sync' correctly aborts when a merge conflict occurs.\n"
        "It will:\n"
        "1. Create temporary local bare repositories.\n"
        "2. Simulate a divergence between local and remote.\n"
        "3. Run 'sync' and verify it fails safely."
    )

    runner.execute_scenario(
        "Simulate Conflict",
        description,
        test.run_test_logic
    )

    runner.run_cleanup(test.cleanup)

if __name__ == "__main__":
    main()
