# Pull Request Guidelines

Before creating a Pull Request, ensure that you verify not only that the build succeeds but also that the `golint` checks pass.

*   **Linting Requirements**: The `golint` settings must comply with the GitHub Actions workflow.
    *   This project uses `golangci-lint` with `revive` enabled (e.g., `args: --enable=revive --tests=false`).
    *   Test files (`_test.go`) are excluded from linting.

## Reference Style Guides

*   [Material Design Writing Best Practices](https://m3.material.io/foundations/writing/best-practices)
*   [Digital.gov Style Guide](https://digital.gov/style-guide)
