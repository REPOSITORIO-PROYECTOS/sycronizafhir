@echo off
setlocal
set "SCRIPT_DIR=%~dp0"
set "PS1=%SCRIPT_DIR%instalar-sycronizafhir.ps1"

if not exist "%PS1%" (
  echo [ERROR] No se encontro instalar-sycronizafhir.ps1
  pause
  exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process powershell -Verb RunAs -ArgumentList '-NoProfile -ExecutionPolicy Bypass -File ""%PS1%""'"

exit /b 0
