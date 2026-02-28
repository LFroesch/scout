# Scout

A TUI file explorer and fuzzy finder built in Go. Vim-style navigation, fuzzy/recursive/content search, file operations, git integration, and live preview.

## Install

```bash
# Install script (auto-detects OS/arch)
curl -sSL https://raw.githubusercontent.com/LFroesch/scout/main/install.sh | sh

# Or with Go
go install github.com/LFroesch/scout@latest
```

Pre-compiled binaries available on [GitHub Releases](https://github.com/LFroesch/scout/releases) for Linux, macOS, and Windows (amd64/arm64).

## Usage

```bash
scout            # current directory
scout /some/path # specific directory
```

## Shell CD Integration

Press `ctrl+g` inside scout to exit and have your shell `cd` to the directory you were browsing. Requires a shell wrapper — add this to `~/.zshrc` or `~/.bashrc`:

```zsh
function scout() {
  command scout "$@"
  local f="$HOME/.config/scout/last_dir"
  [ -f "$f" ] && cd "$(cat "$f")" && rm -f "$f"
}
```

Then `source ~/.zshrc` (or open a new terminal). After that, `ctrl+g` in scout will drop you into the current directory when it exits. Normal `q`/`ctrl+c` quit without changing directory.

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
| `/` | Search (fuzzy) |
| `Tab` (in search) | Cycle: Dir / Recursive / Content |
| `S` | Cycle sort: Name/Size/Date/Type |
| `.` | Toggle hidden files |
| `o` | Open (file: editor/default app, dir: VS Code) |
| `O` | Open current directory in VS Code |
| `y` | Copy path to clipboard |
| `c/x/p` | Copy/cut/paste files (replace clipboard) |
| `C/X` | Append to copy/cut clipboard (multi-file) |
| `D` | Delete (confirmation) |
| `u` | Undo delete (trash) |
| `R` | Rename |
| `N/M` | New file/directory |
| `b/B` | View/add bookmarks |
| `alt+up/down` | Scroll preview |
| `,` | Open config |
| `?` | Help |
| `q/ctrl+c` | Quit |
| `ctrl+g` | Quit and cd to current dir (shell integration) |

## Features

- **Search** -- Fuzzy match in current dir, recursive across project tree, or content search via ripgrep. Results highlighted with match chars.
- **File operations** -- Create, rename, delete (with undo via trash), copy/cut/paste, path copy to clipboard.
- **Preview** -- Live file preview with scroll support. Categorizes text, media, archives, binaries, etc. LRU cached (50 files), skips files >1MB.
- **Git** -- Branch display in header, `[M]` markers on modified files.
- **Bookmarks** -- Frecency-sorted (frequency + recency). Press `b` to view, `B` to add.
- **File type icons** -- Visual indicators per language/type.
- **Configurable** -- Editor, root path boundary, skip directories (with wildcards), search limits, hidden files default.

## Configuration

Config lives at `~/.config/scout/scout-config.json` (press `,` to edit):

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

| Key | Description | Default |
|-----|-------------|---------|
| `skip_directories` | Dirs to skip in search (supports wildcards) | common dev dirs |
| `maxResults` | Max search results (100-50000) | 5000 |
| `maxDepth` | Recursive search depth (1-20) | 5 |
| `maxFilesScanned` | Max files per search (1000-1000000) | 100000 |
| `root_path` | Navigation ceiling | `$HOME` |
| `editor` | Editor for `e` key | system default |

## Platform Support

| Platform | Status |
|----------|--------|
| Linux | Supported, actively tested |
| WSL | Supported, actively tested |
| macOS | Should work, untested |
| Windows | Use WSL |

## Optional Dependencies

- **ripgrep** (`rg`) -- Needed for content search (`/` then `Tab` twice)
- **gio** or **trash-put** -- Enables trash-based deletion with undo (`u`)