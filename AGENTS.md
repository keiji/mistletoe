# Communication Guidelines

*   **Language**:
    *   Prioritize **Japanese** for basic interactions, and include **English** as well.
    *   For **Pull Requests and Comments**, all interactions must be in **Japanese**.

# Development Guidelines

## Code Style & Design

*   **Google Go Style Guide**: The code must strictly follow the [Google Go Style Guide](https://google.github.io/styleguide/go/).
    *   Use `MixedCaps` for naming.
    *   Package names should be short, lowercase, and singular.
    *   Error strings should not be capitalized (unless beginning with proper nouns or acronyms) and should not end with punctuation.
*   **General Go Software Design**:
    *   Avoid monolithic packages. Use the `internal/` directory to organize code into logical packages (e.g., `git`, `gh`, `config`, `ui`).
    *   Avoid `utils` or `common` packages. instead, name packages by their domain.
    *   Keep interfaces small and defined where they are used.
*   **Testability**:
    *   Design code to be testable. Use dependency injection or mockable variables (like `ExecCommand`) for external interactions (Git, filesystem, etc.).
    *   Ensure tests cover edge cases and error conditions.
*   **Unit Tests (Git Configuration)**:
    *   **Must Configure Git User**: When writing unit tests that involve Git operations (even mocked or local), you **must** explicitly configure `user.email` and `user.name` to ensure reproducible behavior across environments.
    *   Example:
        ```go
        exec.Command("git", "-C", dir, "config", "user.email", "test@example.com").Run()
        exec.Command("git", "-C", dir, "config", "user.name", "Test User").Run()
        ```
*   **Manual Tests**:
    *   Refer to `manual_tests/index.md` for manual test implementation guidelines.
*   **Code Comments**:
    *   **No Change History**: Do not include comments about the change history or actions taken in the code (e.g., "Added this call", "Fixed bug", "Modified by ..."). The Git history is the source of truth for changes. Comments should focus on "why" and "how" the code works, not "what changed".

# Pull Request Guidelines

Before creating a Pull Request, ensure that you verify not only that the build succeeds but also that the `revive` checks pass. **This is a strict requirement; the CI process will fail if any linting errors are detected.**

*   **Linting Requirements**: The `revive` settings must comply with the GitHub Actions workflow.
    *   This project uses `golangci-lint` with `revive` enabled (e.g., `args: --enable=revive --tests=false`).
    *   **Zero Tolerance**: All linting errors, including "unused" variables or functions, must be resolved before submission.
    *   Test files (`_test.go`) are excluded from linting.
*   **Test Requirements**:
    *   **Strict Rule: Commits are only allowed when ALL unit tests PASS. No commit is permitted if even one unit test fails. This is a top priority.**
    *   You **MUST** ensure that all tests pass before committing code.
    *   Run `go test -v ./...` to verify functionality locally.
    *   **Environment Stability**: Be aware that CI environments may have different characteristics (e.g., available stdin/stdout). Ensure tests are robust against these differences (e.g., by mocking standard input/output or using flags like `--ignore-stdin` where appropriate).
    *   **Verification**: Do not assume success based on partial logs. Always verify the final exit code and the full test summary. If a test helper captures exit codes (e.g., mocking `os.Exit`), ensure it correctly propagates these codes using named return values or explicit assignment to avoid false positives.

# Quality Assurance & Process

*   **Self-Correction**: If a test failure occurs, identify the root cause (logic bug, test bug, or environment issue) and fix it *before* asking for another review. Do not submit code that you know or suspect might fail.
*   **Cost Awareness**: Repeated cycles of "submit -> fail -> fix" are costly. Strive for "first-time right" by running comprehensive local verification.
*   **Commit Content**: Ensure every commit contains actual, meaningful changes. Do not push empty or redundant commits.

## How to run revive (for Agents)

To run `revive` in the sandbox environment:

1.  **Install revive**:

    ```bash
    go install github.com/mgechev/revive@latest
    ```

2.  **Use the provided configuration file** (`revive.toml`) in the root directory. This file is configured to match the project's settings (enabling standard rules but disabling `error-strings`).

3.  **Run revive**:

    ```bash
    $(go env GOPATH)/bin/revive -config revive.toml -exclude "**/*_test.go" ./...
    ```

## Reference Style Guides

*   [Material Design Writing Best Practices](https://m3.material.io/foundations/writing/best-practices)
*   [Digital.gov Style Guide](https://digital.gov/style-guide)
*   [Google Go Style Guide](https://google.github.io/styleguide/go/)

## Documentation Guidelines

*   **Language**: All design documents (e.g., in `docs/`) must be written in **Japanese**.
*   **Writing Style (Tone)**:
    *   Use the polite **"Desu/Masu" (です・ます)** style for main body text.
    *   For **bullet points (lists)**, use **"Taigendome" (noun-ending/incomplete sentence)** style and **do not use punctuation (periods)** at the end.
*   **Mermaid Diagrams**:
    *   Node IDs must be descriptive (e.g., `CheckEnv`, `ParseBlock`) rather than single letters (`A`, `B`).
    *   Japanese text within diagrams must be enclosed in double quotes (e.g., `id["日本語テキスト"]`).
