# Contributing to RUSEON Core

Thank you for your interest in contributing to RUSEON Core! We welcome contributions from the community to help make our Cloud-Native Video Data Platform even better.

## How to Contribute

1. **Fork and Branch**: Fork the repository and create a new branch from `dev` (e.g., `feat/new-feature` or `fix/bug-name`).
2. **Discuss first**: For major features or breaking changes, please open an Issue to discuss your ideas before writing code.
3. **Write Code**: Implement your changes. Ensure you adhere to the project's coding standards.
4. **Test**: Write tests for your changes. Run `go test -race ./...` to ensure everything passes.
5. **Lint**: Run `golangci-lint run` to ensure your code matches our style guidelines.
6. **Commit**: Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) for your commit messages.
7. **Submit PR**: Open a Pull Request targeting the `dev` branch.

## Commit Message Guidelines

We follow the Conventional Commits specification. Please structure your commit messages as follows:

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Allowed types:**
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `style`: Changes that do not affect the meaning of the code (white-space, formatting, etc.)
- `refactor`: A code change that neither fixes a bug nor adds a feature
- `perf`: A code change that improves performance
- `test`: Adding missing tests or correcting existing tests
- `chore`: Changes to the build process or auxiliary tools and libraries

## Code of Conduct

Please note that this project is released with a Contributor Code of Conduct. By participating in this project you agree to abide by its terms.
