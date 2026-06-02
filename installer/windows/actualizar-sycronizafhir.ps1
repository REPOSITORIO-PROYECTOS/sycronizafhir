param(
    [switch]$ReopenMonitor,
    [int]$WaitPid = 0
)

$ErrorActionPreference = "Stop"

function Write-Log([string]$Message) {
    $logPath = Join-Path ${env:ProgramFiles} "sycronizafhir\update.log"
    $line = "$(Get-Date -Format o) | $Message"
    Add-Content -Path $logPath -Value $line
}

function Wait-ForParentProcess {
    param([int]$ProcessId)

    if ($ProcessId -le 0) {
        return
    }

    Write-Log "Esperando cierre del proceso padre PID=$ProcessId"
    try {
        Wait-Process -Id $ProcessId -Timeout 45 -ErrorAction Stop
    }
    catch {
        Write-Log "WARN: timeout esperando PID=$ProcessId ($($_.Exception.Message))"
    }
}

function Stop-AllAppInstances {
    $taskName = "sycronizafhir-auto-start"
    Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue

    $deadline = (Get-Date).AddSeconds(45)
    do {
        $processes = @(Get-Process -Name "sycronizafhir" -ErrorAction SilentlyContinue)
        if ($processes.Count -eq 0) {
            break
        }
        foreach ($proc in $processes) {
            try {
                Stop-Process -Id $proc.Id -Force -ErrorAction Stop
            }
            catch {
                Write-Log "WARN: no se pudo cerrar PID $($proc.Id): $($_.Exception.Message)"
            }
        }
        Start-Sleep -Seconds 1
    } while ((Get-Date) -lt $deadline)

    if (Get-Process -Name "sycronizafhir" -ErrorAction SilentlyContinue) {
        throw "No se pudieron cerrar todas las instancias de sycronizafhir"
    }

    Start-Sleep -Seconds 2
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
    Set-Content -Path $path -Value $Version -Encoding UTF8 -NoNewline
}

function Sync-InstalledExecutable {
    param([string]$InstallDir)

    $sourceExe = Join-Path $InstallDir "sycronizafhir-win10plus-amd64.exe"
    $targetExe = Join-Path $InstallDir "sycronizafhir.exe"
    if (!(Test-Path $sourceExe)) {
        throw "No se encontro binario actualizado: $sourceExe"
    }

    $sourceLength = (Get-Item $sourceExe).Length
    for ($attempt = 1; $attempt -le 8; $attempt++) {
        try {
            Copy-Item $sourceExe $targetExe -Force
            Start-Sleep -Milliseconds 500
            $targetLength = (Get-Item $targetExe).Length
            if ($targetLength -eq $sourceLength) {
                Write-Log "Ejecutable sincronizado ($targetLength bytes) intento $attempt"
                return
            }
            Write-Log "WARN: tamano distinto tras copia ($targetLength vs $sourceLength), reintento $attempt"
        }
        catch {
            Write-Log "WARN: copia exe intento ${attempt}: $($_.Exception.Message)"
        }
        Start-Sleep -Seconds 2
    }

    throw "No se pudo reemplazar sycronizafhir.exe (archivo en uso o permisos insuficientes)"
}

function Clear-WebviewCache {
    $cachePath = Join-Path $env:APPDATA "sycronizafhir\webview2"
    if (Test-Path $cachePath) {
        Remove-Item $cachePath -Recurse -Force -ErrorAction SilentlyContinue
        Write-Log "Cache WebView2 limpiada"
    }
    $markerPath = Join-Path $env:APPDATA "sycronizafhir\webview-version.marker"
    if (Test-Path $markerPath) {
        Remove-Item $markerPath -Force -ErrorAction SilentlyContinue
    }
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

    $filesToCopy = Get-ChildItem -Path $extractDir -File
    foreach ($file in $filesToCopy) {
        if ($file.Name -eq "version.txt") {
            continue
        }
        Copy-Item -Path $file.FullName -Destination (Join-Path $installDir $file.Name) -Force
    }

    Sync-InstalledExecutable -InstallDir $installDir
    Clear-WebviewCache
}

try {
    Wait-ForParentProcess -ProcessId $WaitPid

    $config = Load-Config
    if (-not $config.enabled) {
        Write-Log "Auto-update deshabilitado en config."
        exit 0
    }

    $release = Get-LatestRelease -Owner $config.github_owner -Repo $config.github_repo -Token $config.github_token
    $latestVersion = $release.tag_name
    $installedVersion = Get-InstalledVersion

    if ($installedVersion -eq $latestVersion) {
        $installDir = Join-Path ${env:ProgramFiles} "sycronizafhir"
        $sourceExe = Join-Path $installDir "sycronizafhir-win10plus-amd64.exe"
        $targetExe = Join-Path $installDir "sycronizafhir.exe"
        if ((Test-Path $sourceExe) -and (Test-Path $targetExe)) {
            $sourceLength = (Get-Item $sourceExe).Length
            $targetLength = (Get-Item $targetExe).Length
            if ($sourceLength -eq $targetLength) {
                Write-Log "Sin cambios. Version $installedVersion y ejecutable alineado."
                exit 0
            }
        }
        Write-Log "Version $installedVersion pero ejecutable desalineado; reparando..."
    }

    Write-Log "Actualizando a $latestVersion (actual: $installedVersion)"

    Stop-AllAppInstances
    Update-FromRelease -Release $release -AssetName $config.release_asset_name
    Set-InstalledVersion -Version $latestVersion

    Start-ScheduledTask -TaskName "sycronizafhir-auto-start" -ErrorAction SilentlyContinue
    Write-Log "Actualizacion completada a $latestVersion"

    if ($ReopenMonitor) {
        $installDir = Join-Path ${env:ProgramFiles} "sycronizafhir"
        $exePath = Join-Path $installDir "sycronizafhir.exe"
        if (Test-Path $exePath) {
            Start-Sleep -Seconds 2
            Start-Process -FilePath $exePath -WorkingDirectory $installDir
            Write-Log "Monitor reabierto tras actualizacion."
        }
    }
}
catch {
    Write-Log "ERROR update: $($_.Exception.Message)"
    exit 1
}
