
```sh
go mod init gorae
go mod tidy
```

Build a binary

```sh
go build -o gorae
./gorae
./gorae -root ~/Documents/Papers
```



```sh
sudo pacman -S noto-fonts-emoji
```

```sh
sudo apt install fonts-noto-color-emoji
```

## Configuration

On first run the app writes `~/.config/gorae/config.json` (or `${XDG_CONFIG_HOME}/gorae/config.json`). Edit it via `:config edit` to tweak paths and behavior. Two useful keys:

- `editor`: command used when pressing `:config edit` or editing metadata
- `pdf_viewer`: command used to open PDFs. Provide the binary plus optional arguments; the PDF path is appended automatically. Quotes are supported and required if your command contains spaces, e.g. `"pdf_viewer": "\"C:\\\\Program Files\\\\SumatraPDF\\\\SumatraPDF.exe\""`
- `notes_dir`: directory where per-PDF note files are stored (defaults to `${meta_dir}/notes`). Files are regular text/Markdown so you can sync or back them up separately.

## Metadata

- Press `e` to preview metadata, `e` again to edit inline, or `v` to open the structured fields in your configured editor.
- Press `n` while in the metadata popup to open the note for the current file in your editor (notes are stored as Markdown files in `notes_dir`).
- Metadata fields include Title, Author, Journal/Conference, Year, Tag, and Abstract. Notes are stored separately.
- In the metadata popup use ↑/↓ or PgUp/PgDn to scroll through long content.

TODO
- arxiv command with selections
- Search function
- [Text extraction, pymupdf4llm](https://pymupdf.readthedocs.io/en/latest/pymupdf4llm/)
- Yank bibtex / line style
- Bookmark / Favorite
- Page count
- UI improvement
- logo command
- Command autocomplete
- Screen renew or update key or auto

AI features:
- AI tag
- 
