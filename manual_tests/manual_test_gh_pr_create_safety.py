import os
import subprocess
import time
import shutil
import json
import sys
import pty
import select
import termios
import tty

# Add current directory to sys.path to import interactive_runner
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from interactive_runner import InteractiveRunner, print_green

def run_command(cmd, cwd=None, env=None):
    """Run a shell command and check for errors."""
    try:
        # Use simple subprocess for setup commands
        subprocess.run(cmd, check=True, cwd=cwd, env=env, shell=True, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
    except subprocess.CalledProcessError as e:
        print_green(f"Error running command: {cmd}")
        print_green(f"Stderr: {e.stderr.decode()}")
        sys.exit(1)

def main():
    runner = InteractiveRunner("'pr create' Safety Check Test")
    runner.parse_args()

    # Define vars to be used in cleanup
    test_dir_ptr = {"path": None}

    def cleanup():
        if test_dir_ptr["path"] and os.path.exists(test_dir_ptr["path"]):
            print_green("Cleaning up temporary directory...")
            try:
                shutil.rmtree(test_dir_ptr["path"])
            except Exception as e:
                print(f"Cleanup failed: {e}")

    def scenario_logic():
        print_green("Starting Manual Test for 'pr create' Safety Check...")

        # 1. Setup Environment
        test_dir = os.path.abspath("test_safety_env")
        test_dir_ptr["path"] = test_dir

        if os.path.exists(test_dir):
            shutil.rmtree(test_dir)
        os.makedirs(test_dir)

        print_green(f"Test directory: {test_dir}")

        # Use pre-built mstl-gh
        script_dir = os.path.dirname(os.path.abspath(__file__))
        mstl_gh_bin = os.path.abspath(os.path.join(script_dir, "../bin/mstl-gh"))
        if sys.platform == "win32":
            mstl_gh_bin += ".exe"

        if not os.path.exists(mstl_gh_bin):
            print_green(f"[ERROR] mstl-gh binary not found at {mstl_gh_bin}. Please run build_all.sh first.")
            sys.exit(1)

        print_green(f"Using mstl-gh: {mstl_gh_bin}")

        # Create Bare Remote Repo
        remote_dir = os.path.join(test_dir, "remote-a.git")
        os.makedirs(remote_dir)
        run_command("git init --bare", cwd=remote_dir)

        # Create Local Repo A
        repo_a_dir = os.path.join(test_dir, "repo-a")
        os.makedirs(repo_a_dir)
        run_command("git init", cwd=repo_a_dir)
        run_command("git config user.email 'test@example.com'", cwd=repo_a_dir)
        run_command("git config user.name 'Test User'", cwd=repo_a_dir)

        repo_url = "https://github.com/example/repo-a"
        run_command(f"git remote add origin {repo_url}", cwd=repo_a_dir)

        remote_url_file = "file://" + remote_dir.replace("\\", "/")

        # Use Local Config for insteadOf
        run_command(f"git config url.\"{remote_url_file}\".insteadOf \"{repo_url}\"", cwd=repo_a_dir)

        # Initial content and push to remote
        with open(os.path.join(repo_a_dir, "file.txt"), "w") as f:
            f.write("content 1")
        run_command("git add .", cwd=repo_a_dir)
        run_command("git branch -M main", cwd=repo_a_dir)
        run_command("git commit -m 'commit 1'", cwd=repo_a_dir)

        # Push initial state
        run_command("git push -u origin main", cwd=repo_a_dir)

        # Make Commit 2 (So we are Ahead)
        with open(os.path.join(repo_a_dir, "file.txt"), "a") as f:
            f.write("\ncontent 2")
        run_command("git add .", cwd=repo_a_dir)
        run_command("git commit -m 'commit 2'", cwd=repo_a_dir)

        # Create fake gh
        fake_gh = os.path.join(test_dir, "gh")
        gh_script_content = """#!/usr/bin/env python3
import sys
import json
import time

args = sys.argv[1:]

if "auth" in args and "status" in args:
    print("Logged in to github.com as testuser")
    sys.exit(0)

if "--version" in args:
    print("gh version 2.0.0")
    sys.exit(0)

if "pr" in args and "list" in args:
    # Simulate network delay slightly to ensure table renders first
    time.sleep(0.5)
    print("[]")
    sys.exit(0)

if "repo" in args and "view" in args:
    if "-q" in args:
        idx = args.index("-q")
        if idx + 1 < len(args):
            query = args[idx+1]
            if ".viewerPermission" in query:
                print("WRITE")
                sys.exit(0)
    print(json.dumps({"viewerPermission": "WRITE"}))
    sys.exit(0)

print("")
sys.exit(0)
"""
        with open(fake_gh, "w") as f:
            f.write(gh_script_content)
        run_command(f"chmod +x {fake_gh}")

        env = os.environ.copy()
        env["PATH"] = test_dir + os.pathsep + env["PATH"]
        env["HOME"] = test_dir

        # Create config
        config = {
            "repositories": [
                {
                    "id": "repo-a",
                    "url": repo_url,
                    "branch": "main"
                }
            ]
        }

        config_path = os.path.join(test_dir, "mistletoe.json")
        with open(config_path, "w") as f:
            json.dump(config, f)

        # Create .mstl directory and dependency-graph.md
        mstl_dir = os.path.join(test_dir, ".mstl")
        os.makedirs(mstl_dir, exist_ok=True)
        with open(os.path.join(mstl_dir, "dependency-graph.md"), "w") as f:
            f.write("graph TD\nrepo-a")

        # Run mstl-gh pr create with PTY
        print_green("Running mstl-gh pr create (--verbose)...")
        print_green("The tool will prompt 'Proceed with Push?'.")
        print_green("Wait for the script to detect this prompt and inject a change.")

        cmd = [mstl_gh_bin, "pr", "create", "-f", config_path, "--title", "Test PR", "--body", "Body", "--verbose"]

        # NOTE: This test intentionally does NOT support auto-yes because it relies on timing a race condition during the prompt.
        # If we added --yes, the prompt would be skipped instantly and we couldn't inject the change in time.
        # However, to avoid breaking the full runner with --yes, we warn if yes is set.
        if runner.args and runner.args.yes:
             print_green("[WARNING] This safety test relies on interactive timing and cannot fully automate the race condition injection with --yes.")
             # We assume --yes implies we want automation, but here automation defeats the test purpose unless we used pexpect or similar.
             # For now, we proceed but the test might fail or behave unexpectedly if it skips the prompt.
             # Actually, if we pass --yes to mstl-gh, it won't prompt, so we can't inject.
             # So we DO NOT pass --yes to the cmd here, even if runner has it.
             pass

        # Fork PTY
        master_fd, slave_fd = pty.openpty()

        process = subprocess.Popen(
            cmd,
            stdin=slave_fd,
            stdout=slave_fd,
            stderr=slave_fd,
            cwd=test_dir,
            env=env,
            close_fds=True
        )
        os.close(slave_fd) # Close slave in parent

        injected = False
        output_buffer = ""
        prompt_detected = False

        try:
            while process.poll() is None:
                # Select on master_fd and sys.stdin
                r, w, e = select.select([master_fd, sys.stdin], [], [], 0.1)

                if master_fd in r:
                    try:
                        data = os.read(master_fd, 1024)
                        if data:
                            # Forward output to user's stdout
                            os.write(sys.stdout.fileno(), data)
                            output_buffer += data.decode('utf-8', errors='replace')

                            # Check for Prompt
                            if not injected and "Proceed with Push" in output_buffer:
                                if not prompt_detected:
                                    prompt_detected = True
                                    print_green("\n[TEST] Prompt detected! Injecting race condition (new commit)...")

                                    # Inject Change! (Commit 3)
                                    with open(os.path.join(repo_a_dir, "file2.txt"), "w") as f:
                                        f.write("content 3")
                                    run_command("git add .", cwd=repo_a_dir)
                                    run_command("git commit -m 'commit 3'", cwd=repo_a_dir)

                                    print_green("[TEST] Change injected. PLEASE TYPE 'yes' TO CONTINUE.")
                                    injected = True

                    except OSError:
                        break # Process closed

                if sys.stdin in r:
                    # Forward user input to process (PTY)
                    d = os.read(sys.stdin.fileno(), 1024)
                    os.write(master_fd, d)

        except OSError:
            pass
        finally:
            os.close(master_fd)
            process.wait()

        # Check Result in Buffer
        print_green("\n--- Final Check ---")

        expected_error = "has changed since status collection"
        if expected_error in output_buffer:
            print_green("SUCCESS: Safety check triggered correctly.")
        else:
            print_green("FAILURE: Safety check did NOT trigger or message not found.")
            sys.exit(1)

    description = (
        "This test verifies that 'pr create' aborts if the repository state changes "
        "between status collection and push (race condition).\n"
        "It uses a mock 'gh' and local git repositories.\n"
        "You will interactively confirm the prompt after the script injects a change."
    )

    runner.execute_scenario(
        "Race Condition Safety Check",
        description,
        scenario_logic
    )

    runner.run_cleanup(cleanup)

if __name__ == "__main__":
    main()
