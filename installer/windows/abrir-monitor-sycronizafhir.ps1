$ErrorActionPreference = "Stop"

$installDir = Join-Path ${env:ProgramFiles} "sycronizafhir"
$exePath = Join-Path $installDir "sycronizafhir.exe"

if (!(Test-Path $exePath)) {
    Write-Host "No se encontro sycronizafhir.exe en $installDir" -ForegroundColor Red
    Write-Host "Reinstala desde el setup oficial." -ForegroundColor Yellow
    exit 1
}

# La ventana usa el mutex global "sycronizafhir-singleton"; si hay una instancia
# en background corriendo, la app la apaga y toma su lugar. Si ya hay una
# ventana abierta, simplemente la trae al frente y termina.
Start-Process -FilePath $exePath -WorkingDirectory $installDir
