# Lucas CLI

A powerful Go-based command-line application with interactive TUI features and cross-platform support.

## Features

- **Interactive CLI**: Beautiful terminal user interface powered by Bubble Tea
- **Hub Functionality**: Centralized management and coordination features
- **Sony Bravia Control**: Remote control and API integration for Sony Bravia TVs
- **Cross-Platform**: Builds for Linux (AMD64/ARM64) and macOS (ARM64)
- **Modern Architecture**: Built with Cobra CLI framework and Bazel build system
- **Structured Logging**: Console-friendly logging with Zerolog

## Architecture

```
lucas/
├── cmd/                    # Command definitions
│   ├── cli/               # Interactive TUI components
│   ├── bravia.go         # Sony Bravia TV control
│   ├── cli.go            # CLI subcommand
│   ├── hub.go            # Hub subcommand  
│   └── root.go           # Root command & shared logic
├── internal/
│   ├── bravia/           # Sony Bravia TV client
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

# Sony Bravia TV Control
./lucas bravia remote power --host 192.168.1.100:80 --credential 0000
./lucas bravia control power-status --host 192.168.1.100:80 --credential 0000
```

### Interactive CLI Features

The `lucas cli` command launches a beautiful TUI with the following options:
- Interactive File Manager
- System Information
- Process Monitor  
- Log Viewer
- Configuration Editor

### Sony Bravia TV Control

Control Sony Bravia TVs using IRCC remote commands and JSON API:

```bash
# List available remote control codes
./lucas bravia list remote

# Send remote control commands
./lucas bravia remote power --host 192.168.1.100:80 --credential 0000
./lucas bravia remote volume-up --host 192.168.1.100:80 --credential 0000
./lucas bravia remote hdmi1 --host 192.168.1.100:80 --credential 0000

# List available control API methods  
./lucas bravia list control

# Send control API commands
./lucas bravia control power-status --host 192.168.1.100:80 --credential 0000
./lucas bravia control volume-info --host 192.168.1.100:80 --credential 0000
./lucas bravia control system-info --host 192.168.1.100:80 --credential 0000

# Enable debug logging for troubleshooting
./lucas bravia remote power --host 192.168.1.100:80 --credential 0000 --debug
```

**Supported Features:**
- **Remote Control**: Power, volume, channels, navigation, input switching
- **System Control**: Power status, volume info, system information
- **Content Management**: Playing content info, app list, content list
- **Authentication**: PSK (Pre-Shared Key) support
- **Debug Mode**: Request/response logging for troubleshooting

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