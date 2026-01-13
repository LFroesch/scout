# Scout - Development Roadmap

## Current Tasks

## Medium

- show date on file window? search window too?
- git [M] stops the cursor bar background
- Add bookmark pinning/favoriting system (quick dial integration with frecency) split panel? quick dial one side, frecent on other? more columns? idk
- make double ? right click ? work like o/enter

## Prior to Launch

- Make sure that how this is all set up is correct
- User/test install to powershell? not just wsl?
- Cross-platform testing (Windows, macOS, verify clipboard)
- normalize to X client type (ps, wsl, ???)
- test if it works with other editors (vim/etc)

### Code Quality & Testing/

- [ ] Integration tests (full workflows, cross-platform compatibility)
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

### 2026-01-13 - Search Cancellation Race Condition Fix
- **Fixed premature search cancellation**: Removed cancelCurrentSearch() from Enter key handler - pressing Enter now only locks results without stopping the search
- **Background search continues**: Search continues running in background after pressing Enter, allowing results to stream in while navigating/scrolling
- **Explicit exit only**: Search now only cancels on explicit actions (ESC, q, Ctrl+C, "/") or when navigating away from search mode
- **Resolved race condition**: Fixed issue where rapidly hitting Enter then scrolling/copying would cancel searches before completion

### 2026-01-13 - Search Mode Command Key Fix
- **All commands require locked results**: Fixed all command keys (w/s/y/o/S/./f) to only trigger actions when search results are locked (press Enter to lock)
- **Preview scrolling in search**: w/s and alt+up/alt+down scroll preview when locked
- **Open file in search**: o opens file/directory in editor/VS Code when locked
- **Path copy in search**: y copies selected file path when locked
- **Sort in search**: S cycles sort modes when locked
- **Toggle hidden in search**: . toggles hidden files when locked
- **Navigate in search**: f navigates to directory/parent when locked
- **Type any character while searching**: All these keys can be typed in search query when not locked (e.g., search for "open", "filter", "SYSTEM", ".bashrc", "windows")

### 2026-01-13 - File Type System Overhaul & Performance Caching
- **Refactored binary detection**: Removed confusing greylist, implemented industry-standard file type categorization (Text, Code, Media, Document, Archive, Database, Font, Executable, Unknown)
- **Smart file categorization**: Extension-based detection with content fallback for unknown types - fixed bugs where .h/.ini were misclassified
- **Enhanced preview messages**: Different messages for each file type (media, documents, archives, fonts) instead of generic "binary file"
- **Preview caching**: LRU cache (50 files) stores file content with mod-time validation - instant re-display when revisiting files
- **Git status caching**: 5-second TTL cache for git modified/branch status - reduces redundant git calls
- **Performance**: Cached previews + git status = smoother navigation, especially in git repos with many files

### 2026-01-13 - Cursor Visibility Fix (Filtering & Toggling Hidden Files)
- **Fixed cursor disappearing after filter operations**: Cursor now properly moves to last valid item when list shrinks (e.g., cursor at index 10, filtered list has 5 items → cursor moves to item 4)
- **Auto-scroll adjustment**: Scroll offset automatically adjusts to ensure cursor stays visible after filtering, searching, or toggling hidden files
- **New helper method**: Added `ensureCursorInBounds()` that clamps cursor and adjusts scroll - called after `loadFiles()`, `updateFilter()`, and search result updates
- **Consistent behavior**: Applied fix to all filtering operations: search (/, tab modes), toggle hidden files (.), and directory navigation

### 2026-01-10 - Status Bar Styling Fix & Search Status Colors
- **Consistent background colors**: All status bar text now has explicit bg 235 - fixes rendering issues where background wasn't applied consistently
- **Proper label styling**: Labels like "Branch:", "Sort:" use purple (99) with bold and bg 235
- **Proper value styling**: Values like branch name, sort mode use white (255) with bg 235
- **Fixed separators**: All separators (":", " | ", "/") now use whiteStyle with bg 235
- **Keybind hints**: All keybind labels (purple) and descriptions (white) properly styled with bg 235
- **Orange search indicators**: "Searching", "Searching...", "search continues..." now use orange (214) bold for visibility
- **Purple file counts**: All numeric counts in search status ("87 files found", "2000 files scanned") use purple (99) bold
- **White descriptive text**: Words like "files found", "files scanned", "Search complete:" use white (255)
- **Styled minimum char message**: "Type at least X characters to search" uses white for text, purple for number
- **Exit search status**: Shows orange "Exited search" message when leaving search mode (ESC, q, or f key)

