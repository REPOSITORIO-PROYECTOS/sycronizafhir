$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$outputDir = Join-Path $root "dist-dev"
$packageDir = Join-Path $outputDir "sync-bridge-base"
$zipPath = Join-Path $outputDir "sync-bridge-base.zip"

Write-Host "Preparando paquete base de desarrollo..." -ForegroundColor Cyan

if (Test-Path $packageDir) { Remove-Item $packageDir -Recurse -Force }
if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
if (!(Test-Path $outputDir)) { New-Item -ItemType Directory -Path $outputDir | Out-Null }

New-Item -ItemType Directory -Path $packageDir | Out-Null

$include = @(
    "cmd",
    "internal",
    "sql",
    "go.mod",
    "go.sum",
    "README.md",
    ".env.example",
    "INSTALLAR.txt",
    "INFORME_INCIDENTE_DESARROLLO.md"
)

foreach ($item in $include) {
    $src = Join-Path $root $item
    if (Test-Path $src) {
        Copy-Item $src -Destination $packageDir -Recurse -Force
    }
}

Compress-Archive -Path (Join-Path $packageDir "*") -DestinationPath $zipPath -Force

Write-Host "Paquete generado:" -ForegroundColor Green
Write-Host $zipPath
Write-Host ""
Write-Host "Incluye codigo base para modificar y ejecutar (sin secretos)." -ForegroundColor Yellow

