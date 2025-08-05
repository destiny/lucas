# Lucas CLI

A powerful Go-based command-line application with interactive TUI features and cross-platform support.

## Features

- **Interactive CLI**: Beautiful terminal user interface powered by Bubble Tea
- **Hub Functionality**: Centralized management and coordination features
- **Cross-Platform**: Builds for Linux (AMD64/ARM64) and macOS (ARM64)
- **Modern Architecture**: Built with Cobra CLI framework and Bazel build system
- **Structured Logging**: Console-friendly logging with Zerolog

## Architecture

```
lucas/
├── cmd/                    # Command definitions
│   ├── cli/               # Interactive TUI components
│   ├── cli.go            # CLI subcommand
│   ├── hub.go            # Hub subcommand  
│   └── root.go           # Root command & shared logic
├── internal/
│   └── logger/           # Logging utilities
├── BUILD.bazel           # Bazel build configuration
├── MODULE.bazel          # Bazel module dependencies
├── Makefile             # Alternative build targets
└── main.go              # Application entry point
```

## Usage

### Basic Commands

```bash
# Show help
./lucas --help

# Start interactive CLI mode
./lucas cli

# Access hub functionality
./lucas hub

# Enable verbose logging
./lucas -v cli
```

### Interactive CLI Features

The `lucas cli` command launches a beautiful TUI with the following options:
- Interactive File Manager
- System Information
- Process Monitor  
- Log Viewer
- Configuration Editor

## Building

### Using Bazel (Recommended)

```bash
# Build for current platform
bazel build //:lucas

# Cross-compile for all platforms
bazel build //:lucas_linux_amd64 //:lucas_linux_arm64 //:lucas_darwin_arm64

# Run locally
bazel run //:lucas -- --help
```

### Using Make (Alternative)

```bash
# Build for current platform
make build

# Cross-compile for all platforms  
make cross-compile

# Clean build artifacts
make clean
```

### Using Go (Development)

```bash
# Build directly
go build -o lucas main.go

# Run with live reload during development
go run main.go cli
```

## Cross-Platform Binaries

Built binaries are available in:
- `bazel-bin/lucas_/lucas` (current platform)
- `bazel-bin/lucas_linux_amd64_/lucas_linux_amd64`
- `bazel-bin/lucas_linux_arm64_/lucas_linux_arm64`
- `bazel-bin/lucas_darwin_arm64_/lucas_darwin_arm64`

## Development

### Adding New Commands

1. Create new command file in `cmd/`
2. Add command to `cmd/root.go`
3. Create corresponding BUILD.bazel if needed
4. Run `bazel run //:gazelle` to update dependencies

### Adding New TUI Features  

1. Extend `cmd/cli/tui.go` with new models
2. Add new menu options to the choices array
3. Implement corresponding functionality

## Dependencies

- **Cobra**: CLI framework and command structure
- **Bubble Tea**: Terminal user interface framework
- **Lip Gloss**: ANSI styling and colors
- **Zerolog**: Structured logging with console output
- **Bazel**: Build system with cross-compilation support

## Requirements

- Go 1.23+
- Bazel 8.3+ (for Bazel builds)
- Make (for Makefile builds)

## License

This project is structured for growth and can accommodate many interactive features as the application scales.