### 2026-01-10 - UI Polish & Search Mode Improvements
- **Enhanced status bar styling**: Numbers and keybinds now styled in purple (99) with darker background (235) - file counts, clipboard counts, scanned files, and all keybind hints are highlighted
- **Middle-click documentation**: Added middle-click keybind description to help page showing navigation to clicked directory/parent
- **Hidden files toggle in search**: Added `.` key support in search mode to toggle hidden files - works while actively searching and updates results in real-time

### 2026-01-10 - Preview Panel Height Fix
- **Fixed height calculation bug**: Changed contentHeight from `availableHeight - 2` to `availableHeight - 1` to properly account for header (was causing panels to be 1 line too short, making status bar clip)
- **Added MaxHeight enforcement**: Both renderFileList() and renderPreview() borderStyles now use MaxHeight() in addition to Height() to strictly prevent content overflow
- **Added strict line limiting**: Preview content enforces maximum line count (`availableHeight - 1`) to prevent overflow beyond available height
- **Removed double height styling**: Eliminated outer Height() wrappers since panels already set heights internally
- **Fixed status bar clipping**: Both panels now render at exactly the correct height, leaving proper space for status bar

### 2026-01-10 - Keybinding UX Improvements
- **Removed 'e' keybinding**: Simplified keybindings by removing confusing 'e' key - now just ENTER, 'o', and 'f' with clear purposes
- **Smart file opening with fallback**: Both ENTER and 'o' now try editor/VS Code first (code→vim→nano→vi), then fall back to system default if no editor found - always provides feedback
- **Content search line jumping**: When opening files from content search results, both ENTER and 'o' open the file at the matching line number
- **'f' key navigation**: Press 'f' to navigate to selected directory (or parent of file) in scout - works in both normal and search modes, exits search mode automatically
- **Cancel searches on navigation**: When exiting search mode via navigation (f key, middle-click, enter on dir), in-progress searches are now cancelled to prevent unexpected results
- **Dynamic status bar hints**: Status bar shows what ENTER/O/F will do based on selected item - "enter: open | o: editor | f: parent dir" for files, "enter: open | o: VS Code" for dirs

### 2026-01-10 - Search Mode UX Enhancements
- **Drive path in status bar**: Status bar now shows full path (e.g., "/mnt/c") during drive scanning instead of just file count - makes it clear which drive is being searched
- **File open feedback**: Shows "Can't open [file] via scout" in status bar when double-click/enter can't open a file - clear feedback instead of silent failures
- **Middle-click navigation**: Middle-click on any file/directory in search or normal mode navigates to that directory (or parent if file) in scout - exits search mode and loads the location

### 2026-01-10 - Hidden Files, Mouse Clicks & Preview Path
- **Hidden files default behavior**: Confirmed hidden files show by default (config default: `ShowHidden: true`) - toggle with `.` key affects file browser only, search always shows hidden files regardless of toggle state
- **Fixed mouse click offset in search**: Mouse clicks now calculate visible height correctly accounting for scroll indicators - prevents clicking wrong items when list is scrolled
- **Fixed mouse click offset in normal mode**: Applied same fix to normal file browser mode for consistent behavior
- **Full path in preview**: Preview pane now displays full file/directory path below the filename for both file browser and search results - helps identify exact location when navigating deep directories

### 2026-01-09 - Search UX Improvements
- **'q' key in locked search**: Press 'q' when search results are locked to exit search mode (same as ESC) - types normally when actively searching
- **Purple title in search**: Title color stays purple (99) in search mode instead of changing to gray - consistent with normal mode

### 2026-01-09 - Search Results Interaction Enhancements
- **Mouse wheel scroll in search**: Added scroll support in search mode - matches normal mode behavior for consistent UX
- **Click support in search**: Single-click selects result, double-click opens file or enters directory (when results locked)
- **Sort search results**: Press `S` in search mode to cycle through Name/Size/Date/Type sorting - preserves match highlighting during sort
- **Fixed ultra search highlighting**: Match positions now account for drive label prefix "[C] " - highlights correct characters in results
- **Search complete status**: Shows "Search complete: X files" when all drives finish (was already implemented, now verified working)
- **Drive-specific status**: Displays "Searching C:..." or "Searching L:..." during ultra search (was already implemented, now verified working)

