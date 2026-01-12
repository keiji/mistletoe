
import os
import shutil
import tempfile
import subprocess
import json
import unittest

class TestInitDependencies(unittest.TestCase):
    def setUp(self):
        self.test_dir = tempfile.mkdtemp()
        self.mstl_bin = os.path.abspath("mstl")

        # Build mstl
        subprocess.run(["go", "build", "-o", self.mstl_bin, "cmd/mstl/main.go"], check=True)

        # Setup 3 bare repos
        self.repos = {}
        for name in ["repoA", "repoB", "repoC"]:
            repo_path = os.path.join(self.test_dir, name)
            os.makedirs(repo_path)
            subprocess.run(["git", "init", "--bare"], cwd=repo_path, check=True)
            self.repos[name] = repo_path

        # Create config.json
        self.config_path = os.path.join(self.test_dir, "config.json")
        self.config = {
            "repositories": [
                {"id": "repoA", "url": "file://" + self.repos["repoA"]},
                {"id": "repoB", "url": "file://" + self.repos["repoB"]},
                {"id": "repoC", "url": "file://" + self.repos["repoC"]}
            ]
        }
        with open(self.config_path, "w") as f:
            json.dump(self.config, f)

        # Create valid dependency graph
        self.dep_path = os.path.join(self.test_dir, "dep.md")
        with open(self.dep_path, "w") as f:
            f.write("```mermaid\ngraph TD\n    repoA --> repoB\n    repoB --> repoC\n```\n")

        # Create invalid dependency graph (syntax ok, invalid ID)
        self.invalid_dep_path = os.path.join(self.test_dir, "invalid_dep.md")
        with open(self.invalid_dep_path, "w") as f:
            f.write("```mermaid\ngraph TD\n    repoA --> repoZ\n```\n")

    def tearDown(self):
        shutil.rmtree(self.test_dir)
        if os.path.exists(self.mstl_bin):
            os.remove(self.mstl_bin)

    def test_init_with_valid_dependencies(self):
        dest_dir = os.path.join(self.test_dir, "work_valid")

        cmd = [
            self.mstl_bin, "init",
            "-f", self.config_path,
            "--dependencies", self.dep_path,
            "--dest", dest_dir,
            "--ignore-stdin"
        ]

        result = subprocess.run(cmd, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, f"Init failed: {result.stderr}")

        # Check .mstl/dependency-graph.md
        dep_output = os.path.join(dest_dir, ".mstl", "dependency-graph.md")
        self.assertTrue(os.path.exists(dep_output))

        with open(dep_output, "r") as f:
            content = f.read()
            self.assertIn("repoA --> repoB", content)
            self.assertIn("repoB --> repoC", content)

    def test_init_with_invalid_dependencies(self):
        dest_dir = os.path.join(self.test_dir, "work_invalid")

        cmd = [
            self.mstl_bin, "init",
            "-f", self.config_path,
            "--dependencies", self.invalid_dep_path,
            "--dest", dest_dir,
            "--ignore-stdin"
        ]

        result = subprocess.run(cmd, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Error validating dependency graph", result.stdout)

    def test_init_missing_dependency_file(self):
        dest_dir = os.path.join(self.test_dir, "work_missing")

        cmd = [
            self.mstl_bin, "init",
            "-f", self.config_path,
            "--dependencies", os.path.join(self.test_dir, "does_not_exist.md"),
            "--dest", dest_dir,
            "--ignore-stdin"
        ]

        result = subprocess.run(cmd, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Error reading dependency file", result.stdout)

if __name__ == '__main__':
    unittest.main()
