# Gorae

![Gorae logo](gorae.svg)

Gorae (*고래*, *whale*) is a cozy TUI librarian for your PDFs. It watches a folder, keeps metadata in sync, and gives you quick search/favorite/to-read queues so you can swim through papers with joy. I hope the app makes your reading life calmer and more fun.

**Highlights**
- Fast file browser with metadata-aware favorites, to-read list, and reading states.
- Search across content or metadata with instant previews/snippets.
- In-app metadata editor, arXiv importer, and BibTeX copy.
- Themeable UI (colors, glyphs, borders) plus helper folders you can browse in any file manager.

## Quick install

1. Install Go 1.21+ from [go.dev](https://go.dev/dl/).
2. Clone this repository:

   ```sh
   git clone https://github.com/Han8931/gorae.git
   cd gorae
   ```

3. Run the helper script (default path: `~/.local/bin/gorae` on Linux, `/usr/local/bin/gorae` on macOS):

   ```sh
   ./install.sh
   # choose another destination via env var or first argument
   GORAE_INSTALL_PATH=/usr/local/bin/gorae ./install.sh
   ./install.sh ~/bin/gorae
   ```

Once the script finishes, ensure the destination directory is on your `PATH`, then launch the app with:

```sh
gorae        # optionally: gorae -root /path/to/Papers
```

## Platform support

Gorae is tested on macOS (Apple Silicon + Intel) and common Linux distros including Arch Linux and Debian/Ubuntu. As long as Go 1.21+ and the Poppler CLI tools (`pdftotext`, `pdfinfo`) are available, the TUI runs the same everywhere. Use `brew install golang poppler`, `sudo apt install golang-go poppler-utils`, or `sudo pacman -S go poppler` to grab the prerequisites on your platform.

## Manual install

```sh
git clone https://github.com/Han8931/gorae.git
cd gorae

# Install to $(go env GOPATH)/bin so it is available everywhere
go install ./cmd/gorae
export PATH="$(go env GOPATH)/bin:$PATH"

# or build/copy to a directory you manage
go build -o gorae ./cmd/gorae
install -Dm755 gorae ~/.local/bin/gorae   # adjust destination as needed
```

After the binary is on `PATH`, launch `gorae` from any folder (pass `-root /path/to/Papers` to point at a different library). One command is all it takes to start browsing your collection.

## Everyday use

- `f` = toggle Favorite, `t` = toggle To-read, `r` = cycle reading state.
- `y` copies BibTeX, `n` edits notes, `e`/`v` open metadata editors.
- `/` searches, `F`/`T`/`g r|u|d` open the smart lists (favorites, to-read, reading states).
- `:theme reload` reapplies your `theme.toml` changes, `:theme show` prints the active theme path.
- `:help` inside the app lists every command.

For deeper instructions (config, themes, metadata, search tips, helper folders, etc.) read **[docs/user-guide.md](docs/user-guide.md)**. Prefer a different look? Grab one of the ready-made themes in `themes/` (e.g., `aurora.toml`, `matcha.toml`, `fancy-dark.toml`) and point `config.theme_path` at it or copy it to `~/.config/gorae/theme.toml`.

TODO
- logo command

AI features:
- [Text extraction, pymupdf4llm](https://pymupdf.readthedocs.io/en/latest/pymupdf4llm/)
- AI tag
- AI Summary
- Extract texts
- Knowledge Graphs
- RAG
- Prompt management

## [Bangudae Petroglyphs](https://en.wikipedia.org/wiki/Bangudae_Petroglyphs)

[Pictures](https://www.khan.co.kr/article/202007080300025)
The world's earliest known depictions of whale hunting are found in the Bangudae Petroglyphs in South Korea, dating back around 7,000 years (6,000 BC), showcasing detailed scenes of boats and harpoons; however, similar ancient whale art is also found in the White Sea region (Russia/Scandinavia) and Norway, possibly as old, depicting complex hunts and spiritual meanings beyond simple prey, suggesting widespread ancient maritime cultures. 

## Acknowledgement
