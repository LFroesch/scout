# Scout

TUI file explorer for the terminal. Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Navigate your filesystem with vim keys, search across directories, preview files, manage bookmarks, and do basic file ops without leaving the terminal.

## Install

```bash
go install github.com/LFroesch/scout@latest
```

Or grab a binary from [Releases](https://github.com/LFroesch/scout/releases). There's also an install script:

```bash
curl -sSL https://raw.githubusercontent.com/LFroesch/scout/main/install.sh | sh
```

## Usage

```bash
scout            # open in current directory
scout /some/path # open in specific directory
```

## Shell CD Integration

`ctrl+g` exits scout and cds your shell to wherever you were browsing. Add this wrapper to your `.zshrc` / `.bashrc`:

```zsh
function scout() {
  command scout "$@"
  local f="$HOME/.config/scout/last_dir"
  [ -f "$f" ] && cd "$(cat "$f")" && rm -f "$f"
}
```

Then `source ~/.zshrc`. Normal `q`/`ctrl+c` quit without changing directory.

## Keybindings

Press `?` in-app for the full list.

| Key | Action |
|-----|--------|
| `j/k`, `up/down` | Navigate |
| `enter/l/right` | Open dir/file |
| `esc/h/left` | Parent dir |
| `g/G` | First/last item |
| `ctrl+d/u` | Half-page scroll |
| `ctrl+f/b` | Full-page scroll |
| `~` | Home directory |
| `/` | Search |
| `Tab` (in search) | Cycle: Dir / Recursive / Content |
| `S` | Cycle sort: Name/Size/Date/Type |
| `.` | Toggle hidden files |
| `o` | Open in editor |
| `O` | Open current dir in editor |
| `y` | Copy path to clipboard |
| `c/x/p` | Copy/cut/paste files |
| `C/X` | Multi-file copy/cut (append) |
| `D` | Delete (with confirmation) |
| `u` | Undo delete |
| `R` | Rename |
| `N/M` | New file/directory |
| `b/B` | View/add bookmarks |
| `alt+up/down` | Scroll preview |
| `,` | Open config |
| `?` | Help |
| `q/ctrl+c` | Quit |
| `ctrl+g` | Quit + cd (see above) |

## What it does

- **Search** with `/`. Searches current dir by default, `Tab` cycles to recursive and content search (needs [ripgrep](https://github.com/BurntSushi/ripgrep)). There's also an ultra mode that searches all mounted drives.
- **File preview** in a side panel. Scrollable, cached, handles text/code/binary detection.
- **File operations**: create, rename, delete (trash-based with undo), copy/cut/paste.
- **Git awareness**: shows current branch and marks modified files with `[M]`.
- **Bookmarks** sorted by frecency (how often + how recently you visit them).
- **Configurable**: editor, search depth/limits, skip directories, hidden files default. Press `,` to edit config.

## Configuration

Config file: `~/.config/scout/scout-config.json` (press `,` to open it)

```json
{
  "skip_directories": ["node_modules", "Python*", ".cache"],
  "maxResults": 5000,
  "maxDepth": 5,
  "maxFilesScanned": 100000,
  "root_path": "/home/user",
  "bookmarks": ["/home/user/projects"],
  "show_hidden": false,
  "preview_enabled": true,
  "editor": "nvim"
}
```

| Key | What it does | Default |
|-----|-------------|---------|
| `skip_directories` | Dirs to skip in search (wildcards ok) | common dev dirs |
| `maxResults` | Max search results | 5000 |
| `maxDepth` | Recursive search depth | 5 |
| `maxFilesScanned` | Max files to scan per search | 100000 |
| `root_path` | Can't navigate above this | `$HOME` |
| `editor` | Your editor (falls back to code/vim/nano/vi) | — |

## Platform Support

Built and tested on Linux and WSL. Should work on macOS but haven't tested it. For Windows, use WSL.

## Optional Dependencies

- [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`) for content search
- `gio` or `trash-put` for trash-based delete with undo