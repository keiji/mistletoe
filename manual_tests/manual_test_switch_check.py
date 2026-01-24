import os
import sys
import shutil
import tempfile
import subprocess
import json
import atexit

sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from interactive_runner import InteractiveRunner, print_green, print_red

def log(msg):
    print_green(f"[TEST] {msg}")

def fail(msg):
    print_red(f"[FAIL] {msg}")
    sys.exit(1)

class SwitchCheckTest:
    def __init__(self, runner):
        self.runner = runner
        self.test_dir = tempfile.mkdtemp(prefix="mstl_switch_check_")
        script_dir = os.path.dirname(os.path.abspath(__file__))
        self.bin_path = os.path.abspath(os.path.join(script_dir, "../bin/mstl"))
        if sys.platform == "win32":
            self.bin_path += ".exe"

        self.repo1_dir = os.path.join(self.test_dir, "repo1")
        self.repo2_dir = os.path.join(self.test_dir, "repo2")
        self.config_file = os.path.join(self.test_dir, "mstl.json")
        atexit.register(self.cleanup)

    def cleanup(self):
        if os.path.exists(self.test_dir):
            try:
                shutil.rmtree(self.test_dir)
            except:
                pass

    def run_git(self, cwd, *args):
        # Configure user for commit
        env = os.environ.copy()
        env["GIT_AUTHOR_NAME"] = "Test"
        env["GIT_AUTHOR_EMAIL"] = "test@example.com"
        env["GIT_COMMITTER_NAME"] = "Test"
        env["GIT_COMMITTER_EMAIL"] = "test@example.com"
        subprocess.run(["git"] + list(args), cwd=cwd, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, env=env)

    def setup(self):
        log(f"Setting up in {self.test_dir}")
        os.makedirs(self.repo1_dir)
        os.makedirs(self.repo2_dir)

        # Init Repos
        self.run_git(self.repo1_dir, "init")
        self.run_git(self.repo1_dir, "remote", "add", "origin", "dummy1")
        self.run_git(self.repo1_dir, "commit", "--allow-empty", "-m", "init")
        self.run_git(self.repo1_dir, "checkout", "-b", "main")

        self.run_git(self.repo2_dir, "init")
        self.run_git(self.repo2_dir, "remote", "add", "origin", "dummy2")
        self.run_git(self.repo2_dir, "commit", "--allow-empty", "-m", "init")
        self.run_git(self.repo2_dir, "checkout", "-b", "dev") # Different branch

        # Config
        config = {
            "repositories": [
                {"url": "dummy1", "id": "repo1"},
                {"url": "dummy2", "id": "repo2"}
            ]
        }
        with open(self.config_file, "w") as f:
            json.dump(config, f)

    def run(self):
        self.setup()

        # Step 1: Abort case
        log("Step 1: Running switch -c with mismatch. Expect prompt. Inputting 'n'...")
        cmd = [self.bin_path, "switch", "-c", "new-feature", "--file", self.config_file, "--ignore-stdin"]

        p = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, cwd=self.test_dir)
        stdout, stderr = p.communicate(input="n\n")

        combined = stdout + stderr
        # print(combined) # Debug

        if p.returncode == 0:
            fail("Expected non-zero exit code when aborted, got 0")

        if "Branch names do not match" not in combined:
            print(f"STDOUT:\n{stdout}")
            print(f"STDERR:\n{stderr}")
            fail("Did not find warning message")
        # Check for table structure indicators
        if "repo1" not in combined or "main" not in combined:
            fail("Did not find repo1/main in status table")
        if "repo2" not in combined or "dev" not in combined:
            fail("Did not find repo2/dev in status table")
        if "Repository" not in combined or "Local" not in combined:
            fail("Did not find table headers")

        log("Step 1 Passed (Aborted correctly).")

        # Step 2: Proceed case
        log("Step 2: Running switch -c again. Inputting 'y'...")
        p = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, cwd=self.test_dir)
        stdout, stderr = p.communicate(input="y\n")

        if p.returncode != 0:
            fail(f"Expected success, got exit code {p.returncode}. Output:\n{stdout}\n{stderr}")

        # Verify branches
        def get_branch(d):
            res = subprocess.run(["git", "symbolic-ref", "--short", "HEAD"], cwd=d, capture_output=True, text=True)
            return res.stdout.strip()

        b1 = get_branch(self.repo1_dir)
        b2 = get_branch(self.repo2_dir)

        if b1 != "new-feature" or b2 != "new-feature":
            fail(f"Branches not switched. Repo1: {b1}, Repo2: {b2}")

        log("Step 2 Passed (Switched correctly).")
        log("All tests passed.")

def main():
    runner = InteractiveRunner("Switch Consistency Check")
    test = SwitchCheckTest(runner)
    test.run()

if __name__ == "__main__":
    main()
