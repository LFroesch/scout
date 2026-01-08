# Scout - Production Readiness Roadmap

## Current Tasks

- Add speed dial to bookmarks [1-9 or w/e]
- add Entire System fuzzy? ish? search
- clean orphaned code/readme stuff etc
- Clean up keybinds / help / prune
- [x] Refactor main.go into modules (COMPLETED - extracted search, fileops, git, utils)

### Critical Bugs
### Cross-Platform Support

- [ ] **Testing & validation**
  - Test on actual Windows system
  - Test on macOS
  - Verify clipboard library on all platforms

### Critical Production Issues

**Why These Matter for "Download-Worthy" Status:**

1. **Error Recovery** - Currently any failed operation crashes or silently fails
   - [ ] Wrap all file operations in proper error handling
   - [ ] Show user-friendly error dialogs instead of status messages
   - [ ] Never crash - always recover gracefully
   - [ ] Example: "Cannot delete file (permission denied). Try running with sudo or changing permissions."

2. **Safety Features** - Prevent user disasters
   - [ ] Symlink detection - Don't follow loops, show → target
   - [ ] Permission validation - Check before attempting operations
   - [ ] Confirmation for bulk delete (>5 files)
   - [ ] Undo system or trash instead of rm (already using trash, but no undo)
   - [ ] File size warnings (deleting >100MB? confirm first)

3. **Bulk Operations UX** - Multi-select exists but confusing
   - [ ] Show "5 files selected" in status bar
   - [ ] Highlight all selected files differently
   - [ ] "Select all" keybind (ctrl+a)
   - [ ] "Deselect all" keybind (ctrl+d)
   - [ ] Preview what bulk operation will do before executing

4. **Search Performance** - Prevent freezing on large directories
   - [x] Debounce (require 2+ chars before expensive searches)
   - [ ] Limit results to 1000 max with "... 500 more" message
   - [ ] Add timeout for searches >5 seconds
   - [ ] Show progress for slow operations

5. **Missing Essential Features**
   - [ ] Speed dial bookmarks (1-9 keys to jump)
   - [ ] System-wide search mode (search entire /, not just current dir)
   - [ ] Quick actions menu (custom commands per file type)
   - [ ] File preview for more types (images, PDFs, archives)
   - [ ] Diff view for git changes

### Code Quality & Architecture
- [x] **Refactor main.go into modules** (COMPLETED)
  - [x] config/ - Configuration management
  - [x] search/ - Fuzzy search & content search
  - [x] fileops/ - File operations (copy, move, delete, trash)
  - [x] git/ - Git integration
  - [x] utils/ - Utilities (icons, formatting, type detection)
  - [x] ui/ - Split main.go into model.go, update.go, view.go, helpers.go (M-U-V pattern)
  - Each module/file has clear responsibility ✓
  - Clear separation of concerns ✓

- [ ] **Add comprehensive error handling**
  - Wrap all exec.Command() calls with proper error handling
  - User-friendly error messages instead of silent failures
  - Log errors to file for debugging

- [ ] **Add logging system**
  - Create ~/.config/scout/scout.log
  - Log errors, warnings, and debug info
  - Configurable log levels

- [ ] **Input validation**
  - Validate file paths before operations
  - Sanitize user input in rename/create dialogs
  - Check permissions before file operations

### Testing
- [ ] **Unit tests for core functions**
  - File operations (copy, move, delete)
  - Fuzzy search logic
  - Bookmark sorting/frecency
  - Path handling

- [ ] **Integration tests**
  - Full workflows (copy/paste, bookmark management)
  - Cross-platform compatibility tests
  - Git integration tests

### Documentation
- [ ] **Code documentation**
  - Add godoc comments to all exported functions
  - Document complex algorithms (fuzzy search, frecency)

- [ ] **User documentation**
  - Installation guide for all platforms
  - Troubleshooting section
  - Configuration examples

## Future Tasks

### Features
- [ ] Independent right pane navigation (main.go:1572 TODO)
- [ ] Plugin system for extensibility
- [ ] Configuration via TUI settings menu
- [ ] Customizable keybindings
- [ ] Theme support

### Performance
- [ ] Benchmark large directory performance
- [ ] Optimize fuzzy search algorithm
- [ ] Cache git status checks
- [ ] Lazy load file previews

### Distribution - **CRITICAL FOR DOWNLOADS**

**Current State:** Users need Go installed, manual build, no releases
**Target State:** `brew install scout` or `curl -sSL install.sh | sh`

**Why This Matters:**
- 90% of users won't install Go just to try your tool
- No releases = looks abandoned/not serious
- No package managers = friction to install = no users

