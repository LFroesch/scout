# Cross-Platform Support Plan

## Current State

Scout runs on **Linux** and **WSL** (tested). Cross-platform code paths exist for macOS (drive detection, trash, file opening) and Windows (drive detection, trash via PowerShell) but are untested. Goal: full macOS + native Windows support.

### What Already Works Cross-Platform

- **Clipboard**: `atotto/clipboard` handles xclip/xsel (Linux), pbcopy (macOS), Windows clipboard API
- **File opening**: `open-golang` uses xdg-open (Linux), open (macOS), start (Windows)
- **Git**: works everywhere
- **Editor support**: configurable via `config.Editor`, fallback chain
- **Search skip dirs**: already has per-OS entries in `search.go`
- **File type detection**: extension-based, platform-agnostic

## What Needs Fixing

### 1. Platform Detection (NEW — `internal/utils/platform.go`)

No detection exists yet. Need:

```go
// IsWSL detects WSL environment (check /proc/version for "microsoft" or "WSL")
func IsWSL() bool

// ConfigDir returns platform-appropriate config directory
// Linux/macOS: ~/.config/scout/
// Windows: %APPDATA%\scout\
func ConfigDir() string

// DataDir returns platform-appropriate data directory (for logs, undo metadata, last_dir)
// Linux/macOS: ~/.config/scout/ (same as config for now)
// Windows: %LOCALAPPDATA%\scout\
func DataDir() string
```

### 2. Config Directory — Hardcoded `~/.config/scout/`

All config/data paths use `filepath.Join(homeDir, ".config", "scout")`. On Windows this creates `C:\Users\foo\.config\scout\` which works but isn't idiomatic.

**Files to update:**
- `internal/config/config.go:35,135,201` — config file path
- `internal/logger/logger.go:28` — log file path
- `internal/fileops/fileops.go:180` — undo metadata dir
- `model.go:1205` — last_dir file for shell cd integration

**Fix**: Replace all with `platform.ConfigDir()` / `platform.DataDir()`. Use `os.UserConfigDir()` (Go stdlib, returns `%APPDATA%` on Windows, `~/.config` on Linux/macOS).

### 3. Path Normalization — `NormalizeToWSLPath()` Runs Unconditionally

**File**: `internal/utils/drives.go:111-157`

`NormalizeToWSLPath()` converts `C:\` → `/mnt/c` on ALL platforms. On native Windows this breaks everything. `removeDuplicates()` also calls it unconditionally.

**Fix**: Guard with `IsWSL()` — only normalize when actually in WSL. On native Windows, keep Windows-native paths (`C:\foo`).

### 4. Drive Detection — `GetMountedDrives()` / `GetDriveLabel()`

**File**: `internal/utils/drives.go:14-108`

Already has `runtime.GOOS` switch for Windows/macOS/Linux. The Windows path checks `A:\` through `Z:\` with `os.Stat()`. The macOS path reads `/Volumes`. These exist but are untested.

**Fix**: Test and verify. The Windows drive detection looks correct. macOS `/Volumes` scanning should work. May need minor fixes.

### 5. Backtick Key — Hardcoded `/mnt/c` Jump

**File**: `update.go:2201-2214`

Backtick always tries `/mnt/c`. On macOS this is meaningless. On native Windows it should jump to `C:\`.

**Fix**:
- WSL → `/mnt/c` (current behavior)
- Native Windows → `C:\` or user's system drive
- macOS → `/` or `~` (no Windows drives)
- Update help text in `view.go:1160`

### 6. Trash / Undo — Partially Cross-Platform

**File**: `internal/fileops/fileops.go`

`MoveToTrash()` (lines 127-150) already has OS switch:
- macOS: `osascript` AppleScript ✅ (needs testing)
- Windows: PowerShell `Microsoft.VisualBasic` ✅ (needs testing, may fail on execution policy)
- Linux: `gio trash` / `trash-put` ✅ (works)

`DeleteWithUndo()` (lines 221-234) only knows Linux (`~/.local/share/Trash/files/`) and macOS (`~/.Trash/`) trash locations. Missing Windows Recycle Bin path for undo.

**Fix**:
- Test macOS AppleScript trash
- Add PowerShell execution policy error handling
- For undo on Windows: may need to skip undo or use a different approach (Recycle Bin isn't easily accessible from Go)

## Implementation Order

### Phase 1: Platform Foundation (enables everything else)

1. **Create `internal/utils/platform.go`** — `IsWSL()`, `ConfigDir()`, `DataDir()`
2. **Update config/logger/fileops paths** — use `ConfigDir()`/`DataDir()` everywhere
3. **Guard `NormalizeToWSLPath()`** — only in WSL

### Phase 2: macOS Support (low-hanging fruit)

4. **Test on macOS** — drive detection, trash, file opening, clipboard
5. **Fix backtick key** — make it WSL-only or platform-aware
6. **Verify trash undo** — macOS `~/.Trash/` path in `DeleteWithUndo()`

### Phase 3: Native Windows Support

7. **Test drive detection** — A-Z loop in `GetMountedDrives()`
8. **Test/fix PowerShell trash** — execution policy handling, fallback
9. **Backtick → `C:\`** on native Windows
10. **Windows undo** — decide approach (skip undo, or find Recycle Bin path)

## Files to Modify

| File | Change |
|------|--------|
| `internal/utils/platform.go` | **CREATE** — IsWSL, ConfigDir, DataDir |
| `internal/utils/drives.go` | Guard NormalizeToWSLPath with IsWSL |
| `internal/config/config.go` | Use platform.ConfigDir() |
| `internal/logger/logger.go` | Use platform.DataDir() |
| `internal/fileops/fileops.go` | Use platform.DataDir() for undo dir, test trash |
| `model.go` | Use platform.DataDir() for last_dir |
| `update.go` | Platform-aware backtick key |
| `view.go` | Update backtick help text |

## Testing Matrix

| Platform | Current | Target |
|----------|---------|--------|
| Linux | ✅ Tested | ✅ No changes needed |
| WSL | ✅ Tested | ✅ Guard WSL-specific code |
| macOS | ⚠️ Untested | ✅ Test + minor fixes |
| Windows (native) | ❌ Broken | ✅ Config dir + path normalization + trash |

## Success Criteria

- [ ] `go build` succeeds on Linux, macOS, Windows
- [ ] Config saves to OS-idiomatic location
- [ ] Drive detection works on all platforms
- [ ] Clipboard works everywhere (already does via library)
- [ ] Trash works or gracefully falls back to permanent delete with warning
- [ ] No WSL assumptions leak into non-WSL environments
- [ ] Backtick key does something useful per platform (or is hidden)
