$ErrorActionPreference = "Stop"

function Write-Step([string]$Message) {
    Write-Host "`n==> $Message" -ForegroundColor Cyan
}

function Test-AdminRights {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    $isAdmin = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    if (-not $isAdmin) {
        throw "ADMIN_REQUIRED"
    }
}

function Confirm-AdminElevation {
    try {
        Test-AdminRights
    }
    catch {
        if ($_.Exception.Message -ne "ADMIN_REQUIRED") { throw }
        Write-Host "Solicitando permisos de administrador (UAC)..." -ForegroundColor Yellow
        $scriptPath = $MyInvocation.MyCommand.Path
        Start-Process -FilePath "powershell.exe" -ArgumentList "-ExecutionPolicy Bypass -File `"$scriptPath`"" -Verb RunAs
        exit 0
    }
}

function Get-TargetExeName {
    return "sycronizafhir-win10plus-amd64.exe"
}

function Test-WebView2Installed {
    $keys = @(
        "HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}",
        "HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}",
        "HKCU:\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
    )
    foreach ($key in $keys) {
        if (Test-Path $key) {
            $value = (Get-ItemProperty -Path $key -ErrorAction SilentlyContinue).pv
            if ($value -and $value -ne "0.0.0.0") { return $true }
        }
    }
    return $false
}

function Install-WebView2Runtime {
    param([string]$SourceDir)

    if (Test-WebView2Installed) {
        Write-Host "[OK] WebView2 Runtime ya instalado." -ForegroundColor Green
        return
    }

    Write-Host "WebView2 Runtime ausente. Instalando..." -ForegroundColor Yellow
    $bootstrapper = Join-Path $SourceDir "MicrosoftEdgeWebview2Setup.exe"
    if (-not (Test-Path $bootstrapper)) {
        throw "No se encontro MicrosoftEdgeWebview2Setup.exe en $SourceDir. Reintenta el setup completo o instala WebView2 Runtime manualmente desde https://go.microsoft.com/fwlink/p/?LinkId=2124703"
    }

    $process = Start-Process -FilePath $bootstrapper -ArgumentList "/silent /install" -Wait -PassThru
    if ($process.ExitCode -ne 0) {
        throw "El bootstrapper de WebView2 fallo (exit $($process.ExitCode))."
    }
    Write-Host "[OK] WebView2 Runtime instalado." -ForegroundColor Green
}

function Install-App {
    param(
        [string]$SourceDir,
        [string]$InstallDir
    )

    if (!(Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir | Out-Null
    }

    $exeName = Get-TargetExeName
    $sourceExe = Join-Path $SourceDir $exeName
    if (!(Test-Path $sourceExe)) {
        throw "No se encontró el ejecutable requerido: $sourceExe"
    }

    $targetExe = Join-Path $InstallDir "sycronizafhir.exe"
    Copy-Item $sourceExe $targetExe -Force

    $docSource = Join-Path $SourceDir "ERRORES_MONITOR.md"
    if (Test-Path $docSource) {
        Copy-Item $docSource (Join-Path $InstallDir "ERRORES_MONITOR.md") -Force
    }

    return $targetExe
}

function Register-StartupTask {
    param([string]$ExePath)

    $taskName = "sycronizafhir-auto-start"
    $action = New-ScheduledTaskAction -Execute $ExePath -Argument "--background"
    $triggerStartup = New-ScheduledTaskTrigger -AtStartup
    $triggerLogon = New-ScheduledTaskTrigger -AtLogOn
    $principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
    $settings = New-ScheduledTaskSettingsSet `
        -AllowStartIfOnBatteries `
        -StartWhenAvailable `
        -DontStopIfGoingOnBatteries `
        -MultipleInstances IgnoreNew `
        -RestartCount 999 `
        -RestartInterval (New-TimeSpan -Minutes 1)

    if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) {
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
    }

    Register-ScheduledTask -TaskName $taskName -Action $action -Trigger @($triggerStartup, $triggerLogon) -Principal $principal -Settings $settings | Out-Null
}

function Register-UpdateTask {
    param([string]$InstallDir)

    $taskName = "sycronizafhir-auto-update"
    $updateScript = Join-Path $InstallDir "actualizar-sycronizafhir.ps1"
    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$updateScript`""
    $triggerDaily = New-ScheduledTaskTrigger -Daily -At "03:00"
    $triggerLogon = New-ScheduledTaskTrigger -AtLogOn
    $principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
    $settings = New-ScheduledTaskSettingsSet `
        -AllowStartIfOnBatteries `
        -StartWhenAvailable `
        -DontStopIfGoingOnBatteries `
        -MultipleInstances IgnoreNew

    if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) {
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
    }

    Register-ScheduledTask -TaskName $taskName -Action $action -Trigger @($triggerDaily, $triggerLogon) -Principal $principal -Settings $settings | Out-Null
}

