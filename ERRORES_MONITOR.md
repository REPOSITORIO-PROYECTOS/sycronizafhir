# Guia rapida de errores del monitor

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
- **Comportamiento actual**: la app busca otro puerto libre automaticamente.

