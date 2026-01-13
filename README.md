# 🦅 Scout

A lightning-fast TUI file explorer and fuzzy finder for developers. Navigate, search, and preview files with ease.

## Features

### Core File Management
- **📁 Full File Operations** - Delete, rename, create files/directories with confirmation dialogs
- **✂️ Copy/Cut/Paste** - Clipboard operations for files and directories (c, x, p)
- **🔄 Undo** - Restore last deleted file from trash (press u)
- **🗑️ Safe Deletion** - Move to trash when possible, with confirmation dialogs
- **📊 Smart Sorting** - Cycle through sort modes: Name, Size, Date, Type (press s)
- **📐 File Info Display** - Shows file sizes, git status, and symlink indicators

### Advanced Search
- **⚡ Advanced Fuzzy Search** - Lightning-fast fuzzy matching with result highlighting
- **🔄 Recursive Search** - Search across entire project tree (toggle with Tab in search mode)
- **🔍 Content Search** - Search inside files with ripgrep integration (Ctrl+G)
- **🎨 Search Highlighting** - Matched characters highlighted in search results
- **⌨️ Intuitive Navigation** - Arrow keys to navigate results, ESC clears then exits

### Git Integration
- **🔀 Enhanced Git Status** - Shows modified files and current branch in your repository
- **📝 Git Awareness** - Modified files marked with [M] indicator

### Navigation & UI
- **👁️ Live Preview** - See file contents without opening, with scrollable preview
- **📁 Smart Navigation** - Keyboard-driven interface with vim-like controls
- **🏷️ File Icons** - Visual file type indicators for quick recognition
- **📌 Smart Bookmarks** - Save and manage frequently accessed directories with frecency ranking
- **🎯 Frecency Tracking** - Bookmarks sorted by frequency and recency of visits
- **🚀 Advanced Navigation** - Half-page (Ctrl+D/U) and full-page (Ctrl+F/B) scrolling
- **⚙️ Configurable Root** - Set navigation boundaries with configurable root directory

### Integration & Productivity
- **💻 VS Code Integration** - Open files/directories as VS Code workspaces with 'o' key
- **⚙️ Configurable Editor** - Set your preferred editor in config file
- **📋 Cross-platform Clipboard** - Reliable copy-to-clipboard on all platforms
- **📐 Responsive Design** - Automatically fits terminal size with scrollable windows

## Installation

### Quick Install (Recommended)

**One-liner install script** - automatically detects your OS and architecture:

```bash
curl -sSL https://raw.githubusercontent.com/LFroesch/scout/main/install.sh | sh
```

This downloads the latest pre-compiled binary and installs it to your PATH. No Go required!

### Pre-compiled Binaries

