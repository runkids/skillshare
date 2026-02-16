#Requires -Version 5.1
<#
.SYNOPSIS
    Install skillshare on Windows
.DESCRIPTION
    Downloads and installs the latest skillshare release from GitHub
.EXAMPLE
    irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
#>

$ErrorActionPreference = "Stop"

# PowerShell 5.1 defaults to TLS 1.0/1.1; GitHub requires TLS 1.2+
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo = "runkids/skillshare"
$BinaryName = "skillshare"

function Write-Info { param($Message) Write-Host $Message -ForegroundColor Green }
function Write-Warn { param($Message) Write-Host $Message -ForegroundColor Yellow }
function Write-Err { param($Message) Write-Host $Message -ForegroundColor Red; exit 1 }

# Detect architecture
function Get-Arch {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { Write-Err "Unsupported architecture: $arch" }
    }
}

# Get latest version from GitHub API
function Get-LatestVersion {
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
        return $release.tag_name
    } catch {
        Write-Err "Failed to get latest version: $_"
    }
}

# Get install directory
function Get-InstallDir {
    $dir = "$env:LOCALAPPDATA\Programs\skillshare"
    if (-not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    return $dir
}

# Add to PATH if not already present
function Add-ToPath {
    param($Dir)

    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($currentPath -notlike "*$Dir*") {
        Write-Info "Adding $Dir to PATH..."
        [Environment]::SetEnvironmentVariable("Path", "$currentPath;$Dir", "User")
        $env:Path = "$env:Path;$Dir"
        return $true
    }
    return $false
}

function Install-Skillshare {
    Write-Info "Installing skillshare..."
    Write-Host ""

    $arch = Get-Arch
    $version = Get-LatestVersion
    $versionNum = $version.TrimStart("v")
    $installDir = Get-InstallDir

    $url = "https://github.com/$Repo/releases/download/$version/${BinaryName}_${versionNum}_windows_${arch}.zip"

    Write-Info "Downloading skillshare $version for windows/$arch..."

    # Create temp directory
    $tempDir = Join-Path $env:TEMP "skillshare-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

    try {
        $zipPath = Join-Path $tempDir "skillshare.zip"

        # Download
        Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing

        # Extract
        Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force

        # Find and move binary
        $exePath = Join-Path $tempDir "$BinaryName.exe"
        if (-not (Test-Path $exePath)) {
            Write-Err "Binary not found in archive"
        }

        $destPath = Join-Path $installDir "$BinaryName.exe"
        Move-Item -Path $exePath -Destination $destPath -Force

        # Add to PATH
        $pathAdded = Add-ToPath -Dir $installDir

        Write-Host ""
        Write-Info "Successfully installed skillshare to $destPath"
        Write-Host ""

        # Show version
        & $destPath version

        Write-Host ""
        if ($pathAdded) {
            Write-Warn "PATH updated. Restart your terminal for changes to take effect."
            Write-Host ""
        }

        Write-Info "Get started:"
        Write-Info "  skillshare init"
        Write-Info "  skillshare --help"

    } finally {
        # Cleanup
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Install-Skillshare
