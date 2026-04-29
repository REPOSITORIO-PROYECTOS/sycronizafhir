$ErrorActionPreference = "Stop"

function Require-Admin {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw "Ejecuta este desinstalador como Administrador."
    }
}

try {
    Require-Admin

    $taskName = "sycronizafhir-auto-start"
    if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) {
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
        Write-Host "[OK] Tarea eliminada: $taskName" -ForegroundColor Green
    }

    Get-Process -Name "sycronizafhir" -ErrorAction SilentlyContinue | Stop-Process -Force

    $installDir = Join-Path ${env:ProgramFiles} "sycronizafhir"
    if (Test-Path $installDir) {
        Remove-Item $installDir -Recurse -Force
        Write-Host "[OK] Carpeta eliminada: $installDir" -ForegroundColor Green
    }

    Write-Host "Desinstalacion completada." -ForegroundColor Green
}
catch {
    Write-Host "[ERROR] $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}
