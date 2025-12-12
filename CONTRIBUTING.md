# Contributing to StreamArr

Thank you for your interest in contributing to StreamArr! 🎉

## How to Contribute

### Reporting Bugs

1. Check if the bug has already been reported in [Issues](https://github.com/Zerr0-C00L/StreamArr/issues)
2. If not, create a new issue using the **Bug Report** template
3. Include as much detail as possible (logs, screenshots, steps to reproduce)

### Suggesting Features

1. Check if the feature has already been requested
2. Create a new issue using the **Feature Request** template
3. Explain the use case and why it would be useful

### Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Test your changes thoroughly
5. Commit with clear messages (`git commit -m 'Add amazing feature'`)
6. Push to your branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Development Setup

```bash
# Clone your fork
git clone https://github.com/YOUR-USERNAME/StreamArr.git
cd StreamArr

# Start with Docker (recommended for development)
docker compose up -d

# Or manual setup
cp .env.example .env
# Edit .env with your settings
go build -o bin/server cmd/server/main.go
./bin/server
```

### Code Style

- Go: Follow standard Go formatting (`go fmt`)
- TypeScript/React: Follow existing patterns in the codebase
- Commit messages: Clear and descriptive

## Questions?

Feel free to open a [Question issue](https://github.com/Zerr0-C00L/StreamArr/issues/new?template=question.md) or start a [Discussion](https://github.com/Zerr0-C00L/StreamArr/discussions).

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