### 2026-01-09 - Path Deduplication & Search UX
- **Fixed ultrasearch duplicates**: Skip `/mnt` and `/media` when searching from root - prevents duplicate results (root would find files in `/mnt/l/...` which are also found when searching `/mnt/l` separately)
- **Search always includes hidden files**: Searches now find `.bashrc`, `.config/`, etc. regardless of ShowHidden setting - if you're searching, you want to find it
- **Config field ordering**: Moved `skip_directories` to top of config file for better discoverability - first thing users see when editing config
- **Auto-generate skip_directories**: Missing `skip_directories` field now auto-populates with defaults and saves to config for easy editing

### 2026-01-09 - Configurable Skip Directories
- **User-editable filter list**: Added `skip_directories` array to config file - add/remove directories as needed per user
- **Smart defaults**: Ships with sensible blocklist - Python*, Call of Duty*, browser caches, Android SDK, game directories
- **Wildcard support**: Use `*` for pattern matching (e.g., `Python*` matches Python27, Python312, Python313)
- **Merged filtering**: Combines hardcoded essential filters (system dirs, node_modules) + user custom filters
- **Quick config access**: Press `,` to open config file in editor - edit skip_directories and save
- **Removed fuzzy dependency**: Cleaned up unused `github.com/sahilm/fuzzy` package

### 2026-01-09 - Search Overhaul: Substring Matching & Daily-Use Filters
- **Substring matching**: Replaced fuzzy matching with exact substring search - "test" only matches files/paths containing "test" as-is, no more random scattered letter matches
- **Comprehensive filtering**: Added 30+ directories for daily-use navigation - game dirs (Steam, Xbox, Epic), Python venvs, language toolchains (.cargo, .gradle, .go), IDE folders (.vscode, .idea)
- **Pattern matching**: Added Unity*/Unreal* patterns to catch game engine directories
- **All search modes**: Changes apply to recursive, ultra, and content searches
- **Cleaner results**: Searches now return only relevant development/user files, eliminating bloat from game installs and language tooling

### 2026-01-09 - WSL System Directory Filtering
- **Absolute path filtering**: Added filtering for Linux/WSL system directories (/usr, /bin, /sbin, /lib*, /etc, /opt) via absolute path matching
- **Safe for projects**: Only filters root-level system directories - project folders named "lib" or "bin" remain searchable
- **Power user friendly**: Keeps /mnt accessible for Windows drive access, minimal filtering approach
- **Applied to all search modes**: Recursive, content (ripgrep), and ultra search now skip system binaries and libraries
- **Performance**: Prevents scanning thousands of binary/library files in system directories, speeding up searches from root

### 2026-01-09 - Configurable Search Parameters
- **Config options**: Added maxResults, maxDepth, maxFilesScanned to config file (defaults: 5000, 5, 100000)
- **Validation**: Enforces bounds (results: 100-50000, depth: 1-20, files: 1000-1000000) to prevent performance issues
- **Flexible searching**: Users can now tune search aggressiveness based on their hardware and use case

### 2026-01-09 - Parallel Ultra Search with Streaming Results
- **Fixed content search**: Properly handles permission denied errors from ripgrep (exit code 2) - continues with partial results instead of failing
- **Parallel ultra search**: All drives now searched concurrently in separate goroutines - no longer waits for each drive sequentially
- **Streaming results**: Results appear as each drive completes - fast SSDs (/, /mnt/c) show results in seconds while slow HDDs continue in background
- **Real-time progress**: Status bar shows "Searching [drive]... X files scanned" with per-drive updates every 1000 files
- **Performance boost**: Ultra search on 5 drives went from 4+ minutes sequential to ~4 minutes parallel (limited by slowest drive, not sum of all)
- **Better UX**: Can see and navigate results from fast drives immediately, cancel anytime with ESC and keep found results

### 2026-01-09 - Search Permission & Performance Improvements
- **Expanded skip directories**: Added 15+ new system directories to skip list (Config.Msi, PerfLogs, /proc, /sys, /dev, /run, /tmp, /var, /boot, /snap, etc.)
- **Pattern matching for skips**: Dynamic filtering for TEMP*, UMFD-*, wsl*, AMD*, found.*, *Font Driver* directories
- **Reduced log spam**: Permission errors now counted in summary instead of logging each individually (was 400+ error lines, now 1 summary line)
- **Better cancellation**: Searches now properly detect user ESC and cancel immediately without logging as errors
- **Performance**: Searches on /mnt/c now skip 2-3x more directories, resulting in faster scans and cleaner logs

