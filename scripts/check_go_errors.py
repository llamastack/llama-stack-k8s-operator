#!/usr/bin/env python3

import sys
import re

# This script checks that error messages in fmt.Errorf calls start with a
# specific wording to ensure consistency.

REQUIRED_WORDING = "failed to"

# We compile the regex for efficiency. This pattern looks for:
# - fmt.Errorf
# - Followed by ("
# - It then captures the content inside the double quotes.
ERROR_PATTERN = re.compile(r'fmt\.Errorf\("([^"]+)"')

def check_file(filepath: str) -> list[str]:
    """
    Checks a single file for fmt.Errorf message violations using regex.

    Args:
        filepath: The path to the Go file to check.

    Returns:
        A list of strings, where each string is a formatted error line.
    """
    invalid_lines = []
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            for i, line in enumerate(f, 1):
                # search() finds the first occurrence of the pattern in the line.
                match = ERROR_PATTERN.search(line)
                if match:
                    # group(1) contains the content of the first capturing group,
                    # which is the error message string itself.
                    error_message = match.group(1)
                    if not error_message.startswith(REQUIRED_WORDING):
                        invalid_lines.append(f"{i}:{line.strip()}")

    except FileNotFoundError:
        print(f"Error: File not found: {filepath}", file=sys.stderr)
        return [f"0:File not found: {filepath}"]
    except Exception as e:
        print(f"Error processing file {filepath}: {e}", file=sys.stderr)
        return [f"0:Error processing file: {e}"]

    return invalid_lines

def main():
    """
    Main function to process command-line arguments and run checks.
    """
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <file1.go> <file2.go> ...")
        sys.exit(1)

    exit_code = 0
    filepaths = sys.argv[1:]

    for path in filepaths:
        errors = check_file(path)
        if errors:
            exit_code = 1
            print(f"Error in {path}: The following error messages do not start with '{REQUIRED_WORDING}':")
            for error_line in errors:
                print(error_line)
            print("")

    sys.exit(exit_code)

if __name__ == "__main__":
    main()
