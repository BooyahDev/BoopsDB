param(
    [string]$Version = $env:BOOPS_VERSION,
    [string]$Repo = $(if ($env:BOOPS_REPO) { $env:BOOPS_REPO } else { "BooyahDev/BoopsDB" }),
    [string]$InstallDir = $(if ($env:BOOPS_INSTALL_DIR) { $env:BOOPS_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\boops" })
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = "latest"
}

switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { $arch = "amd64" }
    "ARM64" { $arch = "arm64" }
    "x86" { $arch = "386" }
    default {
        throw "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE"
    }
}

$asset = "boops-windows-$arch.zip"
if ($Version -eq "latest") {
    $url = "https://github.com/$Repo/releases/latest/download/$asset"
} else {
    $url = "https://github.com/$Repo/releases/download/$Version/$asset"
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("boops-" + [System.Guid]::NewGuid())
$zip = Join-Path $tmp $asset

New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    Write-Host "Downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $zip
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Expand-Archive -Path $zip -DestinationPath $tmp -Force
    Copy-Item -Path (Join-Path $tmp "boops.exe") -Destination (Join-Path $InstallDir "boops.exe") -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $paths = $userPath -split ";" | Where-Object { $_ }
    if ($paths -notcontains $InstallDir) {
        $newPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $InstallDir } else { "$userPath;$InstallDir" }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Host "Added $InstallDir to the user PATH. Open a new terminal before running boops."
    }

    Write-Host "Installed boops to $(Join-Path $InstallDir "boops.exe")"
} finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
