# 🔍 Scout

A lightning-fast TUI file explorer and fuzzy finder for developers. Navigate, search, and preview files with ease.

## Features

- **⚡ Fuzzy Search** - Instantly filter files as you type in current directory
- **👁️ Live Preview** - See file contents without opening, with scrollable preview
- **🔀 Git Integration** - Shows modified files in your repository  
- **📁 Smart Navigation** - Keyboard-driven interface with vim-like controls
- **🎯 Quick Actions** - Open, edit, copy path with single keystrokes
- **💻 VS Code Integration** - Open files/directories as VS Code workspaces with 'o' key
- **📐 Responsive Design** - Automatically fits terminal size with scrollable windows
- **🏷️ File Icons** - Visual file type indicators for quick recognition
- **📌 Smart Bookmarks** - Save and manage frequently accessed directories
- **⚙️ Configurable Root** - Set navigation boundaries with configurable root directory
- **🛡️ Safe Deletion** - Confirmation dialogs for destructive operations

## Installation

```bash
# Clone the repository
git clone <your-repo>/scout
cd scout

# Build
go build -o scout main.go

# Install globally
cp scout ~/.local/bin/

# Make sure ~/.local/bin is in PATH
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
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

### Navigation
- **`↑/↓` or `j/k`** - Move cursor up/down
- **`g`** - Jump to first item
- **`G`** - Jump to last item
- **`enter/l/→`** - Open directory or file
- **`esc/h/←`** - Go to parent directory (respects root path limit)
- **`~`** - Go to home directory

### Search & Filter
- **`/`** - Enter fuzzy search mode
- **`esc`** - Exit search mode
- **Type** - Fuzzy filter files in current directory

### File Operations
- **`e`** - Edit file in default editor
- **`o`** - Open file/directory (files: default app, directories: VS Code workspace)
- **`y`** - Copy file path to clipboard

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
- **`ctrl+s`** - Scroll preview down
- **`ctrl+w`** - Scroll preview up

### General
- **`q/ctrl+c`** - Quit Scout

## Configuration

Scout stores its configuration in `~/.config/scout/scout-config.json`:

```json
{
  "root_path": "/home/lucas",
  "bookmarks": [
    "/home/lucas",
    "/home/lucas/projects",
    "/etc"
  ],
  "show_hidden": false,
  "preview_enabled": true
}
```

### Configuration Options

- **`root_path`** - Sets the highest directory you can navigate to (default: your home directory)
- **`bookmarks`** - Array of bookmarked directory paths (root_path is auto-added if missing)
- **`show_hidden`** - Whether to show hidden files by default
- **`preview_enabled`** - Whether to show the preview pane by default

## File Preview

Scout automatically previews:
- **Text files** - Full content with text wrapping and scrolling support
- **Directories** - Lists contents with file count and icons
- **Images** - Shows file info (no image preview)
- **Binary files** - Shows file size and type only

Large files (>1MB) and binary files are not previewed for performance.

### Preview Navigation
- Use **`ctrl+s`** and **`ctrl+w`** to scroll through long file contents
- Preview automatically wraps text to fit the terminal width
- Scroll indicators show when more content is available above/below

## Bookmarks System

- **Auto-bookmark root**: Your configured root path is automatically bookmarked
- **Easy access**: Press `b` to view all bookmarks in a full-screen overlay
- **VS Code integration**: Press `o` on any bookmark to open it as a VS Code workspace
- **Safe deletion**: Confirmation dialog prevents accidental bookmark removal
- **Status bar info**: See full path of highlighted bookmark in status bar

## Git Integration

Files modified in your git repository are marked with `[M]` indicator. Scout automatically detects when you're in a git repository and shows the current status.

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
- Instant fuzzy search within current directory
- Efficient file system traversal
- Minimal memory footprint
- Responsive even in large directories
- Automatic terminal size adaptation

## Examples

### Quick File Navigation
```
1. Launch Scout: `scout`
2. Press `/` to search
3. Type "main.go" 
4. Press `enter` to open
```

### Browse and Preview
```
1. Navigate with `j/k`
2. Preview shows on the right
3. Use `ctrl+s/w` to scroll preview
4. Press `p` to toggle preview
5. Press `e` to edit selected file
```

### Bookmark Management
```
1. Navigate to interesting directory
2. Press `B` to bookmark it
3. Press `b` to view all bookmarks
4. Navigate with `j/k`, press `enter` to go
5. Press `o` to open in VS Code
6. Press `d` to delete (with confirmation)
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
3. Path is now in clipboard
```

## Tips

- **Root boundary**: Configure `root_path` to prevent accidentally navigating above your work directory
- **Fuzzy search**: Use `/` liberally - it's very fast for finding files
- **Quick navigation**: Use `g`/`G` to jump to top/bottom of file lists
- **Space optimization**: Hide preview with `p` for more file list space in narrow terminals
- **Bookmark workflow**: Bookmark project roots, then use `b` → `o` for instant VS Code access
- **Status bar**: Watch the status bar for file info and keyboard shortcuts

## Current Limitations

- Search only works within current directory (not recursive)
- No file operations (copy, move, delete files)
- No multiple file selection
- No image preview (terminal limitation)
- No customizable keybindings

## Future Ideas

- Recursive file search with ripgrep integration
- Basic file operations (create, copy, move, delete)
- Multiple file selection with bulk operations
- Customizable themes and keybindings
- Plugin system for external tool integration
- Session management and workspace restoration