$ErrorActionPreference = "Stop"

function Require-Admin {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw "Ejecuta este script como Administrador."
    }
}

try {
    Require-Admin

    $startTask = "sycronizafhir-auto-start"
    if (Get-ScheduledTask -TaskName $startTask -ErrorAction SilentlyContinue) {
        Disable-ScheduledTask -TaskName $startTask | Out-Null
        Stop-ScheduledTask -TaskName $startTask -ErrorAction SilentlyContinue
    }

    $updateTask = "sycronizafhir-auto-update"
    if (Get-ScheduledTask -TaskName $updateTask -ErrorAction SilentlyContinue) {
        Disable-ScheduledTask -TaskName $updateTask | Out-Null
    }

    Get-Process -Name "sycronizafhir" -ErrorAction SilentlyContinue | Stop-Process -Force

    Write-Host "[OK] sycronizafhir detenido y tareas deshabilitadas." -ForegroundColor Green
    Write-Host "Para volver a activar inicio automatico, ejecutar el instalador nuevamente." -ForegroundColor Yellow
}
catch {
    Write-Host "[ERROR] $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

