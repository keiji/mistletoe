import subprocess
import uuid
import json
import os
import sys
import shutil
import time

class MstlGhTest:
    def __init__(self):
        self.uuid = str(uuid.uuid4())[:8]
        self.user = self.get_gh_user()
        self.repo_a_name = f"mistletoe-test-{self.uuid}-A"
        self.repo_b_name = f"mistletoe-test-{self.uuid}-B"
        self.repo_a_url = f"https://github.com/{self.user}/{self.repo_a_name}.git"
        self.repo_b_url = f"https://github.com/{self.user}/{self.repo_b_name}.git"
        self.cwd = os.getcwd()
        self.test_dir = os.path.join(self.cwd, f"test_workspace_{self.uuid}")
        self.config_file = os.path.join(self.test_dir, "mistletoe.json")
        self.mstl_bin = os.path.abspath(os.path.join(self.cwd, "mstl-gh"))
        if sys.platform == "win32":
            self.mstl_bin += ".exe"

    def run_cmd(self, args, cwd=None, input_str=None, check=True, capture_output=False):
        if cwd is None:
            cwd = self.cwd

        # print(f"DEBUG: Running {args} in {cwd}")
        try:
            result = subprocess.run(
                args,
                cwd=cwd,
                input=input_str.encode() if input_str else None,
                check=check,
                capture_output=capture_output,
                text=True if capture_output else False
            )
            return result
        except subprocess.CalledProcessError as e:
            print(f"Error running command: {args}")
            if e.stdout:
                print(f"Stdout: {e.stdout}")
            if e.stderr:
                print(f"Stderr: {e.stderr}")
            raise e

    def get_gh_user(self):
        print("[-] Getting GitHub user...")
        res = self.run_cmd(["gh", "api", "user", "--jq", ".login"], capture_output=True)
        return res.stdout.strip()

    def check_existing_repos(self):
        print("[-] Checking for existing repositories...")
        res = self.run_cmd(["gh", "repo", "list", "--limit", "1", "--json", "name"], capture_output=True)
        repos = json.loads(res.stdout)

        if len(repos) > 0:
            print("\n" + "!" * 60)
            print("WARNING: Existing repositories found on your account.")
            print("!" * 60)
            print("To proceed, you must acknowledge the following:")
            print("1. Existing repositories exist on this account.")
            print("2. Although unlikely due to naming, there is a risk of destruction of existing repositories.")
            print("3. The following temporary repositories will be created:")
            print(f"   - {self.repo_a_name}")
            print(f"   - {self.repo_b_name}")
            print("\nType 'I AGREE' to proceed:")

            try:
                response = input()
            except EOFError:
                response = ""

            if response.strip() != "I AGREE":
                print("Consent not given. Aborting.")
                sys.exit(1)

    def build_mstl_gh(self):
        print("[-] Building mstl-gh...")
        self.run_cmd(["go", "build", "-o", self.mstl_bin, "cmd/mstl-gh/main.go"])

    def setup_repos(self):
        print(f"[-] Creating temporary repositories...")
        self.run_cmd(["gh", "repo", "create", self.repo_a_name, "--private"])
        self.run_cmd(["gh", "repo", "create", self.repo_b_name, "--private"])

        # Initialize repos with a commit so they can be cloned/pushed to
        # We need to clone them, add a file, and push?
        # Actually `gh repo create` with `--private` creates empty repo.
        # mstl init works on empty repos? No, `init` clones.
        # Cloning an empty repo is fine, but checking out a branch might fail if it doesn't exist.
        # Let's initialize them with a README.
        # But `gh repo create` has `--add-readme`? No, maybe.
        # Safer: Create locally, init, push.

        tmp_setup = os.path.join(self.cwd, f"setup_{self.uuid}")
        os.makedirs(tmp_setup, exist_ok=True)

        for repo in [self.repo_a_name, self.repo_b_name]:
            r_dir = os.path.join(tmp_setup, repo)
            os.makedirs(r_dir)
            self.run_cmd(["git", "init"], cwd=r_dir)
            self.run_cmd(["git", "remote", "add", "origin", f"https://github.com/{self.user}/{repo}.git"], cwd=r_dir)
            with open(os.path.join(r_dir, "README.md"), "w") as f:
                f.write(f"# {repo}")
            self.run_cmd(["git", "add", "."], cwd=r_dir)
            self.run_cmd(["git", "commit", "-m", "Initial commit"], cwd=r_dir)
            self.run_cmd(["git", "push", "-u", "origin", "main"], cwd=r_dir)

        shutil.rmtree(tmp_setup)

    def create_config(self):
        os.makedirs(self.test_dir, exist_ok=True)
        config = {
            "repositories": [
                {"url": self.repo_a_url, "branch": "main"},
                {"url": self.repo_b_url, "branch": "main"}
            ]
        }
        with open(self.config_file, "w") as f:
            json.dump(config, f, indent=2)
        print(f"[-] Config created at {self.config_file}")

    def test_init(self):
        print("[TEST] Running init...")
        self.run_cmd([self.mstl_bin, "init", "-f", "mistletoe.json"], cwd=self.test_dir)

        # Verify
        if not os.path.isdir(os.path.join(self.test_dir, self.repo_a_name)):
            raise Exception("Repo A directory not found")
        if not os.path.isdir(os.path.join(self.test_dir, self.repo_b_name)):
            raise Exception("Repo B directory not found")

    def test_switch(self):
        print("[TEST] Running switch...")
        # Create branch locally first in one? No, switch -c creates it.
        self.run_cmd([self.mstl_bin, "switch", "-c", "feature/test-gh"], cwd=self.test_dir)

        # Verify
        res = self.run_cmd(["git", "symbolic-ref", "--short", "HEAD"], cwd=os.path.join(self.test_dir, self.repo_a_name), capture_output=True)
        if res.stdout.strip() != "feature/test-gh":
            raise Exception(f"Repo A not on correct branch: {res.stdout}")

    def test_status_clean(self):
        print("[TEST] Running status (clean)...")
        self.run_cmd([self.mstl_bin, "status"], cwd=self.test_dir)

    def test_push(self):
        print("[TEST] Running push...")
        # Make changes
        for repo in [self.repo_a_name, self.repo_b_name]:
            r_dir = os.path.join(self.test_dir, repo)
            with open(os.path.join(r_dir, "test.txt"), "w") as f:
                f.write("test content")
            self.run_cmd(["git", "add", "."], cwd=r_dir)
            self.run_cmd(["git", "commit", "-m", "Add test.txt"], cwd=r_dir)

        # Push with confirmation
        self.run_cmd([self.mstl_bin, "push"], cwd=self.test_dir, input_str="yes\n")

    def test_pr_create(self):
        print("[TEST] Running pr create...")
        self.run_cmd([self.mstl_bin, "pr", "create", "-t", "Test PR", "-b", "Test Body"], cwd=self.test_dir)

        # Verify PRs exist
        res = self.run_cmd(["gh", "pr", "list", "--repo", self.repo_a_url, "--head", "feature/test-gh", "--json", "url"], capture_output=True)
        prs = json.loads(res.stdout)
        if len(prs) == 0:
            raise Exception("PR for Repo A not found")

    def test_pr_status(self):
        print("[TEST] Running pr status...")
        self.run_cmd([self.mstl_bin, "pr", "status"], cwd=self.test_dir)

    def test_pr_update(self):
        print("[TEST] Running pr update...")
        # Make more changes
        r_dir = os.path.join(self.test_dir, self.repo_a_name)
        with open(os.path.join(r_dir, "update.txt"), "w") as f:
            f.write("updated content")
        self.run_cmd(["git", "add", "."], cwd=r_dir)
        self.run_cmd(["git", "commit", "-m", "Update content"], cwd=r_dir)
        self.run_cmd([self.mstl_bin, "push"], cwd=self.test_dir, input_str="yes\n")

        self.run_cmd([self.mstl_bin, "pr", "update"], cwd=self.test_dir, input_str="yes\n")

    def test_snapshot(self):
        print("[TEST] Running snapshot...")
        self.run_cmd([self.mstl_bin, "snapshot", "-o", "snapshot.json"], cwd=self.test_dir)
        if not os.path.exists(os.path.join(self.test_dir, "snapshot.json")):
            raise Exception("Snapshot file not created")

    def test_pr_checkout(self):
        print("[TEST] Running pr checkout...")
        # Get PR URL from Repo A
        res = self.run_cmd(["gh", "pr", "list", "--repo", self.repo_a_url, "--head", "feature/test-gh", "--json", "url"], capture_output=True)
        prs = json.loads(res.stdout)
        pr_url = prs[0]['url']

        # Prepare clean directory
        checkout_dir = os.path.join(self.cwd, f"test_checkout_{self.uuid}")
        os.makedirs(checkout_dir)

        try:
            self.run_cmd([self.mstl_bin, "pr", "checkout", "-u", pr_url], cwd=checkout_dir)

            # Verify
            if not os.path.isdir(os.path.join(checkout_dir, self.repo_a_name)):
                raise Exception("Checkout Repo A failed")
            if not os.path.isdir(os.path.join(checkout_dir, self.repo_b_name)):
                raise Exception("Checkout Repo B failed")
        finally:
            shutil.rmtree(checkout_dir, ignore_errors=True)

    def cleanup(self):
        print("[-] Cleaning up...")
        if os.path.exists(self.test_dir):
            shutil.rmtree(self.test_dir, ignore_errors=True)

        print(f"[-] Deleting remote repositories...")
        for repo in [self.repo_a_name, self.repo_b_name]:
            try:
                # gh repo delete requires confirmation
                self.run_cmd(["gh", "repo", "delete", repo, "--yes"])
            except Exception as e:
                print(f"Failed to delete {repo}: {e}")

    def run(self):
        try:
            self.check_existing_repos()
            self.build_mstl_gh()
            self.setup_repos()
            self.create_config()
            self.test_init()
            self.test_switch()
            self.test_status_clean()
            self.test_push()
            self.test_pr_create()
            self.test_pr_status()
            self.test_pr_update()
            self.test_snapshot()
            self.test_pr_checkout()
            print("\n" + "=" * 30)
            print("ALL TESTS PASSED")
            print("=" * 30 + "\n")
        except Exception as e:
            print(f"\n[ERROR] Test failed: {e}")
            import traceback
            traceback.print_exc()
        finally:
            self.cleanup()

if __name__ == "__main__":
    test = MstlGhTest()
    test.run()
