# Release Checklist - Windows

## Validado en este entorno

- `go test ./...` ejecuta correctamente.
- `scripts/build-release.ps1` compila binarios y genera:
  - `dist/sycronizafhir-installer/`
  - `dist/sycronizafhir-installer-package.zip`
- Si Inno Setup 6 no está instalado, el script deja advertencia controlada (sin romper el build base).

## Validación recomendada en VM limpia (pendiente manual)

1. Instalar Inno Setup 6 en máquina de build.
2. Ejecutar `.\scripts\build-release.ps1`.
3. Ejecutar `dist\sycronizafhir-setup.exe`.
4. Confirmar:
   - aparece asistente de instalación y texto explicativo,
   - solicita UAC,
   - finaliza sin errores,
   - crea acceso directo en escritorio,
   - queda ejecutando en segundo plano.
5. Reiniciar Windows y validar:
   - la tarea `sycronizafhir-auto-start` arranca sola,
   - la app sigue en segundo plano,
   - abrir desde acceso directo muestra monitor/control.
6. Desinstalar desde “Aplicaciones instaladas” y validar:
   - elimina tareas `sycronizafhir-auto-start` y `sycronizafhir-auto-update`,
   - elimina carpeta `Program Files\sycronizafhir`,
   - elimina acceso directo.
