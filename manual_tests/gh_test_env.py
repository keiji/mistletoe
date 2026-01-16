import os
import sys
import uuid
import json
import shutil
import subprocess
import signal
import time
from interactive_runner import print_green

class GhTestEnv:
    VISIBILITY_PRIVATE = "private"
    VISIBILITY_PUBLIC = "public"

    def __init__(self, root_dir=None):
        self.cwd = root_dir if root_dir else os.getcwd()
        self.user = self.get_gh_user()
        self.uuid = str(uuid.uuid4())[:8]
        self.auto_yes = False

        # Determine paths
        script_dir = os.path.dirname(os.path.abspath(__file__))
        self.mstl_bin = os.path.abspath(os.path.join(script_dir, "../bin/mstl-gh"))
        if sys.platform == "win32":
            self.mstl_bin += ".exe"

        if not os.path.exists(self.mstl_bin):
            print_green(f"[ERROR] mstl-gh binary not found at {self.mstl_bin}. Please run build_all.sh first.")
            # We don't exit here because generate_repo_names might be called before build in old logic, but now build is pre-req.
            # However, existing scripts call build_mstl_gh explicitly. We should probably remove those calls or make them no-ops.

        self.test_dir = os.path.join(self.cwd, f"test_workspace_{self.uuid}")
        self.config_file = os.path.join(self.test_dir, "mistletoe.json")
        self.dependency_file = os.path.join(self.test_dir, "dependency-graph.md")

        # Repository info placeholders
        self.repo_names = []
        self.repo_urls = {}

        # Configure git to use gh for credentials
        self.setup_git_auth()

    def setup_git_auth(self):
        try:
            subprocess.run(["gh", "auth", "setup-git"], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            # Suppress default branch hint
            subprocess.run(["git", "config", "--global", "init.defaultBranch", "main"], check=False, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        except Exception as e:
            print(f"[WARNING] Failed to setup git auth via gh: {e}")

    def get_gh_user(self):
        try:
            res = subprocess.run(
                ["gh", "api", "user", "--jq", ".login"],
                capture_output=True, text=True, check=True
            )
            return res.stdout.strip()
        except subprocess.CalledProcessError:
            print_green("[ERROR] Failed to get GitHub user. Is 'gh' installed and authenticated?")
            # FALLBACK for test environment without gh
            if os.environ.get("MOCK_GH_USER"):
                 return os.environ.get("MOCK_GH_USER")
            sys.exit(1)

    def generate_repo_names(self, count=3):
        while True:
            names = [f"mistletoe-test-{self.uuid}-{chr(65+i)}" for i in range(count)]
            if all(not self.repo_exists(n) for n in names):
                self.repo_names = names
                self.repo_urls = {n: f"git@github.com:{self.user}/{n}.git" for n in names}
                return
            self.uuid = str(uuid.uuid4())[:8]

    def repo_exists(self, repo_name):
        try:
            subprocess.run(
                ["gh", "repo", "view", f"{self.user}/{repo_name}"],
                check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
            )
            return True
        except subprocess.CalledProcessError:
            # Assume it doesn't exist if gh fails or returns error
            return False

    def setup_repos(self, visibility=VISIBILITY_PRIVATE):
        if visibility not in [self.VISIBILITY_PRIVATE, self.VISIBILITY_PUBLIC]:
            raise ValueError(f"Invalid visibility: {visibility}")

        if not self.repo_names:
            self.generate_repo_names()

        # Repositories are created here
        # Mock creation if MOCK_GH_USER is set
        if os.environ.get("MOCK_GH_USER"):
             # Create bare repos to simulate remote
             pass
        else:
             for repo in self.repo_names:
                subprocess.run(["gh", "repo", "create", repo, f"--{visibility}"], check=True)

        tmp_setup = os.path.join(self.cwd, f"setup_{self.uuid}")
        os.makedirs(tmp_setup, exist_ok=True)

        try:
            for repo in self.repo_names:
                r_dir = os.path.join(tmp_setup, repo)
                os.makedirs(r_dir)
                subprocess.run(["git", "init"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
                # Configure dummy user for committing
                subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
                subprocess.run(["git", "config", "user.name", "Test User"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)

                remote_url = f"git@github.com:{self.user}/{repo}.git"
                if os.environ.get("MOCK_GH_USER"):
                     # Create a local bare repo to act as remote
                     bare_dir = os.path.join(self.cwd, f"{repo}.git")
                     os.makedirs(bare_dir, exist_ok=True)
                     subprocess.run(["git", "init", "--bare"], cwd=bare_dir, check=True, stdout=subprocess.DEVNULL)
                     # Set default branch
                     subprocess.run(["git", "symbolic-ref", "HEAD", "refs/heads/main"], cwd=bare_dir, check=True, stdout=subprocess.DEVNULL)
                     remote_url = bare_dir

                subprocess.run(["git", "remote", "add", "origin", remote_url], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)

                readme_path = os.path.join(r_dir, "README.md")
                with open(readme_path, "w") as f:
                    f.write(f"# {repo}")

                subprocess.run(["git", "add", "."], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
                subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
                # Ensure the branch is named 'main' before pushing
                subprocess.run(["git", "branch", "-M", "main"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
                subprocess.run(["git", "push", "-u", "origin", "main"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
        finally:
            shutil.rmtree(tmp_setup, ignore_errors=True)

    def create_config_and_graph(self):
        os.makedirs(self.test_dir, exist_ok=True)

        # Handle variable number of repos, default to assuming at least one
        if not self.repo_names:
             return

        config_repos = []
        for n in self.repo_names:
            url = self.repo_urls[n]
            if os.environ.get("MOCK_GH_USER"):
                 url = os.path.join(self.cwd, f"{n}.git")
            config_repos.append({"url": url, "branch": "main", "id": n})

        config = {
            "repositories": config_repos
        }
        with open(self.config_file, "w") as f:
            json.dump(config, f, indent=2)

        # Only create graph if we have enough repos (mock implementation for fewer)
        with open(self.dependency_file, "w") as f:
            f.write("```mermaid\n")
            f.write("graph TD\n")
            if len(self.repo_names) >= 3:
                a, b, c = self.repo_names[0], self.repo_names[1], self.repo_names[2]
                f.write(f'    {a} --> {b}\n')
                f.write(f'    {b} --> {c}\n')
                if len(self.repo_names) >= 4:
                     d = self.repo_names[3]
                     f.write(f'    {d}\n')
            else:
                for n in self.repo_names:
                     f.write(f'    {n}\n')
            f.write("```\n")

    def cleanup(self):
        print_green("[-] Cleaning up workspace...")
        if os.path.exists(self.test_dir):
            shutil.rmtree(self.test_dir, ignore_errors=True)

        print_green("[-] Deleting remote repositories...")

        if os.environ.get("MOCK_GH_USER"):
             for repo in self.repo_names:
                  bare_dir = os.path.join(self.cwd, f"{repo}.git")
                  if os.path.exists(bare_dir):
                       shutil.rmtree(bare_dir, ignore_errors=True)
             return

        # 1. Rename all repositories
        print_green("    Renaming repositories...")
        for repo in self.repo_names:
            try:
                new_name = f"{repo}-deleting"
                subprocess.run(["gh", "repo", "rename", new_name, "--repo", f"{self.user}/{repo}", "--yes"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            except Exception as e:
                print_green(f"    Failed to rename {repo}: {e}")

        time.sleep(2)

        # 2. Verify renames
        print_green("    Verifying renames...")
        try:
            res = subprocess.run(["gh", "repo", "list", self.user, "--json", "name", "--limit", "1000"], capture_output=True, text=True, check=True)
            repos = json.loads(res.stdout)
            current_names = [r["name"] for r in repos]
            for repo in self.repo_names:
                new_name = f"{repo}-deleting"
                if new_name not in current_names:
                    print_green(f"    [WARNING] Rename verification failed for {repo}")
        except Exception as e:
            print_green(f"    Failed to verify renames: {e}")

        time.sleep(2)

        # 3. Delete renamed repositories
        print_green("    Deleting repositories...")
        for repo in self.repo_names:
            try:
                new_name = f"{repo}-deleting"
                subprocess.run(["gh", "repo", "delete", f"{self.user}/{new_name}", "--yes"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                print_green(f"    Deleted {repo}")
            except Exception as e:
                print_green(f"    Failed to delete {repo}: {e}")

    def run_mstl_cmd(self, args, cwd=None):
        if cwd is None:
            cwd = self.test_dir

        if self.auto_yes and "--yes" not in args:
            args = list(args) + ["--yes"]

        cmd = [self.mstl_bin] + args
        try:
            # Inject GH_EXEC_PATH if in mock mode to allow mstl to pass its own checks
            env = os.environ.copy()
            if os.environ.get("MOCK_GH_USER"):
                 # We need a dummy gh executable for mstl to check existence
                 # We assume the caller or test setup has put 'gh' in PATH or similar,
                 # or we skip strict gh checks if possible.
                 # mstl-gh uses checkGhAvailability which runs "gh auth status".
                 # We should mock this if possible.
                 pass

            return subprocess.run(
                cmd, cwd=cwd,
                check=True, text=True, env=env
            )
        except subprocess.CalledProcessError as e:
            raise e
