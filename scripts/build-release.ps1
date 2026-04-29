$ErrorActionPreference = "Stop"

function Write-Step([string]$Message) {
    Write-Host "`n==> $Message" -ForegroundColor Cyan
}

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
$distRoot = Join-Path $root "dist"
$packageDir = Join-Path $distRoot "sycronizafhir-installer"
$appExeName = "sycronizafhir-win10plus-amd64.exe"
$launcherExeName = "sycronizafhir-setup-launcher.exe"

Write-Step "Preparando carpeta dist"
if (Test-Path $packageDir) { Remove-Item $packageDir -Recurse -Force }
New-Item -ItemType Directory -Path $packageDir -Force | Out-Null

Write-Step "Compilando binario principal"
Push-Location $root
try {
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -trimpath -ldflags "-s -w" -o (Join-Path $packageDir $appExeName) ./cmd/app
    $env:GOARCH = "386"
    go build -trimpath -ldflags "-s -w" -o (Join-Path $packageDir "sycronizafhir-win7-386.exe") ./cmd/app
}
finally {
    Pop-Location
}

Write-Step "Compilando launcher del setup"
Push-Location $root
try {
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -trimpath -ldflags "-s -w" -o (Join-Path $packageDir $launcherExeName) ./cmd/installer-launcher
}
finally {
    Pop-Location
}

Write-Step "Copiando scripts y documentación del instalador"
$installerAssets = @(
    "installer/windows/instalar-sycronizafhir.ps1",
    "installer/windows/desinstalar-sycronizafhir.ps1",
    "installer/windows/actualizar-sycronizafhir.ps1",
    "installer/windows/detener-sycronizafhir.ps1",
    "installer/windows/github-update-config.json",
    "docs/ERRORES_MONITOR.md"
)

foreach ($asset in $installerAssets) {
    $src = Join-Path $root $asset
    if (!(Test-Path $src)) {
        throw "No se encontró asset requerido: $asset"
    }
    Copy-Item $src (Join-Path $packageDir (Split-Path $asset -Leaf)) -Force
}

Write-Step "Generando ZIP del instalador"
$zipPath = Join-Path $distRoot "sycronizafhir-installer-package.zip"
if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
Compress-Archive -Path (Join-Path $packageDir "*") -DestinationPath $zipPath -Force

Write-Step "Compilando setup con Inno Setup (si está disponible)"
$isccCandidates = @(
    "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
    "${env:ProgramFiles}\Inno Setup 6\ISCC.exe"
)
$iscc = $isccCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1
if ($iscc) {
    & $iscc (Join-Path $root "installer/windows/sycronizafhir-setup.iss") "/DSourceDir=$packageDir" "/DOutputDir=$distRoot"
    Write-Host "[OK] Setup .exe generado en dist/" -ForegroundColor Green
} else {
    Write-Host "[WARN] Inno Setup no detectado. Se generó paquete ZIP en: $zipPath" -ForegroundColor Yellow
    Write-Host "       Instala Inno Setup 6 y vuelve a ejecutar este script para crear el setup .exe." -ForegroundColor Yellow
}

Write-Host "`nBuild release completado." -ForegroundColor Green
