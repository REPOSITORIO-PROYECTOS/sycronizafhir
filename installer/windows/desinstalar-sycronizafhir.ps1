$ErrorActionPreference = "Stop"

function Test-AdminRights {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw "Ejecuta este desinstalador como Administrador."
    }
}

function Remove-TaskIfExists([string]$TaskName) {
    if (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue) {
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
        Write-Host "[OK] Tarea eliminada: $TaskName" -ForegroundColor Green
    }
}

try {
    Test-AdminRights

    Remove-TaskIfExists -TaskName "sycronizafhir-auto-start"
    Remove-TaskIfExists -TaskName "sycronizafhir-auto-update"

    Get-Process -Name "sycronizafhir" -ErrorAction SilentlyContinue | Stop-Process -Force

    $desktopPath = [Environment]::GetFolderPath("Desktop")
    $shortcutNames = @(
        "sycronizafhir.lnk",
        "Agencia TA - Sync Monitor.lnk"
    )
    foreach ($shortcutName in $shortcutNames) {
        $shortcutPath = Join-Path $desktopPath $shortcutName
        if (Test-Path $shortcutPath) {
            Remove-Item $shortcutPath -Force
            Write-Host "[OK] Acceso directo eliminado del escritorio: $shortcutName" -ForegroundColor Green
        }
    }

    $installDir = Join-Path ${env:ProgramFiles} "sycronizafhir"
    if (Test-Path $installDir) {
        Remove-Item $installDir -Recurse -Force
        Write-Host "[OK] Carpeta eliminada: $installDir" -ForegroundColor Green
    }

    Write-Host "Desinstalación completada." -ForegroundColor Green
}
catch {
    Write-Host "[ERROR] $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}
