# Scout

TUI file explorer for the terminal. Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Navigate your filesystem with vim keys, search across directories, preview files, manage bookmarks, and do basic file ops without leaving the terminal.

## Quick Install

Supported platforms: Linux and macOS. On Windows, use WSL.

Recommended (installs to `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/LFroesch/scout/main/install.sh | bash
```

Or download a binary from [GitHub Releases](https://github.com/LFroesch/scout/releases).

Or install with Go:

```bash
go install github.com/LFroesch/scout@latest
```

Or build from source:

```bash
make install
```

Command:

```bash
scout
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
| `f` | Navigate to dir (or parent of file) |
| `g/G` | First/last item |
| `ctrl+d/u` | Half-page scroll |
| `ctrl+f/b` | Full-page scroll |
| `~` | Home directory |
| `` ` `` | Jump to /mnt/c (WSL) or / (Linux) |
| `/` | Search |
| `Tab` (in search) | Cycle: Dir / Recursive / Content / Ultra |
| `ctrl+p` (in search) | Toggle preview panel |
| `ctrl+n` (in search) | Toggle name-only / full-path search |
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
| `r` | Refresh current view |
| `b/B` | View/add bookmarks |
| `w/s`, `alt+up/down` | Scroll preview |
| `,` | Open config |
| `?` | Help |
| `q/ctrl+c` | Quit |
| `ctrl+g` | Quit + cd (see above) |

## What it does

- **Search** with `/`. `Tab` cycles through four modes: current dir, recursive, content search (needs [ripgrep](https://github.com/BurntSushi/ripgrep)), and ultra (all mounted drives). Press `Enter` to lock results for navigation, then browse/open files without losing your search.
- **File preview** in a side panel. Scrollable, cached, handles text/code/binary detection.
- **File operations**: create, rename, delete (trash-based with undo), copy/cut/paste. Multi-file clipboard with `C`/`X`.
- **Git awareness**: shows current branch and marks modified files with `[M]`.
- **Bookmarks** sorted by frecency (how often + how recently you visit them).
- **Configurable**: editor, search depth/limits, skip directories, hidden files default. Press `,` to edit config.
- **Mouse support**: single-click selects, double-click opens, middle-click navigates to directory, scroll wheel works everywhere.

## Configuration

Config file: `~/.config/scout/scout-config.json` (press `,` to open it)

```json
{
  "skip_directories": ["node_modules", "Python*", ".cache"],
  "maxResults": 5000,
  "maxDepth": 5,
  "maxFilesScanned": 100000,
  "root_path": "",
  "bookmarks": ["/home/user/projects"],
  "show_hidden": true,
  "preview_enabled": true
}
```

| Key | What it does | Default |
|-----|-------------|---------|
| `skip_directories` | Dirs to skip in search. Supports exact names (`node_modules`), wildcards (`Python*`), and absolute paths (`/usr/bin`) | ~60 common dirs |
| `maxResults` | Max search results | 5000 |
| `maxDepth` | Recursive search depth | 5 |
| `maxFilesScanned` | Max files to scan per search | 100000 |
| `root_path` | Can't navigate above this (empty = no limit) | `""` |
| `show_hidden` | Show dotfiles by default | `true` |
| `preview_enabled` | Show preview panel on startup | `true` |

### Editor

Scout uses `$VISUAL` → `$EDITOR` → probes for `code`, `vim`, `nano`, `vi`.

```bash
export EDITOR=nvim      # terminal editor
export VISUAL=cursor    # GUI editor (checked first)
```

Add to your `~/.zshrc` or `~/.bashrc` and it will work across all TUI apps.

## Platform Support

Built and tested on Linux and WSL. Should work on macOS but haven't tested it. For Windows, use WSL.

## Optional Dependencies

- [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`) for content search
- `gio` or `trash-put` for trash-based delete with undo

## License

[AGPL-3.0](LICENSE)
