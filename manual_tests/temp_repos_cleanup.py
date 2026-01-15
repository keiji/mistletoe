#!/usr/bin/env python3
import subprocess
import json
import sys
import os
import argparse

# Add current directory to sys.path to import interactive_runner
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from interactive_runner import print_green, print_red

def run_command(args):
    try:
        result = subprocess.run(
            args,
            check=True,
            capture_output=True,
            text=True
        )
        return result.stdout.strip()
    except subprocess.CalledProcessError as e:
        print_red(f"Error running command {' '.join(args)}: {e.stderr}")
        sys.exit(1)

def get_current_user():
    return run_command(["gh", "api", "user", "--jq", ".login"])

def list_temp_repos(user):
    # Fetch list of repositories. Limit set to 1000 to cover many potential leftovers.
    # We filter client-side to ensure we match the pattern exactly.
    json_str = run_command(["gh", "repo", "list", user, "--limit", "1000", "--json", "name"])
    repos = json.loads(json_str)

    temp_repos = []
    for repo in repos:
        name = repo["name"]
        if name.startswith("mistletoe-test-"):
            temp_repos.append(name)

    return temp_repos

def delete_repo(user, repo_name):
    full_name = f"{user}/{repo_name}"
    print_green(f"Deleting {full_name}...")
    try:
        subprocess.run(["gh", "repo", "delete", full_name, "--yes"], check=True)
        print_green(f"Deleted {full_name}")
    except subprocess.CalledProcessError:
        print_red(f"Failed to delete {full_name}")

def main():
    parser = argparse.ArgumentParser(description="Cleanup temporary repositories starting with 'mistletoe-test-'")
    parser.add_argument("--yes", action="store_true", help="Automatically confirm deletion without prompting")
    args = parser.parse_args()

    print_green("Checking for 'mistletoe-test-*' repositories...")

    try:
        user = get_current_user()
    except Exception:
        print_red("Failed to get GitHub user. Ensure 'gh' is authenticated.")
        sys.exit(1)

    temp_repos = list_temp_repos(user)

    if not temp_repos:
        print_green("No 'mistletoe-test-*' repositories found.")
        return

    print_green(f"\nFound {len(temp_repos)} repository(s):")
    for repo in temp_repos:
        print_green(f" - {repo}")

    if args.yes:
        print_green("\nDo you want to DELETE all these repositories? (yes/no) [Auto-Yes]: yes")
        choice = "yes"
    else:
        print_green("\nDo you want to DELETE all these repositories? (yes/no)")
        choice = input("> ").lower().strip()

    if choice == "yes":
        print_green("\nStarting cleanup...")
        for repo in temp_repos:
            delete_repo(user, repo)
        print_green("\nCleanup complete.")
    else:
        print_green("Operation cancelled.")

if __name__ == "__main__":
    main()