function Start-AppNow {
    Start-ScheduledTask -TaskName "sycronizafhir-auto-start"
}

function New-DesktopShortcut {
    param([string]$InstallDir)

    $desktopPath = [Environment]::GetFolderPath("Desktop")
    $shortcutPath = Join-Path $desktopPath "Agencia TA - Sync Monitor.lnk"
    $launcherScript = Join-Path $InstallDir "abrir-monitor-sycronizafhir.ps1"

    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = "powershell.exe"
    $shortcut.Arguments = "-NoProfile -ExecutionPolicy Bypass -File `"$launcherScript`""
    $shortcut.WorkingDirectory = $InstallDir
    $shortcut.WindowStyle = 1
    $shortcut.Description = "Agencia TA - Monitor de sincronización"
    $shortcut.Save()
}

try {
    Confirm-AdminElevation
    Write-Step "Instalando sycronizafhir"

    $sourceDir = Split-Path -Parent $MyInvocation.MyCommand.Path
    $installDir = Join-Path ${env:ProgramFiles} "sycronizafhir"

    Write-Step "Verificando WebView2 Runtime"
    Install-WebView2Runtime -SourceDir $sourceDir

    $exePath = Install-App -SourceDir $sourceDir -InstallDir $installDir
    Write-Host "[OK] Copiado en: $exePath" -ForegroundColor Green

    Copy-Item (Join-Path $sourceDir "actualizar-sycronizafhir.ps1") (Join-Path $installDir "actualizar-sycronizafhir.ps1") -Force
    Copy-Item (Join-Path $sourceDir "desinstalar-sycronizafhir.ps1") (Join-Path $installDir "desinstalar-sycronizafhir.ps1") -Force
    Copy-Item (Join-Path $sourceDir "detener-sycronizafhir.ps1") (Join-Path $installDir "detener-sycronizafhir.ps1") -Force
    Copy-Item (Join-Path $sourceDir "abrir-monitor-sycronizafhir.ps1") (Join-Path $installDir "abrir-monitor-sycronizafhir.ps1") -Force
    if (!(Test-Path (Join-Path $installDir "github-update-config.json"))) {
        Copy-Item (Join-Path $sourceDir "github-update-config.json") (Join-Path $installDir "github-update-config.json") -Force
    }

    Write-Step "Registrando inicio automático con Windows"
    Register-StartupTask -ExePath $exePath
    Write-Host "[OK] Tarea programada creada: sycronizafhir-auto-start" -ForegroundColor Green

    Write-Step "Registrando auto-actualización desde GitHub"
    Register-UpdateTask -InstallDir $installDir
    Write-Host "[OK] Tarea programada creada: sycronizafhir-auto-update" -ForegroundColor Green

    Write-Step "Iniciando aplicación en segundo plano"
    Start-AppNow
    Write-Host "[OK] Aplicación iniciada como tarea en segundo plano." -ForegroundColor Green

    Write-Step "Creando acceso directo en Escritorio"
    New-DesktopShortcut -InstallDir $installDir
    Write-Host "[OK] Acceso directo creado en el Escritorio." -ForegroundColor Green

    Write-Host "`nInstalación completada." -ForegroundColor Green
}
catch {
    Write-Host "`n[ERROR] $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}
