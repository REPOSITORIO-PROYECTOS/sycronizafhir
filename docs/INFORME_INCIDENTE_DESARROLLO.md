# Incidente Sync-Bridge (Handoff a Desarrollo)

## Resumen

Se valido el comportamiento del middleware `sync-bridge` en entorno local con DB de prueba.  
El problema principal actual no es compilacion del binario, sino **configuracion/conectividad** (DB local, realtime y SMTP).

## Estado actual (actualizado)

### 1) Bloqueante real: conexion a PostgreSQL local (segun entorno)

- Sintoma observado: `password authentication failed` en `LOCAL_POSTGRES_URL` (en entorno original).
- Impacto: la app no completa inicio si falla DB local.
- Estado: **vigente** en entorno con credenciales incorrectas.

### 2) Realtime Supabase (inbound) dependiente de URL real

- Si `SUPABASE_REALTIME_URL` esta en placeholder (`your-project.supabase.co`) el inbound entra en bucle de reconexion.
- Impacto: outbound puede funcionar, inbound no.
- Estado: **vigente** hasta cargar URL real.

### 3) SMTP para reportes

- `dbscan` ahora soporta `MAIL_*` y fallback `DBSCAN_*`.
- Si credenciales Gmail no son validas/app-password, falla envio con `535 5.7.8`.
- Impacto: escaneo puede correr, pero notificacion por mail falla.
- Estado: **vigente** hasta credenciales SMTP correctas.

### 4) Punto corregido respecto al reporte previo

- El bloqueo por SQLite/CGO fue mitigado en codigo:
  - se reemplazo `go-sqlite3` por `modernc.org/sqlite` (pure Go).
- Impacto: no requiere `gcc` para correr la cola SQLite.
- Estado: **resuelto en el codigo base actual**.

## Mapa tecnico vigente

### Outbound (local -> destino PostgreSQL)

- Descubre tablas dinamicamente por schema (`SYNC_SOURCE_SCHEMA`).
- Filtra por presencia de `fecha_modificacion`.
- Excluye tecnicas (`SYNC_EXCLUDE_TABLES`).
- Upsert por PK detectada (`ON CONFLICT`) al destino configurado (`HOST_SUPABASE`, etc.).

### Inbound (realtime -> buzon local)

- Escucha `SUPABASE_REALTIME_URL`/canal/tabla.
- Inserta en `sync_buzon_pedidos`.
- Trigger local marca `procesado`/`error_log`.

## Acciones pedidas a Desarrollo (priorizadas)

1. **P0** Normalizar `.env` para una sola verdad por ambiente (local/test/prod).
2. **P0** Definir validacion de startup por componentes (local DB, destino DB, realtime, SMTP) con salida clara.
3. **P1** Agregar flag para desactivar inbound realtime en ambientes sin URL real.
4. **P1** Endurecer manejo de DNS/red (timeouts, backoff, mensajes accionables).
5. **P0 Seguridad** Rotar secretos expuestos y sacar credenciales de archivos compartidos.

## Evidencia de ejecucion local

- `dbscan` escaneo OK sobre DB de prueba local.
- `app` inicia monitor `http://127.0.0.1:8088` y workers.
- Falla observable restante en pruebas: realtime placeholder y/o SMTP credenciales.

## Archivos base clave para modificar

- `cmd/app/main.go`
- `cmd/dbscan/main.go`
- `internal/config/config.go`
- `internal/sync/outbound.go`
- `internal/sync/inbound.go`
- `internal/db/queue_sqlite.go`
- `internal/supabase/pg_upsert.go`

