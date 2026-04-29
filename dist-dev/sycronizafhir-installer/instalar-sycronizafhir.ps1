$ErrorActionPreference = "Stop"

function Write-Step([string]$Message) {
    Write-Host "`n==> $Message" -ForegroundColor Cyan
}

function Require-Admin {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    $isAdmin = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    if (-not $isAdmin) {
        throw "ADMIN_REQUIRED"
    }
}

function Ensure-AdminOrRelaunch {
    try {
        Require-Admin
    }
    catch {
        if ($_.Exception.Message -ne "ADMIN_REQUIRED") {
            throw
        }
        Write-Host "Solicitando permisos de administrador (UAC)..." -ForegroundColor Yellow
        $scriptPath = $MyInvocation.MyCommand.Path
        Start-Process -FilePath "powershell.exe" -ArgumentList "-ExecutionPolicy Bypass -File `"$scriptPath`"" -Verb RunAs
        exit 0
    }
}

function Get-TargetExeName {
    $osVersion = [Environment]::OSVersion.Version
    # Windows 7 = 6.1.x, Windows 10/11 = 10.x
    if ($osVersion.Major -eq 6 -and $osVersion.Minor -le 1) {
        return "sycronizafhir-win7-386.exe"
    }
    return "sycronizafhir-win10plus-amd64.exe"
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
        throw "No se encontro el ejecutable requerido: $sourceExe"
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
    $action = New-ScheduledTaskAction -Execute $ExePath
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
    param(
        [string]$InstallDir
    )

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

function Create-DesktopShortcut {
    param([string]$TargetExe)

    $desktopPath = [Environment]::GetFolderPath("Desktop")
    $shortcutPath = Join-Path $desktopPath "sycronizafhir.lnk"

    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = $TargetExe
    $shortcut.WorkingDirectory = Split-Path -Parent $TargetExe
    $shortcut.WindowStyle = 1
    $shortcut.Description = "sycronizafhir Control Center"
    $shortcut.Save()
}

try {
    Ensure-AdminOrRelaunch
    Write-Step "Instalando sycronizafhir"

    $sourceDir = Split-Path -Parent $MyInvocation.MyCommand.Path
    $installDir = Join-Path ${env:ProgramFiles} "sycronizafhir"

    $exePath = Install-App -SourceDir $sourceDir -InstallDir $installDir
    Write-Host "[OK] Copiado en: $exePath" -ForegroundColor Green

    Copy-Item (Join-Path $sourceDir "actualizar-sycronizafhir.ps1") (Join-Path $installDir "actualizar-sycronizafhir.ps1") -Force
    if (!(Test-Path (Join-Path $installDir "github-update-config.json"))) {
        Copy-Item (Join-Path $sourceDir "github-update-config.json") (Join-Path $installDir "github-update-config.json") -Force
    }

    Write-Step "Registrando inicio automatico con Windows"
    Register-StartupTask -ExePath $exePath
    Write-Host "[OK] Tarea programada creada: sycronizafhir-auto-start" -ForegroundColor Green

    Write-Step "Registrando auto-actualizacion desde GitHub"
    Register-UpdateTask -InstallDir $installDir
    Write-Host "[OK] Tarea programada creada: sycronizafhir-auto-update" -ForegroundColor Green

    Write-Step "Iniciando aplicacion ahora"
    Start-AppNow
    Write-Host "[OK] Aplicacion iniciada como tarea en segundo plano." -ForegroundColor Green

    Write-Step "Creando acceso directo en Escritorio"
    Create-DesktopShortcut -TargetExe $exePath
    Write-Host "[OK] Acceso directo creado en el Escritorio." -ForegroundColor Green

    Write-Host "`nInstalacion completada." -ForegroundColor Green
}
catch {
    Write-Host "`n[ERROR] $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}
