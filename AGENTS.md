# Communication Guidelines

*   **Language**: Prioritize **Japanese** for basic interactions, and include **English** as well.

# Pull Request Guidelines

Before creating a Pull Request, ensure that you verify not only that the build succeeds but also that the `golint` checks pass. **This is a strict requirement; the CI process will fail if any linting errors are detected.**

*   **Linting Requirements**: The `golint` settings must comply with the GitHub Actions workflow.
    *   This project uses `golangci-lint` with `revive` enabled (e.g., `args: --enable=revive --tests=false`).
    *   **Zero Tolerance**: All linting errors, including "unused" variables or functions, must be resolved before submission.
    *   Test files (`_test.go`) are excluded from linting.

## Reference Style Guides

*   [Material Design Writing Best Practices](https://m3.material.io/foundations/writing/best-practices)
*   [Digital.gov Style Guide](https://digital.gov/style-guide)

## Documentation

*   **Design Documents**: All design documents (e.g., in `docs/`) must be written in **Japanese**.