**Action Items:**
- [x] GitHub Actions workflow for releases
  - Build binaries: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64
  - Upload to GitHub Releases automatically on git tag
  - Include checksums (sha256)

- [x] One-liner install script
  - Detects OS/arch automatically
  - Downloads latest binary from GitHub releases
  - Installs to ~/.local/bin or /usr/local/bin
  - Usage: `curl -sSL https://raw.githubusercontent.com/LFroesch/scout/main/install.sh | sh`

- [ ] Package Managers
  - [ ] Homebrew formula (tap: homebrew-scout)
  - [ ] AUR package (for Arch Linux)
  - [ ] Chocolatey (for Windows)
  - [ ] Snap/Flatpak (for Linux)

- [ ] Auto-update mechanism
  - Check for new version on startup (once per day max)
  - Show "Update available: v1.2.3 → v1.3.0" in status bar
  - Simple `scout update` command to self-update

**README Must-Haves for Downloads:**
- [ ] Demo GIF in README (record with asciinema + agg)
- [ ] "Why Scout?" section - Compare to ranger, lf, nnn
- [ ] Instant value prop: "Navigate 10x faster than cd"
- [ ] Quick start: install → screenshot → profit
- [ ] Troubleshooting section for common issues

## DevLog

### 2026-01-07 - Split main.go into Organized Files (Main Package)
- **Split main.go (2637 lines) into 5 organized files** in the main package to improve code organization
- **main.go** (15 lines): Minimal entry point with just main() function
- **model.go** (694 lines): Types, constants, model struct, and all model methods (data manipulation, search, preview)
- **update.go** (773 lines): Init() and Update() - all state update logic and event handling
- **view.go** (1020 lines): View() and all render*() methods - complete UI rendering layer
- **helpers.go** (116 lines): Helper functions (openFile, editFile, openInVSCode, copyPath)
- **Why this matters**: Clear separation of concerns (Model-Update-View pattern), easier navigation, each file has focused responsibility
- **Impact**: main.go reduced from 2637 → 15 lines (99.4% reduction), code is now organized by responsibility rather than dumped in one huge file
- **Note**: Files remain in main package (not internal/) to avoid import cycles with Bubble Tea's Model interface requirements

### 2026-01-07 - Refactored main.go into Internal Modules
- **Extracted 749 lines** from main.go (3142 → 2637 lines) into organized internal modules
- **internal/config** (95 lines): Configuration loading/saving, frecency management
- **internal/search** (203 lines): Fuzzy filename search, content search with ripgrep, recursive file finding
- **internal/fileops** (163 lines): Cross-platform file operations (copy, move, delete, trash integration)
- **internal/git** (52 lines): Git status integration (modified files, current branch)
- **internal/utils** (236 lines): File type detection, icons, formatting, highlighting
- **Why this matters**: Each module now has clear responsibility, easier to test and maintain
- **Impact**: main.go is 16% smaller, modules are independently testable and reusable

### 2026-01-07 - Added Release Automation
- Created `.github/workflows/release.yml` to build cross-platform binaries on version tags
- Builds for Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
- Auto-uploads binaries and SHA256 checksums to GitHub Releases
- Created `install.sh` one-liner script: detects OS/arch, downloads latest binary, installs to PATH
- Updated README with installation options (one-liner first, then binaries, then build from source)

### 2026-01-07 - Initial Production Roadmap
- Identified critical bookmark indexing bug causing wrong directory navigation
- Mapped out cross-platform support gaps (Windows, macOS, WSL)
- Created refactoring plan to split 2900-line main.go into modules
- Prioritized: Critical bugs → Cross-platform → Code quality → Tests → Docs

### 2026-01-07 - Fixed Critical Bookmark Bug
- Fixed bookmark indexing bug where cursor selected wrong directory
- Root cause: Bookmarks displayed sorted by frecency, but navigation indexed unsorted array
- Solution: Added sortedBookmarkPaths field to model, called sortBookmarksByFrecency() on bookmark mode entry
- Updated all bookmark nav code (j/k/enter/o/d), renderBookmarksView(), status bar to use sorted array

### 2026-01-07 - Fixed File List Alignment
- File sizes now right-aligned in left pane for professional appearance
- Git status [M] positioned consistently after filename
- Layout: "✓ 📄 filename.txt [M]        123 KB" with proper spacing
- Used lipgloss.Width() for accurate ANSI code handling in padding calculation

