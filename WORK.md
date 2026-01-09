# Scout - Development Roadmap

## Current Tasks

- Add speed dial to bookmarks (1-9 quick jump)
- Add system-wide search mode (search entire filesystem)
- Cross-platform testing (Windows, macOS, verify clipboard)

### Code Quality & Testing

- [ ] Add logging system
  - Create ~/.config/scout/scout.log
  - Log errors, warnings, debug info with configurable levels

- [ ] Unit tests for core functions
  - File operations (copy, move, delete)
  - Fuzzy search logic
  - Bookmark sorting/frecency
  - Path handling

- [ ] Integration tests
  - Full workflows (copy/paste, bookmark management)
  - Cross-platform compatibility
  - Git integration

### Documentation

- [ ] Add godoc comments to exported functions
- [ ] Document complex algorithms (fuzzy search, frecency)
- [ ] Troubleshooting section in README
- [ ] Configuration examples

### Feature Ideas

- Quick actions menu (custom commands per file type)
- Enhanced file preview (images, PDFs, archives)
- Git diff view for changes

## Future Tasks

**Features:** TUI settings menu, customizable keybindings, theme support

**Performance:** Benchmark large directories, optimize fuzzy search, lazy load previews

**Distribution:**
- Package managers: Homebrew, AUR, Chocolatey, Snap/Flatpak
- Auto-update mechanism with version checking
- Demo GIF and comparison section in README

## DevLog

### 2026-01-08 - Bookmarks View Redesign & Documentation Cleanup
- **Bookmarks UI overhaul**: Switched from rounded to normal borders (matches file list/help screen)
- **Improved layout**: Right-aligned frecency scores shown as `×25`, gray paths in parentheses, better spacing
- **Added scroll indicators**: Shows ▲/▼ when more bookmarks than screen space (matches file list pattern)
- **Status bar integration**: Bookmark keybinds now in status bar (enter/o/d/esc hints on right, current path on left)
- **Documentation condensed**:
  - WORK.md reduced from 355 to ~60 lines (merged DevLog entries, kept important tasks/ideas)
  - README.md tightened: condensed Examples section (95→15 lines), Tips section (17→6 items)
  - Removed redundant keybind info already covered in Keyboard Shortcuts section

### 2026-01-08 - UX Improvements & Code Cleanup
- **Unified search interface**: All search via `/` key, Tab cycles through Current Dir → Recursive → Content Search modes
- **Simplified sort**: Press `S` to cycle Name → Size → Date → Type (removed modal menu)
- **Fixed keybind conflicts**: Moved sort to uppercase `S` to avoid conflict with preview scroll
- **Cleaner UI**: Reorganized status bar, improved search color scheme (purple/gray/yellow)
- **Bug fixes**: Fixed symlink display, scroll indicators, mouse support for all views
- **Removed orphaned code**: Cleaned up unused permissions field, bulk operations, obsolete sort menu code
- **Updated docs**: README and help screen now match current keybindings

### 2026-01-08 - Production Readiness
- **Error handling**: Added error dialog mode with user-friendly messages and actionable suggestions
- **Permission validation**: Check before operations, show helpful errors with chmod/sudo suggestions
- **Undo system**: Press `u` to restore deleted files from trash (~/.local/share/Trash, etc.)
- **Symlink handling**: Detect loops, show targets with `[→]` indicator
- **Frecency tracking**: Fixed bookmark visit count persistence (saves immediately)

### 2026-01-07 - Code Organization & Release Automation
- **Split main.go**: Organized 2637 lines into model.go, update.go, view.go, helpers.go (MUV pattern)
- **Created internal modules**: config, search, fileops, git, utils (749 lines extracted)
- **Release automation**: GitHub workflow for cross-platform binaries, install.sh script
- **Fixed critical bugs**: Bookmark indexing, file list alignment, panel height matching

### 2026-01-07 - Cross-Platform & WSL Support
- **Platform support**: Added macOS/Windows file opening, trash integration via runtime.GOOS
- **WSL navigation**: Added `` ` `` key for /mnt/c jump, `~` for home, zone restrictions
- **Improved UX**: Status bar polish, scrollable help screen, better keybindings
