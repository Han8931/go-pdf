# Local Binary Builds (Not for Git)

These notes walk through building standalone binaries for each supported platform without requiring Go on the target machine.

## Prerequisites

- Go 1.21+ and the project dependencies installed on this machine.
- Poppler CLI tools (`pdftotext`, `pdfinfo`) available so you can test the Linux build; end users still need these tools, but not Go.

From the repo root (`/home/han/Projects/go-pdf`):

```sh
mkdir -p dist
```

## Build Per Platform

1. **Linux (amd64)**
   ```sh
   go build -o dist/gorae ./cmd/gorae
   ./dist/gorae -help     # sanity-check
   ```
2. **macOS (Intel)**
   ```sh
   GOOS=darwin GOARCH=amd64 go build -o dist/gorae-darwin-amd64 ./cmd/gorae
   ```
3. **macOS (Apple Silicon)**
   ```sh
   GOOS=darwin GOARCH=arm64 go build -o dist/gorae-darwin-arm64 ./cmd/gorae
   ```
4. **Windows (amd64)**
   ```sh
   GOOS=windows GOARCH=amd64 go build -o dist/gorae-windows-amd64.exe ./cmd/gorae
   ```

## Distribute

- Share the files inside `dist/` that match each user's platform. Compress them if needed.
- Users copy the binary to a directory on their `PATH` (e.g., `~/.local/bin`, `/usr/local/bin`, `%USERPROFILE%\\bin`) and run `gorae -root /path/to/Papers`.
- No Go toolchain required on the target system, but Poppler tools must be installed for metadata/text extraction features.

*(Keep this file out of version control; see `.gitignore`.)*