### 2026-01-07 - Fixed Panel Height Matching
- Fixed right panel expanding larger than left panel with long lines (go.mod, go.sum)
- Issue 1: lipgloss Width() constraint caused auto-wrapping - removed, using manual truncation only
- Issue 2: Scroll indicators (▲/▼) added 1-2 extra lines without reserving space
- Both panels now strictly enforce max line count by reserving indicator space before calculating visible items
- Status bar spacing fixed to use lipgloss.Width() instead of len() for ANSI codes
- Made truncation more conservative (width-8) for non-fullscreen terminals

### 2026-01-07 - Improved Status Bar & Help UX
- Simplified status bar: removed cluttered two-row keybinds, now shows "Press ? for help"
- Fixed status bar background consistency by adding background color to styled text
- Redesigned help screen: uses consistent panel UI like other views (not centered floating)
- Made help screen scrollable with j/k keys, shows scroll indicators
- Fixed help scrolling bug at bottom by adjusting scroll bounds properly

### 2026-01-07 - Changed Paste Keybind
- Changed paste from `P` to `p` for easier access
- Removed preview toggle (was `p`) - preview always on by default now
- Updated help screen to reflect new keybinding

### 2026-01-07 - Fixed Status Bar Background Consistency
- Removed background styling from keyStyle in help hint to prevent background conflicts
- Applied background color only once via statusStyle.Render() at the end
- Ensures grey background covers entire bottom bar uniformly

### 2026-01-07 - Added Cross-Platform Support
- Integrated github.com/skratchdot/open-golang for file opening (Linux/macOS/Windows)
- Replaced hardcoded `xdg-open` with `open.Run()` for automatic platform detection
- Updated `moveToTrash()` with runtime.GOOS detection:
  - macOS: Uses AppleScript to move files to Finder trash
  - Windows: Uses PowerShell with RecycleBin API
  - Linux: Keeps existing gio/trash-put commands
- Scout now works natively on Windows, macOS, Linux, and WSL
- Removed RootPath filesystem restriction - now allows navigation to entire filesystem including /mnt (WSL Windows drives)
- Added /mnt to default bookmarks for easy WSL/Windows filesystem access

### 2026-01-07 - Improved WSL/Windows Navigation UX
- Added backtick (`) keybind to instantly jump to /mnt/c (Windows C: drive)
- Tilde (~) keybind jumps to WSL home directory
- Added "zone" restriction: prevents accidentally backing out of home directory
  - When in /home/lucas or subdirectories, cannot navigate above home using ←/esc/h or ".."
  - Must use ` keybind to intentionally switch to Windows filesystem
  - Once in /mnt/c, can navigate freely throughout Windows drives
- Shows helpful message when attempting to navigate above home: "Cannot navigate above home directory (use ` to jump to /mnt/c)"
- Updated help screen with new keybindings

### 2026-01-07 - Unified Search UX Overhaul
- **Unified search interface:** Filename (`/`) and content search (`ctrl+g`) now use the same UI
- Removed separate content search modal - everything shows in file list
- **Clear mode indicators:** Top-right shows `🔍 FILENAME [RECURSIVE]` or `🔍 CONTENT [CURRENT DIR]`
- **Recursive toggle works for both:** `ctrl+r` toggles recursive for filename AND content search
- **ESC cancels properly:** Returns to normal mode and clears search state
- **Content search improvements:**
  - Results display as: `path/to/file.go:123 - matched line content`
  - Press `e` to open file at exact line number (works in vim, nano, code)
  - Respects `.gitignore` by default (won't search node_modules, .git, etc)
  - Shows `--hidden` flag only when hidden files enabled
- **Performance:**
  - Debounce: Requires 2+ characters before running expensive searches
  - Shows "Searching..." status for recursive/content searches
  - Result counts: "Found 42 matches" displayed in status
- **Help page fixes:**
  - Updated Search & Filter section with new unified workflow
  - Removed View Options section (obsolete)
  - Increased spacing from %-20s to %-30s for better readability
  - Added ESC hint for canceling searches

---

## Implementation Notes

### Bookmark Bug Fix Details
The issue is in renderBookmarksView() and bookmark navigation:
1. renderBookmarksView() sorts bookmarks by frecency (lines 1957-1970)
2. Display shows sorted array with cursor at position `m.bookmarksCursor`
3. But navigation code (lines 865-896) uses `m.config.Bookmarks[m.bookmarksCursor]`
4. This indexes the UNSORTED array, causing mismatch

**Solution:** Store sorted bookmarks in model or map cursor → original index

### Cross-Platform Strategy
- Use runtime.GOOS to detect platform
- Abstract platform-specific commands into interfaces
- Factory pattern for command execution
- Test matrix: Linux, macOS, Windows, WSL

### Refactoring Strategy
- Start with interfaces and types
- Move related functions together
- Ensure backward compatibility
- Run tests after each module extraction
