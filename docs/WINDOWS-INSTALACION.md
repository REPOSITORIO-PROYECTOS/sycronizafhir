# Instalación Windows (Producción)

## Qué instala

- Servicio de sincronización `sycronizafhir` en `Program Files\sycronizafhir`.
- Arranque automático en segundo plano mediante tareas programadas con privilegios altos.
- Acceso directo en escritorio para abrir el monitor/control.

## Instalación

1. Ejecutar `sycronizafhir-setup.exe`.
2. Aceptar permisos UAC cuando Windows lo solicite.
3. Esperar a que finalice el asistente.

Al terminar, la aplicación queda ejecutándose en segundo plano.

## Operación diaria

- Para abrir el monitor visual, usar el acceso directo `sycronizafhir` del escritorio.
- Para detener temporalmente la ejecución automática:
  - Ejecutar `detener-sycronizafhir.ps1` como administrador.

## Desinstalación

- Desde Configuración de Windows > Aplicaciones > Aplicaciones instaladas > `sycronizafhir`.
- O ejecutando `desinstalar-sycronizafhir.ps1` con permisos de administrador.

La desinstalación elimina tareas programadas, proceso en ejecución, acceso directo y carpeta de instalación.
