$ErrorActionPreference = "Stop"

$Repo = "LFroesch/scout"
$BinaryName = "scout"
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $HOME ".local\bin" }

function Get-TargetArch {
    switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
        "X64" { return "amd64" }
        "Arm64" { return "arm64" }
        default { throw "Unsupported architecture: $([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)" }
    }
}

function Get-Version {
    if ($env:VERSION) { return $env:VERSION }
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    if (-not $release.tag_name) { throw "Unable to resolve release version" }
    return $release.tag_name
}

$version = Get-Version
$arch = Get-TargetArch
$asset = "$BinaryName-windows-$arch.exe"
$baseUrl = "https://github.com/$Repo/releases/download/$version"
$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "$BinaryName-install-$([guid]::NewGuid().ToString('N'))"

try {
    New-Item -ItemType Directory -Path $tmpDir | Out-Null
    $tmpExe = Join-Path $tmpDir $asset
    $tmpChecksums = Join-Path $tmpDir "checksums.txt"
    Invoke-WebRequest -Uri "$baseUrl/$asset" -OutFile $tmpExe
    try {
        Invoke-WebRequest -Uri "$baseUrl/checksums.txt" -OutFile $tmpChecksums
        $expected = Get-Content $tmpChecksums | Where-Object { $_ -match ([regex]::Escape($asset) + '$') } | ForEach-Object { ($_ -split '\s+')[0] } | Select-Object -First 1
        if ($expected) {
            $actual = (Get-FileHash -Path $tmpExe -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($actual -ne $expected.ToLowerInvariant()) { throw "Checksum verification failed" }
        }
    } catch {
        Write-Warning "checksums.txt not found or unreadable; skipping checksum verification"
    }
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $target = Join-Path $InstallDir ($BinaryName + ".exe")
    Copy-Item -Path $tmpExe -Destination $target -Force
    Write-Host "Installed $BinaryName to $target"
    if (-not (($env:Path -split ';') -contains $InstallDir)) {
        Write-Warning "$InstallDir is not in PATH"
        Write-Warning "Add this to your PowerShell profile: `$env:Path += ';$InstallDir'"
    }
} finally {
    if (Test-Path $tmpDir) { Remove-Item -Path $tmpDir -Recurse -Force }
}
