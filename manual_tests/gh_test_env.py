import os
import sys
import uuid
import json
import shutil
import subprocess
import signal

class GhTestEnv:
    def __init__(self, root_dir=None):
        self.cwd = root_dir if root_dir else os.getcwd()
        self.user = self.get_gh_user()
        self.uuid = str(uuid.uuid4())[:8]

        # Determine paths
        self.mstl_bin = os.path.abspath(os.path.join(self.cwd, "mstl-gh"))
        if sys.platform == "win32":
            self.mstl_bin += ".exe"

        self.test_dir = os.path.join(self.cwd, f"test_workspace_{self.uuid}")
        self.config_file = os.path.join(self.test_dir, "mistletoe.json")
        self.dependency_file = os.path.join(self.test_dir, "dependencies.mmd")

        # Repository info placeholders
        self.repo_names = []
        self.repo_urls = {}

    def get_gh_user(self):
        try:
            res = subprocess.run(
                ["gh", "api", "user", "--jq", ".login"],
                capture_output=True, text=True, check=True
            )
            return res.stdout.strip()
        except subprocess.CalledProcessError:
            print("[ERROR] Failed to get GitHub user. Is 'gh' installed and authenticated?")
            sys.exit(1)

    def generate_repo_names(self, count=3):
        while True:
            names = [f"mistletoe-test-{self.uuid}-{chr(65+i)}" for i in range(count)]
            if all(not self.repo_exists(n) for n in names):
                self.repo_names = names
                self.repo_urls = {n: f"https://github.com/{self.user}/{n}.git" for n in names}
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
            return False

    def build_mstl_gh(self):
        print(f"[-] Building mstl-gh...")
        subprocess.run(
            ["go", "build", "-o", self.mstl_bin, "cmd/mstl-gh/main.go"],
            cwd=self.cwd, check=True
        )

    def setup_repos(self):
        if not self.repo_names:
            self.generate_repo_names()

        print(f"[-] Creating temporary repositories: {', '.join(self.repo_names)}...")
        for repo in self.repo_names:
            subprocess.run(["gh", "repo", "create", repo, "--private"], check=True)

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
                subprocess.run(["git", "remote", "add", "origin", f"https://github.com/{self.user}/{repo}.git"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)

                readme_path = os.path.join(r_dir, "README.md")
                with open(readme_path, "w") as f:
                    f.write(f"# {repo}")

                subprocess.run(["git", "add", "."], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
                subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
                subprocess.run(["git", "push", "-u", "origin", "main"], cwd=r_dir, check=True, stdout=subprocess.DEVNULL)
        finally:
            shutil.rmtree(tmp_setup, ignore_errors=True)

    def create_config_and_graph(self):
        os.makedirs(self.test_dir, exist_ok=True)

        # Assuming 3 repos A, B, C for standard graph
        if len(self.repo_names) < 3:
             raise Exception("Need at least 3 repos for standard graph")

        a, b, c = self.repo_names[0], self.repo_names[1], self.repo_names[2]

        config = {
            "repositories": [
                {"url": self.repo_urls[n], "branch": "main", "id": n} for n in self.repo_names
            ]
        }
        with open(self.config_file, "w") as f:
            json.dump(config, f, indent=2)

        with open(self.dependency_file, "w") as f:
            f.write("graph TD\n")
            f.write(f'    "{a}" --> "{b}"\n')
            f.write(f'    "{b}" --> "{c}"\n')

    def cleanup(self):
        print("[-] Cleaning up workspace...")
        if os.path.exists(self.test_dir):
            shutil.rmtree(self.test_dir, ignore_errors=True)

        print("[-] Deleting remote repositories...")
        for repo in self.repo_names:
            try:
                new_name = f"{repo}-deleting"
                subprocess.run(["gh", "repo", "rename", new_name, "--repo", f"{self.user}/{repo}", "--yes"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                subprocess.run(["gh", "repo", "delete", f"{self.user}/{new_name}", "--yes"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                print(f"    Deleted {repo}")
            except Exception as e:
                print(f"    Failed to delete {repo}: {e}")

    def run_mstl_cmd(self, args, cwd=None, input_str=None):
        if cwd is None:
            cwd = self.test_dir

        cmd = [self.mstl_bin] + args
        try:
            return subprocess.run(
                cmd, cwd=cwd, input=input_str.encode() if input_str else None,
                check=True, text=True
            )
        except subprocess.CalledProcessError as e:
            raise e
