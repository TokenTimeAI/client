#
# ttime.ai install script for Windows
# Usage: irm https://ttime.ai/install.ps1 | iex
#

$ErrorActionPreference = "Stop"

$Repo = "tokentimeai/client"
$BinaryName = "ttime"
$InstallDir = "$env:LOCALAPPDATA\ttime"
$Version = $null

function Write-Success {
    param([string]$Message)
    Write-Host "✓ $Message" -ForegroundColor Green
}

function Write-Error {
    param([string]$Message)
    Write-Host "✗ $Message" -ForegroundColor Red
}

function Write-Info {
    param([string]$Message)
    Write-Host "→ $Message" -ForegroundColor Yellow
}

# Detect architecture
$Arch = $env:PROCESSOR_ARCHITECTURE
switch ($Arch) {
    "AMD64" { $Arch = "amd64" }
    "ARM64" { $Arch = "arm64" }
    default {
        Write-Error "Unsupported architecture: $Arch"
        exit 1
    }
}

Write-Info "Installing ttime for Windows ($Arch)..."

# Get latest release
try {
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "ttime-installer" }
    $Version = $Release.tag_name
} catch {
    Write-Error "Could not determine latest version: $_"
    exit 1
}

Write-Info "Downloading ttime $Version..."

# Download URL
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/${BinaryName}_Windows_${Arch}.zip"

# Create temp directory
$TempDir = Join-Path $env:TEMP "ttime-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null

try {
    # Download
    $ZipPath = Join-Path $TempDir "$BinaryName.zip"
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath -UseBasicParsing

    # Extract
    Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force

    # Create install directory
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null

    # Move binary
    $SourcePath = Join-Path $TempDir "$BinaryName.exe"
    $DestPath = Join-Path $InstallDir "$BinaryName.exe"
    Move-Item -Path $SourcePath -Destination $DestPath -Force

    Write-Success "ttime $Version installed to $InstallDir"

    # Add to PATH if not already there
    $CurrentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($CurrentPath -notlike "*$InstallDir*") {
        Write-Info "Adding $InstallDir to PATH..."
        [Environment]::SetEnvironmentVariable("PATH", "$CurrentPath;$InstallDir", "User")
        $env:PATH = "$env:PATH;$InstallDir"
        Write-Success "Added to PATH (restart terminal to use 'ttime' command)"
    }

    Write-Host ""
    Write-Host "Run 'ttime.exe setup' to configure, then 'ttime.exe install' to set up the service."

} finally {
    # Cleanup
    Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}