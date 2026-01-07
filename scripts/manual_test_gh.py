#!/usr/bin/env python3
import subprocess
import uuid
import json
import os
import sys
import shutil
import signal

class MstlGhTest:
    def __init__(self):
        self.cwd = os.getcwd()
        self.user = self.get_gh_user()
        self.uuid = str(uuid.uuid4())[:8]

        # Ensure unique repository names
        self.repo_a_name, self.repo_b_name, self.repo_c_name = self.ensure_unique_names()

        self.repo_a_url = f"https://github.com/{self.user}/{self.repo_a_name}.git"
        self.repo_b_url = f"https://github.com/{self.user}/{self.repo_b_name}.git"
        self.repo_c_url = f"https://github.com/{self.user}/{self.repo_c_name}.git"

        self.test_dir = os.path.join(self.cwd, f"test_workspace_{self.uuid}")
        self.config_file = os.path.join(self.test_dir, "mistletoe.json")
        self.dependency_file = os.path.join(self.test_dir, "dependencies.mmd")
        self.mstl_bin = os.path.abspath(os.path.join(self.cwd, "mstl-gh"))
        if sys.platform == "win32":
            self.mstl_bin += ".exe"

        self.cleanup_needed = True

        # Register signal handler
        signal.signal(signal.SIGINT, self.signal_handler)

    def signal_handler(self, sig, frame):
        print("\n\n[!] Interrupted by user (Ctrl+C).")
        print("Do you want to delete the temporary repositories? (yes/no) [yes]: ", end="", flush=True)
        try:
            choice = input().lower().strip()
        except EOFError:
            choice = "yes"

        if choice in ["", "yes", "y"]:
            self.cleanup()
        else:
            self.cleanup_needed = False
            print("Skipping cleanup. Repositories and workspace are left intact.")

        sys.exit(130)

    def ensure_unique_names(self):
        while True:
            name_a = f"mistletoe-test-{self.uuid}-A"
            name_b = f"mistletoe-test-{self.uuid}-B"
            name_c = f"mistletoe-test-{self.uuid}-C"

            if not self.repo_exists(name_a) and not self.repo_exists(name_b) and not self.repo_exists(name_c):
                return name_a, name_b, name_c

            # Regenerate UUID if collision found
            self.uuid = str(uuid.uuid4())[:8]

    def repo_exists(self, repo_name):
        # Check if repo exists using gh repo view
        # We expect exit code 1 if it doesn't exist
        print(f"[-] Checking if repository {repo_name} exists...")
        try:
            subprocess.run(
                ["gh", "repo", "view", f"{self.user}/{repo_name}"],
                check=True,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL
            )
            return True
        except subprocess.CalledProcessError:
            return False

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
            print(f"   - {self.repo_c_name}")
            print("\nType 'I AGREE' to proceed:")

            try:
                response = input()
            except EOFError:
                response = ""

            if response.strip() != "I AGREE":
                print("Consent not given. Aborting.")
                sys.exit(1)

    def checkpoint(self, name, description, urls=None):
        print("\n" + "="*60)
        print(f"CHECKPOINT: {name}")
        print("="*60)
        print(f"Expected State: {description}")
        if urls:
            print("-" * 60)
            print("Relevant URLs:")
            for k, v in urls.items():
                print(f"  {k}: {v}")
        print("-" * 60)
        print("Please verify the state manually.")
        print("Press Enter to continue (or Ctrl+C to abort)...")
        try:
            input()
        except EOFError:
            pass

    def build_mstl_gh(self):
        print("[-] Building mstl-gh...")
        self.run_cmd(["go", "build", "-o", self.mstl_bin, "cmd/mstl-gh/main.go"])

    def setup_repos(self):
        print(f"[-] Creating temporary repositories...")
        for repo in [self.repo_a_name, self.repo_b_name, self.repo_c_name]:
            self.run_cmd(["gh", "repo", "create", repo, "--private"])

        tmp_setup = os.path.join(self.cwd, f"setup_{self.uuid}")
        os.makedirs(tmp_setup, exist_ok=True)

        for repo in [self.repo_a_name, self.repo_b_name, self.repo_c_name]:
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
                {"url": self.repo_a_url, "branch": "main", "id": self.repo_a_name},
                {"url": self.repo_b_url, "branch": "main", "id": self.repo_b_name},
                {"url": self.repo_c_url, "branch": "main", "id": self.repo_c_name}
            ]
        }
        with open(self.config_file, "w") as f:
            json.dump(config, f, indent=2)
        print(f"[-] Config created at {self.config_file}")

        # Create dependency graph
        with open(self.dependency_file, "w") as f:
            f.write("graph TD\n")
            f.write(f'    "{self.repo_a_name}" --> "{self.repo_b_name}"\n')
            f.write(f'    "{self.repo_b_name}" --> "{self.repo_c_name}"\n')
        print(f"[-] Dependency graph created at {self.dependency_file}")


    def test_init(self):
        print("[TEST] Running init...")
        self.run_cmd([self.mstl_bin, "init", "-f", "mistletoe.json"], cwd=self.test_dir)

        self.checkpoint("Init Complete",
                        f"Directories for {self.repo_a_name}, {self.repo_b_name}, {self.repo_c_name} should exist.",
                        urls={
                            "Repo A": self.repo_a_url,
                            "Repo B": self.repo_b_url,
                            "Repo C": self.repo_c_url
                        })

    def test_switch(self):
        print("[TEST] Running switch...")
        self.run_cmd([self.mstl_bin, "switch", "-c", "feature/complex-dep"], cwd=self.test_dir)

        self.checkpoint("Switch Complete",
                        "All repositories should be on branch 'feature/complex-dep'.",
                         urls={
                            "Repo A": f"{self.repo_a_url}/tree/feature/complex-dep",
                            "Repo B": f"{self.repo_b_url}/tree/feature/complex-dep",
                            "Repo C": f"{self.repo_c_url}/tree/feature/complex-dep"
                        })

    def test_status_clean(self):
        print("[TEST] Running status (clean)...")
        self.run_cmd([self.mstl_bin, "status"], cwd=self.test_dir)

    def test_pr_create(self):
        print("[TEST] Running pr create...")
        # Make changes
        for repo in [self.repo_a_name, self.repo_b_name, self.repo_c_name]:
            r_dir = os.path.join(self.test_dir, repo)
            with open(os.path.join(r_dir, "test.txt"), "w") as f:
                f.write("test content")
            self.run_cmd(["git", "add", "."], cwd=r_dir)
            self.run_cmd(["git", "commit", "-m", "Add test.txt"], cwd=r_dir)
            self.run_cmd([self.mstl_bin, "push"], cwd=self.test_dir, input_str="yes\n")

        self.run_cmd([self.mstl_bin, "pr", "create", "-t", "Complex Dependency PR", "-b", "Testing complex dependencies", "-d", "dependencies.mmd"], cwd=self.test_dir)

        # Get PR URLs
        pr_urls = {}
        for name, url in [("Repo A", self.repo_a_url), ("Repo B", self.repo_b_url), ("Repo C", self.repo_c_url)]:
            res = self.run_cmd(["gh", "pr", "list", "--repo", url, "--head", "feature/complex-dep", "--json", "url"], capture_output=True)
            try:
                prs = json.loads(res.stdout)
                if prs:
                    pr_urls[f"{name} PR"] = prs[0]['url']
            except:
                pass

        self.checkpoint("PR Created",
                        f"PRs created for A, B, C.\n"
                        f"Repo A PR should list B as dependency.\n"
                        f"Repo B PR should list C as dependency and A as dependent.\n"
                        f"Repo C PR should list B as dependent.",
                        urls=pr_urls)

    def test_pr_status_with_feature_branch_config(self):
        print("[TEST] Running pr status with feature branch config...")
        # 1. Update config to use feature/complex-dep
        with open(self.config_file, "r") as f:
            config = json.load(f)

        for repo in config["repositories"]:
            repo["branch"] = "feature/complex-dep"

        with open(self.config_file, "w") as f:
            json.dump(config, f, indent=2)

        # 2. Run pr status and capture output
        res = self.run_cmd([self.mstl_bin, "pr", "status"], cwd=self.test_dir, capture_output=True)
        print(res.stdout)

        # 3. Verify that PR URLs are present
        if "github.com" not in res.stdout:
             raise Exception("PR Status failed to show PR URLs when config uses feature branch.")

        print("[-] PR Status successfully detected PRs with feature branch config.")

    def test_pr_status(self):
        print("[TEST] Running pr status...")
        self.run_cmd([self.mstl_bin, "pr", "status"], cwd=self.test_dir)

    def test_pr_update(self):
        print("[TEST] Running pr update...")
        # Make more changes in C
        r_dir = os.path.join(self.test_dir, self.repo_c_name)
        with open(os.path.join(r_dir, "update.txt"), "w") as f:
            f.write("updated content")
        self.run_cmd(["git", "add", "."], cwd=r_dir)
        self.run_cmd(["git", "commit", "-m", "Update content"], cwd=r_dir)
        self.run_cmd([self.mstl_bin, "push"], cwd=self.test_dir, input_str="yes\n")

        self.run_cmd([self.mstl_bin, "pr", "update"], cwd=self.test_dir, input_str="yes\n")

        # Get Repo C PR URL
        pr_urls = {}
        res = self.run_cmd(["gh", "pr", "list", "--repo", self.repo_c_url, "--head", "feature/complex-dep", "--json", "url"], capture_output=True)
        try:
             prs = json.loads(res.stdout)
             if prs:
                 pr_urls["Repo C PR"] = prs[0]['url']
        except:
            pass

        self.checkpoint("PR Updated", "Repo C PR updated. Check body for new commit hash in snapshot.", urls=pr_urls)

    def test_pr_update_permissions(self):
        print("[TEST] Running pr update (Permission Check - Creator == Me, No Block)...")
        # 1. Modify Repo C PR body to remove Mistletoe block
        print("[-] Removing Mistletoe block from Repo C PR...")
        self.run_cmd(["gh", "pr", "edit", "--body", "Manual edit without Mistletoe block"], cwd=os.path.join(self.test_dir, self.repo_c_name))

        # 2. Update Repo C content to force a change
        r_dir = os.path.join(self.test_dir, self.repo_c_name)
        with open(os.path.join(r_dir, "update_perm.txt"), "w") as f:
            f.write("permission test")
        self.run_cmd(["git", "add", "."], cwd=r_dir)
        self.run_cmd(["git", "commit", "-m", "Update for permission test"], cwd=r_dir)
        self.run_cmd([self.mstl_bin, "push"], cwd=self.test_dir, input_str="yes\n")

        # 3. Run pr update - should succeed because Creator is Me
        self.run_cmd([self.mstl_bin, "pr", "update"], cwd=self.test_dir, input_str="yes\n")

        self.checkpoint("PR Updated (Permission Test)",
                        "Repo C PR should be updated with a new Mistletoe block appended, even though it was missing.",
                        urls={})

    def test_pr_checkout(self):
        print("[TEST] Running pr checkout...")
        # Get PR URL from Repo A
        res = self.run_cmd(["gh", "pr", "list", "--repo", self.repo_a_url, "--head", "feature/complex-dep", "--json", "url"], capture_output=True)
        prs = json.loads(res.stdout)
        pr_url = prs[0]['url']

        # Prepare clean directory
        checkout_dir = os.path.join(self.cwd, f"test_checkout_{self.uuid}")
        os.makedirs(checkout_dir)

        try:
            self.run_cmd([self.mstl_bin, "pr", "checkout", "-u", pr_url], cwd=checkout_dir)

            # Verify
            if not os.path.isdir(os.path.join(checkout_dir, self.repo_a_name)) or \
               not os.path.isdir(os.path.join(checkout_dir, self.repo_b_name)) or \
               not os.path.isdir(os.path.join(checkout_dir, self.repo_c_name)):
                raise Exception("Checkout failed: Not all repos restored.")

            self.checkpoint("Checkout Complete", "A, B, C restored from Repo A's PR snapshot.")

        finally:
            shutil.rmtree(checkout_dir, ignore_errors=True)

    def cleanup(self):
        if not self.cleanup_needed:
            return

        print("[-] Cleaning up...")
        if os.path.exists(self.test_dir):
            shutil.rmtree(self.test_dir, ignore_errors=True)

        print(f"[-] Deleting remote repositories...")
        for repo in [self.repo_a_name, self.repo_b_name, self.repo_c_name]:
            try:
                # Rename before deletion to indicate it's being deleted
                new_name = f"{repo}-deleting"
                print(f"[-] Renaming {repo} to {new_name}...")
                self.run_cmd(["gh", "repo", "rename", new_name, "--repo", f"{self.user}/{repo}", "--yes"])

                # Delete the renamed repository
                print(f"[-] Deleting {new_name}...")
                self.run_cmd(["gh", "repo", "delete", f"{self.user}/{new_name}", "--yes"])
            except Exception as e:
                print(f"Failed to delete {repo} (or renamed version): {e}")

        self.cleanup_needed = False

    def run(self):
        try:
            self.check_existing_repos()
            self.build_mstl_gh()
            self.setup_repos()
            self.create_config()
            self.test_init()
            self.test_switch()
            self.test_status_clean()
            self.test_pr_create()
            self.test_pr_status_with_feature_branch_config()
            self.test_pr_status()
            self.test_pr_update()
            self.test_pr_update_permissions()
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
