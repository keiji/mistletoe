# Communication Guidelines

*   **Language**: Prioritize **Japanese** for basic interactions, and include **English** as well.

# Pull Request Guidelines

Before creating a Pull Request, ensure that you verify not only that the build succeeds but also that the `revive` checks pass. **This is a strict requirement; the CI process will fail if any linting errors are detected.**

*   **Linting Requirements**: The `revive` settings must comply with the GitHub Actions workflow.
    *   This project uses `golangci-lint` with `revive` enabled (e.g., `args: --enable=revive --tests=false`).
    *   **Zero Tolerance**: All linting errors, including "unused" variables or functions, must be resolved before submission.
    *   Test files (`_test.go`) are excluded from linting.

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

## Documentation

*   **Design Documents**: All design documents (e.g., in `docs/`) must be written in **Japanese**.