### 2026-01-09 - Search Performance Overhaul (Large Drive Fix)
- **Smart directory filtering**: Auto-skip large/system dirs (node_modules, .git, Windows/Program Files, etc.) - counts skips, doesn't spam logs
- **Proper cancellation**: Cancel checking integrated into WalkDir loop AND ripgrep execution - ESC stops everything immediately
- **Content search (ripgrep)**: Now cancellable mid-execution - kills ripgrep process on ESC or timeout
- **Debug logging**: All searches log to ~/.config/scout/scout.log (start/cancel/complete, summary stats, timings)
- **Progress visibility**: Status bar shows "Searching... (X files scanned)" with live count updating every 1000 files
- **Better UX on C: drive**: All search modes (recursive, content, ultra) now work properly with large drives

### 2026-01-09 - Search Safety Limits (Large Drive Fix)
- **Fixed search result locking**: Enter now properly locks results for navigation only (can't navigate outside filtered results)
  - While locked: Enter on folder navigates into it and exits search, "/" starts fresh search, ESC clears all
  - Fixed issue where normal navigation (h/left/esc) would let you navigate outside search results
- **Aggressive limits for /mnt/c searches**: Prevents infinite hanging on large drives
  - **Recursive/Ultra**: Max depth 5 (not 10), max 5000 results, max 100k files scanned
  - **Content (ripgrep)**: Max depth 5, max 2000 results, **30 second timeout** (kills process if exceeded)
  - These ensure searches complete in reasonable time even from root of C: drive
  - Navigate to subdirectory first if you need deeper/more comprehensive searches
- **UI improvements**: Removed duplicate "Searching..." message, locked state indicator, warning for large result sets

### 2026-01-09 - Async Search System (UI Freeze Fix)
- **Async searches**: All expensive searches (recursive, content, ultra) now run in background goroutines
  - 300ms debounce delay before triggering search (wait for typing to stop)
  - Search cancellation via channel when new query typed or ESC pressed
  - Progressive results display as search completes
- **No more UI freeze**: Typing remains responsive during large filesystem/content searches
- **Search mode flow**: Cleaner transitions between search and normal mode
  - Enter in search: Exit to normal mode, keep filtered results for easier navigation
  - ESC with query: Clear search query and results, stay in search mode
  - ESC with empty: Exit search mode back to normal
  - "/" in normal: Enter search mode and clear any previous results
  - Search UI only visible when actively in search mode

### 2026-01-09 - Production Quality & Ultrasearch
- **Logging system**: Error/warning logger to ~/.config/scout/scout.log with 5MB auto-rotation
- **Error handling**: Fixed all silent errors across codebase - proper logging and user feedback everywhere
- **Code quality**: Replaced magic numbers with named constants, cleaner professional code
- **Ultrasearch**: New search mode scanning all mounted drives (/, /mnt/*, /media/* on Linux/WSL, volumes on macOS/Windows)
  - Tab cycles: Current Dir → Recursive → Content → Ultra → repeat
  - Drive-labeled results: "[C:] documents/file.txt"
- **Testing**: 19 unit tests covering config, search, fileops packages
- **Documentation**: Added godoc comments to all exported functions

### 2026-01-09 - Double-Click Mouse Support
- **Double-click detection**: Added 400ms threshold for detecting double-clicks on files, folders, and bookmarks
- **Consistent UX**: Single-click selects and previews; double-click activates (enter directory/open file/jump to bookmark)
- **Triple-click prevention**: Click timer resets after double-click to avoid unintended triple-click behavior
- **Mode-aware tracking**: Click state tracked per mode (normal/bookmarks) with position validation

### 2026-01-08 - Binary File Preview Fix
- **Expanded binary detection**: Added 30+ file extensions (.bson, .db, .sqlite, .tiff, .psd, .flv, .webm, .aac, .bz2, fonts, Java class files)
- **Enhanced metadata display**: Binary files now show permissions, file type, and full path instead of attempting text preview
- **Fixed display corruption**: Terminal no longer shows garbled characters when previewing images/archives/databases

### 2026-01-08 - VSCode Terminal Compatibility Fixes
- **Resize debouncing**: Skip redundant resize events to prevent flickering in VSCode integrated terminal
- **Minimum dimensions**: Added constants (60x20 min) with helpful warning message for small terminals
- **Defensive width calculations**: Fixed border alignment issues in narrow terminals by hiding file sizes when space is tight
- **Code cleanup**: Replaced magic number `-9` with named constant `uiOverhead` (9 lines for header/status/borders)
- **Overflow protection**: Added safeguards to prevent negative width calculations causing layout breaks

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
