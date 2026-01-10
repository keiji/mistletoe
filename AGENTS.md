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
*   **Manual Tests**:
    *   Refer to `manual_tests/index.md` for manual test implementation guidelines.

# Pull Request Guidelines

Before creating a Pull Request, ensure that you verify not only that the build succeeds but also that the `revive` checks pass. **This is a strict requirement; the CI process will fail if any linting errors are detected.**

*   **Linting Requirements**: The `revive` settings must comply with the GitHub Actions workflow.
    *   This project uses `golangci-lint` with `revive` enabled (e.g., `args: --enable=revive --tests=false`).
    *   **Zero Tolerance**: All linting errors, including "unused" variables or functions, must be resolved before submission.
    *   Test files (`_test.go`) are excluded from linting.
*   **Test Requirements**:
    *   You **MUST** ensure that all tests pass before committing code.
    *   Run `go test -v ./...` to verify functionality locally.

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
