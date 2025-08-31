# Contributing to gRPC Header Mapper

Thank you for your interest in contributing! This document outlines the process for contributing to this project.

## Development Setup

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/your-username/grpc-header-mapper.git
   cd grpc-header-mapper
   ```
3. Install dependencies:
   ```bash
   make deps
   make tools
   ```

## Making Changes

1. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes and add tests

3. Run the test suite:
   ```bash
   make ci
   ```

4. Commit your changes:
   ```bash
   git commit -m "Add: your feature description"
   ```

## Pull Request Process

1. Push to your fork and create a pull request
2. Ensure all CI checks pass
3. Add a clear description of your changes
4. Link any related issues

## Code Style

- Follow standard Go conventions
- Use `gofmt` and `goimports`
- Add comprehensive tests
- Document exported functions
- Keep commits atomic and well-described

## Testing

- Add unit tests for new functionality
- Ensure benchmark tests pass
- Aim for high test coverage
- Test edge cases and error conditions

## Documentation

- Update README.md for new features
- Add examples for complex functionality
- Document public APIs with Go comments
- Update CHANGELOG.md for releases

## Reporting Issues

When reporting issues, please include:
- Go version
- Library version
- Minimal reproduction case
- Expected vs actual behavior
- Error logs if applicable

Thank you for contributing!
