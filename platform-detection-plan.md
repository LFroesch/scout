# Cross-Platform Detection & Fixes Plan

## Status: WSL-specific assumptions break native Windows/macOS

## Critical Issues (Must Fix)

**Distribution:** ??? maybe?
- Package managers: Homebrew, AUR, Chocolatey, Snap/Flatpak
- Auto-update mechanism with version checking
- Demo GIF and comparison section in README

### 1. Path Normalization Breaks Native Windows
**File**: `internal/utils/drives.go:111-157`
**Problem**: `NormalizeToWSLPath()` converts `C:\` → `/mnt/c` even on native Windows
**Impact**: Drive detection fails, file operations break on PowerShell/CMD
**Fix**: Only normalize when running in WSL environment

### 2. Config Directory Not Cross-Platform
**File**: `internal/config/config.go:35`
**Problem**: Uses `~/.config/scout/` on all platforms (not Windows-idiomatic)
**Impact**: Config stored in wrong location on Windows
**Fix**:
- Windows: `%APPDATA%\scout\` or `%LOCALAPPDATA%\scout\`
- Linux/macOS: `~/.config/scout/`

### 3. Hardcoded WSL Paths
**File**: `update.go:1619-1631` (backtick key), `update.go:1476-1484` (navigation)
**Problem**: Assumes `/mnt/c` exists, blocks navigation with WSL-specific messages
**Impact**: Features broken/confusing on native Windows
**Fix**: Conditional logic based on platform detection

## Implementation Steps

### Step 1: Add Platform Detection
**New file**: `internal/utils/platform.go`
**Add functions**:
- `IsWSL() bool` - detect WSL environment
- `GetConfigDir() string` - return platform-appropriate config path
- `GetHomeRestriction() bool` - whether to restrict navigation above home

### Step 2: Fix Path Normalization
**File**: `internal/utils/drives.go`
- Only call `NormalizeToWSLPath()` when `IsWSL() == true`
- Keep native Windows paths as-is on PowerShell/CMD
- Update `removeDuplicates()` to conditionally normalize

### Step 3: Fix Config Directory
**File**: `internal/config/config.go`
- Replace hardcoded `~/.config/scout` with `GetConfigDir()`
- Test config persistence on Windows

### Step 4: Conditional WSL Features
**File**: `update.go`
- Backtick jump to `/mnt/c`: only enable in WSL
- Home directory restriction: only enforce in WSL
- Update status messages to be platform-agnostic

### Step 5: Test Windows Trash
**File**: `internal/fileops/fileops.go:135-137`
- Add error handling for PowerShell execution policy
- Consider fallback to direct file deletion if trash fails
- Test on Windows PowerShell and CMD

## Testing Matrix

| Platform | Environment | Status |
|----------|-------------|--------|
| Linux | Native | ✅ Should work |
| macOS | Native | ⚠️ Needs testing |
| Windows | PowerShell | ❌ Broken (paths) |
| Windows | CMD | ❌ Broken (paths) |
| Windows | WSL | ✅ Currently works |

## Files to Modify

1. `internal/utils/platform.go` - CREATE (platform detection)
2. `internal/utils/drives.go` - FIX (conditional normalization)
3. `internal/config/config.go` - FIX (config path)
4. `update.go` - FIX (WSL-specific features)
5. `internal/fileops/fileops.go` - IMPROVE (Windows trash)

## Success Criteria

- [ ] App runs on native Windows PowerShell without WSL
- [ ] Config files save to correct OS-specific locations
- [ ] Drive detection works on all platforms
- [ ] Clipboard operations work everywhere
- [ ] Trash/restore works (or gracefully degrades)
- [ ] No WSL assumptions in error messages

## Priority Order

1. **P0**: Platform detection + path normalization (enables Windows)
2. **P1**: Config directory fix (data persistence)
3. **P2**: Conditional WSL features (UX improvement)
4. **P3**: Windows trash improvement (nice-to-have)
