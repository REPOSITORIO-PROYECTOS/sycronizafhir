$ErrorActionPreference = "Stop"

function Write-Log([string]$Message) {
    $logPath = Join-Path ${env:ProgramFiles} "sycronizafhir\update.log"
    $line = "$(Get-Date -Format o) | $Message"
    Add-Content -Path $logPath -Value $line
}

function Load-Config {
    $configPath = Join-Path ${env:ProgramFiles} "sycronizafhir\github-update-config.json"
    if (!(Test-Path $configPath)) {
        throw "No existe github-update-config.json en $configPath"
    }
    return Get-Content $configPath -Raw | ConvertFrom-Json
}

function Get-LatestRelease {
    param(
        [string]$Owner,
        [string]$Repo,
        [string]$Token
    )

    $uri = "https://api.github.com/repos/$Owner/$Repo/releases/latest"
    $headers = @{
        "Accept" = "application/vnd.github+json"
        "User-Agent" = "sycronizafhir-updater"
    }
    if ($Token) {
        $headers["Authorization"] = "Bearer $Token"
    }
    return Invoke-RestMethod -Method Get -Uri $uri -Headers $headers -TimeoutSec 30
}

function Get-InstalledVersion {
    $path = Join-Path ${env:ProgramFiles} "sycronizafhir\version.txt"
    if (!(Test-Path $path)) {
        return ""
    }
    return (Get-Content $path -Raw).Trim()
}

function Set-InstalledVersion([string]$Version) {
    $path = Join-Path ${env:ProgramFiles} "sycronizafhir\version.txt"
    Set-Content -Path $path -Value $Version -Encoding UTF8
}

function Update-FromRelease {
    param(
        $Release,
        [string]$AssetName
    )

    $asset = $Release.assets | Where-Object { $_.name -eq $AssetName } | Select-Object -First 1
    if (-not $asset) {
        throw "No se encontró asset '$AssetName' en release $($Release.tag_name)"
    }

    $tempDir = Join-Path $env:TEMP "sycronizafhir-update"
    if (Test-Path $tempDir) {
        Remove-Item $tempDir -Recurse -Force
    }
    New-Item -ItemType Directory -Path $tempDir | Out-Null

    $zipPath = Join-Path $tempDir $asset.name
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zipPath -UseBasicParsing

    $extractDir = Join-Path $tempDir "extracted"
    Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

    $installDir = Join-Path ${env:ProgramFiles} "sycronizafhir"
    Copy-Item -Path (Join-Path $extractDir "*") -Destination $installDir -Recurse -Force
}

try {
    $config = Load-Config
    if (-not $config.enabled) {
        Write-Log "Auto-update deshabilitado en config."
        exit 0
    }

    $release = Get-LatestRelease -Owner $config.github_owner -Repo $config.github_repo -Token $config.github_token
    $latestVersion = $release.tag_name
    $installedVersion = Get-InstalledVersion

    if ($installedVersion -eq $latestVersion) {
        Write-Log "Sin cambios. Versión actual: $installedVersion"
        exit 0
    }

    Write-Log "Nueva versión detectada: $latestVersion (actual: $installedVersion)"

    Stop-ScheduledTask -TaskName "sycronizafhir-auto-start" -ErrorAction SilentlyContinue
    Get-Process -Name "sycronizafhir" -ErrorAction SilentlyContinue | Stop-Process -Force

    Update-FromRelease -Release $release -AssetName $config.release_asset_name
    Set-InstalledVersion -Version $latestVersion

    Start-ScheduledTask -TaskName "sycronizafhir-auto-start"
    Write-Log "Actualización completada a versión $latestVersion"
}
catch {
    Write-Log "ERROR update: $($_.Exception.Message)"
    exit 1
}
