import sys
import argparse
import datetime
import traceback

GREEN = '\033[92m'
RED = '\033[91m'
RESET = '\033[0m'

def print_green(text):
    print(f"{GREEN}{text}{RESET}")

def print_red(text):
    print(f"{RED}{text}{RESET}")

class InteractiveRunner:
    def __init__(self, description):
        self.parser = argparse.ArgumentParser(description=description)
        self.parser.add_argument("-o", "--output", help="Path to append test results")
        self.parser.add_argument("--yes", action="store_true", help="Automatically answer yes to all prompts and pass --yes to mstl commands")
        self.args = None
        self.log_file = None
        self.test_name = description
        self.failed = False

    def parse_args(self):
        self.args = self.parser.parse_args()
        if self.args.output:
            self.log_file = self.args.output

    def log(self, message, status=None):
        timestamp = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        line = f"[{timestamp}] {message}"
        if status:
            line += f" - {status}"

        if status == "FAILED":
            print_red(line)
        else:
            print_green(line)

        if self.log_file:
            with open(self.log_file, "a") as f:
                f.write(line + "\n")

    def fail(self, message):
        self.log(message, status="FAILED")
        self.failed = True
        sys.exit(1)

    def print_section(self, title):
        print_green("\n" + "="*60)
        print_green(title)
        print_green("="*60)

    def ask_yes_no(self, question, default="yes", force_interactive=False):
        if self.args and self.args.yes and not self.failed and not force_interactive:
            print(f"{question} [Y/n] (Auto-Yes): yes")
            return True

        valid = {"yes": True, "y": True, "ye": True, "no": False, "n": False}
        prompt = f" [Y/n]" if default == "yes" else f" [y/N]"

        while True:
            sys.stdout.write(question + prompt + ": ")
            try:
                choice = input().lower()
            except EOFError:
                if default is not None:
                    print(default)
                    return valid[default]
                print("no")
                return False

            if default is not None and choice == "":
                return valid[default]
            elif choice in valid:
                return valid[choice]
            else:
                sys.stdout.write("Please respond with 'yes' or 'no' (or 'y'/'n').\n")

    def execute_scenario(self, title, expected_result, logic_func):
        self.print_section(title)
        print("[Expected Result]")
        print(expected_result)
        print("")

        if not self.ask_yes_no("Do you want to execute the process?"):
            self.log(f"Test '{title}' SKIPPED by user.")
            return

        print("\n[Execution]")
        try:
            logic_func()
        except Exception as e:
            print(f"Error during execution: {e}")
            traceback.print_exc()
            self.log(f"Test '{title}' FAILED (Exception)", status="FAILED")
            self.failed = True
            return

        print("\n[Verification]")
        if self.ask_yes_no("Process complete. Is the behavior as expected?"):
            self.log(f"Test '{title}' PASSED", status="SUCCESS")
        else:
            self.log(f"Test '{title}' FAILED (User Rejected)", status="FAILED")

    def run_cleanup(self, cleanup_func):
        print("\n")
        if self.ask_yes_no("Delete temporary GitHub repositories?"):
            try:
                cleanup_func()
                print("Cleanup completed.")
            except Exception as e:
                print(f"Cleanup failed: {e}")
        else:
            print("Cleanup skipped.")
