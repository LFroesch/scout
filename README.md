# 🦅 Scout

A lightning-fast TUI file explorer and fuzzy finder for developers. Navigate, search, and preview files with ease.

## Features

### Core File Management
- **📁 Full File Operations** - Delete, rename, create files/directories with confirmation dialogs
- **✂️ Copy/Cut/Paste** - Clipboard operations for files and directories (c, x, P)
- **☑️ Bulk Selection** - Select multiple files with space, operate on all at once
- **🗑️ Safe Deletion** - Move to trash when possible, with confirmation dialogs
- **📊 Smart Sorting** - Sort by name, size, date, or type (press S)
- **📐 File Info Display** - Shows file sizes, selection checkboxes, and git status

### Advanced Search
- **⚡ Advanced Fuzzy Search** - Lightning-fast fuzzy matching with result highlighting
- **🔄 Recursive Search** - Search across entire project tree (toggle with Ctrl+R in search mode)
- **🔍 Content Search** - Search inside files with ripgrep integration (Ctrl+G)
- **🎨 Search Highlighting** - Matched characters highlighted in search results

### Git Integration
- **🔀 Enhanced Git Status** - Shows modified files and current branch in your repository
- **📝 Git Awareness** - Modified files marked with [M] indicator

### Navigation & UI
- **👁️ Live Preview** - See file contents without opening, with scrollable preview
- **🖼️ Dual Pane Mode** - Side-by-side file management (toggle with T)
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
- **`tab`** - Switch between panes (dual pane mode only)

### Search & Filter
- **`/`** - Enter fuzzy search mode (filename search)
- **`ctrl+r`** - Toggle recursive search (in search mode)
- **`ctrl+g`** - Content search (search inside files with ripgrep)
- **`esc`** - Exit search mode
- **Type** - Fuzzy filter files with intelligent matching and highlighting

### File Operations
- **`e`** - Edit file in default editor
- **`o`** - Open file/directory (files: default app, directories: VS Code workspace)
- **`y`** - Copy file path to clipboard
- **`D`** - Delete file/directory (with confirmation)
- **`R`** - Rename file/directory
- **`N`** - Create new file
- **`M`** - Create new directory
- **`c`** - Copy selected files to clipboard
- **`x`** - Cut selected files to clipboard
- **`P`** - Paste files from clipboard
- **`space`** - Toggle selection (for bulk operations)

### Sorting & Display
- **`S`** - Open sort menu (sort by name/size/date/type)
- **`I`** - Toggle permissions display
- **`T`** - Toggle dual pane mode

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
- **`H`** - Toggle hidden files
- **`p`** - Toggle preview pane
- **`r`** - Refresh current directory
- **`w/s`** - Scroll preview up/down (also works with alt+up/down)

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

Scout automatically previews:
- **Text files** - Full content with text wrapping and scrolling support
- **Directories** - Lists contents with file count and icons
- **Images** - Shows file info (no image preview)
- **Binary files** - Shows file size and type only

Large files (>1MB) and binary files are not previewed for performance.

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

## Smart Filtering

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
- **Smart caching** of file metadata and git status
- **Automatic terminal size adaptation** for any screen size

## Examples

### Quick File Navigation
```
1. Launch Scout: `scout`
2. Press `/` to search
3. Type "main.go"
4. Press `enter` to open
```

### Recursive Project Search
```
1. Launch Scout in your project root
2. Press `/` to enter search mode
3. Press `ctrl+r` to enable recursive search
4. Type a few characters from the filename
5. See results from entire project tree with highlighted matches
6. Navigate and open any file instantly
```

### Content Search (Search Inside Files)
```
1. Launch Scout in your project root
2. Press `ctrl+g` for content search
3. Type your search query (e.g., "func main")
4. Press `enter` to search with ripgrep
5. Navigate results with `j/k`
6. Press `o` to open file at the matching line
7. Press `esc` to close search
```

### File Management Operations
```
# Delete files
1. Navigate to file with `j/k`
2. Press `D` to delete
3. Confirm with `y`

# Rename files
1. Navigate to file
2. Press `R` to rename
3. Edit the name and press `enter`

# Copy/Paste workflow
1. Navigate to files you want to copy
2. Press `space` to select multiple files (or just one)
3. Press `c` to copy (or `x` to cut)
4. Navigate to destination directory
5. Press `P` to paste

# Create new files/directories
1. Press `N` to create a new file
2. Press `M` to create a new directory
3. Type the name and press `enter`
```

### Bulk Operations
```
1. Navigate through files with `j/k`
2. Press `space` to select files (✓ appears)
3. Select multiple files
4. Press `c` to copy all selected files
5. Navigate to destination
6. Press `P` to paste all at once
```

### Dual Pane Mode
```
1. Press `T` to toggle dual pane mode
2. Two file lists appear side by side
3. Press `tab` to switch between panes
4. Useful for comparing directories or copying files
```

### Sorting Files
```
1. Press `S` to open sort menu
2. Use `j/k` to select sort mode:
   - Name (default)
   - Size (find largest files)
   - Date (find newest files)
   - Type (group by extension)
3. Press `enter` to apply
```

### Smart Bookmark Workflow
```
1. Navigate to interesting directory
2. Press `B` to bookmark it
3. Visit your bookmarked directories frequently
4. Press `b` to view bookmarks - sorted by visit frequency
5. Most visited directories appear at the top
6. Navigate with `j/k`, press `enter` to go
7. Press `o` to open in VS Code
8. Press `d` to delete (with confirmation)
```

### VS Code Integration
```
1. Navigate to a directory/file
2. Press `o` to open as VS Code workspace
3. Works from both file view and bookmarks view
```

### Copy File Paths
```
1. Navigate to file
2. Press `y` to copy path
3. Path is now in clipboard (works on all platforms)
```

## Requirements

### Optional Dependencies
- **ripgrep (rg)** - Required for content search feature (`ctrl+g`). Install with:
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

- **Root boundary**: Configure `root_path` to prevent accidentally navigating above your work directory
- **Content search**: Install ripgrep (`rg`) for powerful content search - find text inside files instantly
- **Safe deletion**: Install `gio` or `trash-put` to move deleted files to trash instead of permanent deletion
- **Bulk operations**: Select multiple files with `space`, then use `c`/`x`/`P` for efficient file management
- **Git awareness**: Modified files show [M] marker and current branch displays in header
- **Dual pane mode**: Use `T` for dual pane mode when copying/moving files between directories
- **Recursive search**: Use `ctrl+r` in search mode to search your entire project - perfect for finding files deep in the tree
- **Fast navigation**: Use `ctrl+d`/`ctrl+u` for half-page jumps, `ctrl+f`/`ctrl+b` for full-page
- **Frecency magic**: The more you use Scout, the smarter your bookmarks become - most-used directories rise to the top
- **Quick navigation**: Use `g`/`G` to jump to top/bottom of file lists
- **Space optimization**: Hide preview with `p` for more file list space in narrow terminals
- **Bookmark workflow**: Bookmark project roots, then use `b` → `o` for instant VS Code access
- **Custom editor**: Set your preferred editor in config with `"editor": "nvim"` (or vim, nano, etc.)
- **Search highlighting**: Matched characters are highlighted in yellow - helps you understand why results matched
- **Sorting**: Press `S` to sort by size (find largest files) or date (find newest files)
- **Status bar**: Watch the status bar for file info, git branch, and keyboard shortcuts
- **Git awareness**: Current branch shows in header - always know which branch you're working on