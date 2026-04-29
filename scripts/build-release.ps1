$ErrorActionPreference = "Stop"

function Write-Step([string]$Message) {
    Write-Host "`n==> $Message" -ForegroundColor Cyan
}

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
$distRoot = Join-Path $root "dist"
$packageDir = Join-Path $distRoot "sycronizafhir-installer"
$appExeName = "sycronizafhir-win10plus-amd64.exe"
$launcherExeName = "sycronizafhir-setup-launcher.exe"
$wailsBuildOutput = Join-Path $root "build\bin\sycronizafhir.exe"

function Resolve-Wails {
    $candidate = Join-Path $env:USERPROFILE "go\bin\wails.exe"
    if (Test-Path $candidate) { return $candidate }
    $cmd = Get-Command wails -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    throw "Wails CLI no encontrado. Instalar con: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
}

$wails = Resolve-Wails

Write-Step "Limpiando carpeta dist"
if (Test-Path $packageDir) { Remove-Item $packageDir -Recurse -Force }
New-Item -ItemType Directory -Path $packageDir -Force | Out-Null

Write-Step "Compilando frontend + binario principal con wails build"
Push-Location $root
try {
    & $wails build -platform windows/amd64 -trimpath -ldflags "-s -w" -clean
    if ($LASTEXITCODE -ne 0) { throw "wails build fallo (exit $LASTEXITCODE)" }
}
finally {
    Pop-Location
}

if (-not (Test-Path $wailsBuildOutput)) {
    throw "No se encontro el binario esperado en $wailsBuildOutput"
}
Copy-Item $wailsBuildOutput (Join-Path $packageDir $appExeName) -Force

Write-Step "Compilando launcher del setup"
Push-Location $root
try {
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -trimpath -ldflags "-s -w" -o (Join-Path $packageDir $launcherExeName) ./cmd/installer-launcher
    if ($LASTEXITCODE -ne 0) { throw "go build launcher fallo" }
}
finally {
    Pop-Location
}

Write-Step "Copiando scripts y documentacion del instalador"
$installerAssets = @(
    "installer/windows/instalar-sycronizafhir.ps1",
    "installer/windows/desinstalar-sycronizafhir.ps1",
    "installer/windows/actualizar-sycronizafhir.ps1",
    "installer/windows/detener-sycronizafhir.ps1",
    "installer/windows/abrir-monitor-sycronizafhir.ps1",
    "installer/windows/github-update-config.json",
    "docs/ERRORES_MONITOR.md"
)

foreach ($asset in $installerAssets) {
    $src = Join-Path $root $asset
    if (!(Test-Path $src)) {
        throw "No se encontro asset requerido: $asset"
    }
    Copy-Item $src (Join-Path $packageDir (Split-Path $asset -Leaf)) -Force
}

Write-Step "Descargando bootstrapper de WebView2 Runtime (si no esta cacheado)"
$webview2Cache = Join-Path $distRoot "MicrosoftEdgeWebview2Setup.exe"
if (-not (Test-Path $webview2Cache)) {
    Invoke-WebRequest -Uri "https://go.microsoft.com/fwlink/p/?LinkId=2124703" -OutFile $webview2Cache -UseBasicParsing
}
Copy-Item $webview2Cache (Join-Path $packageDir "MicrosoftEdgeWebview2Setup.exe") -Force

Write-Step "Generando ZIP del instalador"
$zipPath = Join-Path $distRoot "sycronizafhir-installer-package.zip"
if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
Compress-Archive -Path (Join-Path $packageDir "*") -DestinationPath $zipPath -Force

Write-Step "Compilando setup con Inno Setup (si esta disponible)"
$isccCandidates = @(
    "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
    "${env:ProgramFiles}\Inno Setup 6\ISCC.exe"
)
$iscc = $isccCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1
if ($iscc) {
    & $iscc (Join-Path $root "installer/windows/sycronizafhir-setup.iss") "/DSourceDir=$packageDir" "/DOutputDir=$distRoot"
    if ($LASTEXITCODE -ne 0) { throw "ISCC fallo (exit $LASTEXITCODE)" }
    Write-Host "[OK] Setup .exe generado en dist/" -ForegroundColor Green
} else {
    Write-Host "[WARN] Inno Setup no detectado. Se genero paquete ZIP en: $zipPath" -ForegroundColor Yellow
    Write-Host "       Instala Inno Setup 6 y vuelve a ejecutar este script para crear el setup .exe." -ForegroundColor Yellow
}

Write-Host "`nBuild release completado." -ForegroundColor Green
