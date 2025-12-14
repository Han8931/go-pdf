# Gorae

<p align="center">
  <img src="gorae.svg" alt="Gorae logo" width="180">
</p>

**Gorae** (*고래*, *whale*) is a cozy TUI librarian for your PDFs—built for Vim/CLI/TUI lovers who want to stay in the terminal, keep metadata in sync, and enjoy quick search plus favorite/to-read queues.

> The Gorae logo is inspired by the Bangudae Petroglyphs (반구대 암각화) in Ulsan, South Korea—one of the earliest known depictions of whales and whale hunting. The simple "glyph-like" whale shape is meant to feel like an engraving: minimal, timeless, and a little handmade—just like a cozy terminal app.

## Highlights

- Fast file browser with metadata-aware favorites, to-read list, and reading states.
- Search across content or metadata with instant previews/snippets.
- In-app metadata editor, arXiv importer, and BibTeX copy.
- Themeable UI (colors, glyphs, borders) plus helper folders you can browse in any file manager.

## Demo

<!-- TODO: Add a screenshot / GIF / asciinema link -->

## Install

### Requirements

**Required**
- Go 1.21+
- Poppler CLI tools: `pdftotext`, `pdfinfo`

**Optional (recommended)**
- A fast PDF viewer (Zathura recommended below)
- OCR / AI features (planned)

Install prerequisites:
- macOS: `brew install golang poppler`
- Debian/Ubuntu: `sudo apt install golang-go poppler-utils`
- Arch: `sudo pacman -S go poppler`

### Quick install (script)

1. Clone this repository:

```sh
git clone https://github.com/Han8931/gorae.git
cd gorae
```

2. Run the helper script (default path: `~/.local/bin/gorae` on Linux, `/usr/local/bin/gorae` on macOS):

```sh
./install.sh

# or choose another destination via env var or first argument
GORAE_INSTALL_PATH=/usr/local/bin/gorae ./install.sh
./install.sh ~/bin/gorae
   ```

3. Ensure the destination directory is on your `PATH`, then launch:

```sh
gorae        # optionally: gorae -root /path/to/Papers
```

### Manual install

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

After the binary is on `PATH`, launch `gorae` from any folder (pass `-root /path/to/Papers` to point at a different library).

## Everyday use

> For deeper instructions, read **[docs/user-guide.md](docs/user-guide.md)** or run `:help`.

| Action             | Key       |
| ------------------ | --------- |
| Move               | `j/k`     |
| Enter dir / up     | `l` / `h` |
| Select             | `Space`   |
| Favorite / To-read | `f` / `t` |
| Reading state      | `r`       |
| Search             | `/`       |
| Help               | `:help`   |

> Arrow keys are also supported.

### Search tips

Search (`/`) with flags like:

* `-t [title]`
* `-a [author]`
* `-y [year]`
* `-c [content]`

## Config & themes

Gorae stores configuration and user data in standard locations:

* Config + theme:
  * `~/.config/gorae/`
  * `~/.config/gorae/theme.toml`
* Data (metadata DB, notes, cache):
  * `~/.local/share/gorae/`

You can open and edit the config from inside the app using `:config`.

If you prefer a different look, pick one of the ready-made themes in `themes/` (e.g., `aurora.toml`, `matcha.toml`, `fancy-dark.toml`) and set `theme_path` in the config (via `:config`), or copy a theme file to:

```sh
cp themes/matcha.toml ~/.config/gorae/theme.toml
```

## Recommended PDF viewer

Gorae works with any viewer command, but we recommend [Zathura](https://pwmt.org/projects/zathura/) with the MuPDF backend. Zathura is minimal, keyboard-driven, starts instantly, supports vi-style navigation, and renders beautifully through MuPDF—great for tiling window managers.

Install:

* Debian/Ubuntu: `sudo apt install zathura zathura-pdf-mupdf`
* Arch: `sudo pacman -S zathura zathura-pdf-mupdf`

Then set the viewer command in your config:

```json
"pdf_viewer": "zathura"
```

If `zathura` is on your `PATH`, Gorae will auto-detect it, so most users can accept the default.

## Roadmap

* [ ] Update and revise README and manual
* [ ] `gorae logo` command

### AI features (planned)

* AI tagging and summarization
* Text extraction (OCR) (see: [https://pymupdf.readthedocs.io/en/latest/pymupdf4llm/](https://pymupdf.readthedocs.io/en/latest/pymupdf4llm/))
* RAG and knowledge graphs
* Prompt management

## Uninstall

1. Delete the binary you installed (default `~/.local/bin/gorae` on Linux or `/usr/local/bin/gorae` on macOS).
2. Remove the config/data folders if you no longer need them:

   ```sh
   rm -rf ~/.config/gorae        # config + theme
   rm -rf ~/.local/share/gorae   # metadata store, notes, db
   ```

That's it—you can re-clone and reinstall at any time.

## Acknowledgements

<!-- TODO: Add libraries/tools you use (Bubble Tea, Bubbles, Lip Gloss, Poppler, etc.) -->

