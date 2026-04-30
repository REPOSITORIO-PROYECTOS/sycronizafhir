$ErrorActionPreference = "Stop"

$installDir = Join-Path ${env:ProgramFiles} "sycronizafhir"
$exePath = Join-Path $installDir "sycronizafhir.exe"
$taskName = "sycronizafhir-auto-start"

if (!(Test-Path $exePath)) {
    Write-Host "No se encontro sycronizafhir.exe en $installDir" -ForegroundColor Red
    Write-Host "Reinstala desde el setup oficial." -ForegroundColor Yellow
    exit 1
}

function Ensure-AutoStartTask {
    param(
        [string]$TaskName,
        [string]$ExePath
    )

    $existing = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($existing) { return }

    try {
        $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent().Name
        $action = New-ScheduledTaskAction -Execute $ExePath -Argument "--background"
        $trigger = New-ScheduledTaskTrigger -AtLogOn -User $currentUser
        $principal = New-ScheduledTaskPrincipal -UserId $currentUser -LogonType Interactive -RunLevel Limited
        $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -StartWhenAvailable -DontStopIfGoingOnBatteries -MultipleInstances IgnoreNew

        Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings | Out-Null
        Write-Host "[OK] Tarea de autoarranque regenerada ($TaskName)." -ForegroundColor Green
    }
    catch {
        Write-Host "[WARN] No se pudo regenerar la tarea de autoarranque: $($_.Exception.Message)" -ForegroundColor Yellow
    }
}

Ensure-AutoStartTask -TaskName $taskName -ExePath $exePath

# La ventana usa el mutex global "sycronizafhir-singleton"; si hay una instancia
# en background corriendo, la app la apaga y toma su lugar. Si ya hay una
# ventana abierta, simplemente la trae al frente y termina.
Start-Process -FilePath $exePath -WorkingDirectory $installDir