Download the latest release for your platform from [GitHub Releases](https://github.com/LFroesch/scout/releases):

- **Linux** (amd64, arm64)
- **macOS** (Intel, Apple Silicon)
- **Windows** (amd64)

Extract and add to your PATH.

### Build from Source

If you have Go installed:

```bash
go install github.com/LFroesch/scout@latest
```

Make sure `$GOPATH/bin` (usually `~/go/bin`) is in your PATH:
```bash
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

## Usage

```bash
# Open Scout in current directory
scout

# Open Scout in specific directory
scout /path/to/directory
```

## Keyboard Shortcuts

**Press `?` at any time to view the in-app help screen with all keyboard shortcuts!**

### Navigation
- **`↑/↓` or `j/k`** - Move cursor up/down
- **`ctrl+d`** - Half-page down
- **`ctrl+u`** - Half-page up
- **`ctrl+f`** - Full-page down
- **`ctrl+b`** - Full-page up
- **`g`** - Jump to first item
- **`G`** - Jump to last item
- **`enter/l/→`** - Open directory or file
- **`esc/h/←`** - Go to parent directory (respects root path limit)
- **`~`** - Go to home directory

### Search & Filter
- **`/`** - Enter search mode (starts with current directory file search)
- **`Tab`** - Cycle search modes (Current Dir → Recursive → Content Search)
- **`↑/↓`** - Navigate search results (while in search mode)
- **`esc`** - Clear search input first, then exit search mode
- **`S`** - Cycle sort mode (Name → Size → Date → Type)
- **`.`** - Toggle hidden files

### File Operations
- **`e`** - Edit file in default editor
- **`o`** - Open file/directory (files: default app, directories: VS Code workspace)
- **`y`** - Copy file path to clipboard
- **`D`** - Delete file/directory (with confirmation)
- **`u`** - Undo last deletion (restore from trash)
- **`R`** - Rename file/directory
- **`N`** - Create new file
- **`M`** - Create new directory
- **`c`** - Copy current file to clipboard
- **`x`** - Cut current file to clipboard
- **`p`** - Paste file from clipboard

### Sorting & Display
- **`S`** - Cycle sort mode (Name → Size → Date → Type)
- **`r`** - Refresh current directory

### Bookmarks
- **`b`** - Open bookmarks view
- **`B`** - Add highlighted directory to bookmarks
- **In bookmarks view:**
  - **`↑/↓`** - Navigate bookmarks
  - **`enter`** - Go to selected bookmark
  - **`o`** - Open bookmark in VS Code
  - **`d`** - Delete bookmark (with confirmation)
  - **`esc`** - Exit bookmarks view

### View Options
- **`.`** - Toggle hidden files
- **`alt+up/alt+down`** - Scroll preview up/down

### General
- **`?`** - Show help screen with all keyboard shortcuts
- **`q/ctrl+c`** - Quit Scout

## Configuration

Scout stores its configuration in `~/.config/scout/scout-config.json`:

```json
{
  "root_path": "/home/user",
  "bookmarks": [
    "/home/user",
    "/home/user/projects",
    "/etc"
  ],
  "show_hidden": false,
  "preview_enabled": true,
  "editor": "nvim",
  "frecency": {
    "/home/user/projects": 25,
    "/home/user": 10
  },
  "last_visited": {
    "/home/user/projects": "2025-11-15T20:00:00Z",
    "/home/user": "2025-11-15T19:30:00Z"
  }
}
```

### Configuration Options

- **`root_path`** - Sets the highest directory you can navigate to (default: your home directory)
- **`bookmarks`** - Array of bookmarked directory paths (root_path is auto-added if missing)
- **`show_hidden`** - Whether to show hidden files by default
- **`preview_enabled`** - Whether to show the preview pane by default
- **`editor`** - Your preferred text editor (e.g., "nvim", "vim", "nano") - used for the 'e' command
- **`frecency`** - Automatically tracked visit counts for directories (used to sort bookmarks)
- **`last_visited`** - Automatically tracked timestamps of last visits (for frecency calculation)

## File Preview

Scout intelligently categorizes and previews files:
- **Text & Code files** - Full content with syntax-aware detection and scrolling support
- **Directories** - Lists contents with file count and icons
- **Media files** (images, video, audio) - Shows file metadata and type
- **Documents** (PDF, Office files) - Indicates external viewer required
- **Archives** (zip, tar, etc.) - Shows archive type and size
- **Databases** (sqlite, db files) - Identifies as binary database
- **Fonts** - Detects font files (.ttf, .woff, etc.)
- **Executables** - Identifies compiled binaries and bytecode

Large files (>1MB) are not previewed for performance. Preview content is cached (LRU, 50 files) for instant re-display.

### Preview Navigation
- Use **`]`** and **`[`** to scroll through long file contents
- Preview automatically wraps text to fit the terminal width
- Scroll indicators show when more content is available above/below

## Bookmarks System

- **Frecency-based sorting**: Bookmarks automatically sorted by frequency and recency of visits
- **Smart tracking**: Scout learns which directories you visit most and surfaces them first
- **Auto-bookmark root**: Your configured root path is automatically bookmarked
- **Easy access**: Press `b` to view all bookmarks in a full-screen overlay
- **Visit counts**: See how many times you've visited each bookmarked directory
- **VS Code integration**: Press `o` on any bookmark to open it as a VS Code workspace
- **Safe deletion**: Confirmation dialog prevents accidental bookmark removal
- **Status bar info**: See full path of highlighted bookmark in status bar

## Git Integration

- **Branch display**: Current git branch shown in header when in a repository
- **Modified files**: Files with changes marked with `[M]` indicator
- **Auto-detection**: Scout automatically detects when you're in a git repository

## File Type Icons

Scout uses intuitive icons for different file types:
- 🐹 Go files (`.go`)
- 📜 JavaScript/TypeScript (`.js`, `.ts`, `.jsx`, `.tsx`)
- 🐍 Python (`.py`)
- 💎 Ruby (`.rb`)
- ☕ Java (`.java`)
- 🦀 Rust (`.rs`)
- ⚙️ C/C++ (`.c`, `.cpp`, `.h`)
- 🌐 HTML (`.html`, `.htm`)
- 🎨 CSS (`.css`, `.scss`, `.sass`)
- 📋 Config files (`.json`, `.yaml`, `.toml`)
- 📝 Markdown (`.md`)
- 🖼️ Images (`.png`, `.jpg`, `.gif`, `.svg`)
- 📦 Archives (`.zip`, `.tar`, `.gz`)
- 🖥️ Scripts (`.sh`, `.bash`)
- 📁 Directories
- And many more...

## Smart Filtering⬆

Scout automatically hides common development artifacts:
- `node_modules`
- `.git` directories  
- `__pycache__`
- Build directories (`dist`, `build`, `target`)
- IDE configs (`.vscode`, `.idea`)
- System files (`.DS_Store`, `Thumbs.db`)

Toggle hidden files with **`H`** to see everything.

## Performance

Scout is designed to be fast:
- **Advanced fuzzy matching** using optimized algorithms for better results
- **Instant search** within current directory
- **Recursive search** with intelligent filtering and caching
- **Efficient file system traversal** using Go's stdlib optimizations
- **Minimal memory footprint** - lightweight and fast even on older hardware
- **Responsive** even in large directories (thousands of files)
- **Smart caching** - LRU preview cache (50 files) and git status cache (5s TTL)
- **Optimized binary detection** - extension-based categorization with content fallback
- **Automatic terminal size adaptation** for any screen size

## Examples

### Workflow Examples

**Recursive Project Search:**
Launch Scout → `/` to search → `Tab` for recursive mode → type filename → `↑/↓` to navigate → `enter` to open

**Content Search (grep):**
`/` → `Tab` twice for content search → type query (e.g., "func main") → `↑/↓` navigate → `o` to open at line

**File Operations:**
Copy workflow: `c` to copy → navigate to destination → `p` to paste
Delete with undo: `D` to delete → `y` confirm → `u` to undo if needed

**Smart Bookmarks:**
`B` to bookmark current dir → `b` to view bookmarks (sorted by frecency) → `enter` to navigate or `o` for VS Code

## Requirements

### Optional Dependencies
- **ripgrep (rg)** - Required for content search feature (accessible via `/` then `Tab` twice). Install with:
  ```bash
  # Ubuntu/Debian
  sudo apt install ripgrep

  # macOS
  brew install ripgrep

  # Arch Linux
  sudo pacman -S ripgrep
  ```

- **gio or trash-put** - Optional, for moving files to trash instead of permanent deletion:
  ```bash
  # Ubuntu/Debian (gio is usually pre-installed with GNOME)
  sudo apt install gvfs

  # Or use trash-cli
  sudo apt install trash-cli
  ```

## Tips

- **Content search**: Install ripgrep (`rg`) for content search feature (search text inside files)
- **Safe deletion**: Install `gio` or `trash-put` for trash support - enables undo with `u` key
- **Frecency magic**: Bookmarks auto-sort by usage frequency - most-visited directories appear first
- **Custom editor**: Set preferred editor in config: `"editor": "nvim"` (used for `e` key)
- **Root boundary**: Configure `root_path` to limit navigation above your work directory
- **Mouse support**: Click to select files, scroll with mouse wheel in all views