# Guia rapida de errores del monitor

## no se abre la ventana de la app

- **Que significa**: el ejecutable no puede iniciar la UI embebida.
- **Causa mas comun**:
  - Falta `Microsoft Edge WebView2 Runtime`.
  - Instalacion incompleta en `Program Files\sycronizafhir`.
- **Que revisar**:
  - Ejecutar nuevamente el setup (`agencia-ta-soluciones-setup.exe`) como admin.
  - Verificar que exista `MicrosoftEdgeWebview2Setup.exe` en el paquete de instalacion.
  - Ver logs recientes en la vista `Logs` del Control Center.

## realtime websocket: bad handshake

- **Que significa**: Supabase Realtime rechazo el handshake HTTP del websocket.
- **Causa mas comun**:
  - `SUPABASE_SERVICE_ROLE_KEY` invalida o en placeholder.
  - Canal/schema/table realtime no coinciden con el proyecto.
- **Que revisar**:
  - `SUPABASE_SERVICE_ROLE_KEY`
  - `SUPABASE_REALTIME_URL`
  - `SUPABASE_REALTIME_CHANNEL`
  - `SUPABASE_REALTIME_SCHEMA`
  - `SUPABASE_REALTIME_TABLE`

## password authentication failed (supabase_postgres)

- **Que significa**: usuario/password incorrectos para el endpoint configurado.
- **Que revisar**:
  - `SUPABASE_DB_URL` (si esta definida, tiene prioridad)
  - o `HOST_SUPABASE`, `PUERTO_SUPABASE`, `USUARIO_SUPABASE`, `CONTRASENA_SUPABASE`

## Tenant or user not found

- **Que significa**: usuario no valido para pooler 6543.
- **Causa comun**: usar `postgres` en vez de `postgres.<project-ref>`.

## monitor port ocupado

- **Que significa**: 8088/8089/8090 estaban en uso.
- **Estado actual**: en arquitectura Wails este escenario ya no aplica para UI.
- **Nota**: la ventana desktop usa IPC nativo (sin puertos HTTP locales para el monitor).

## instancia en segundo plano no cede control a la ventana

- **Que significa**: hay una instancia `--background` ejecutando workers y al abrir la UI no toma control.
- **Que revisar**:
  - Confirmar que no haya multiples procesos `sycronizafhir.exe` colgados.
  - Ejecutar `detener-sycronizafhir.ps1` y abrir nuevamente desde el acceso directo.
  - Si persiste, reinstalar para reprovisionar tareas programadas.

