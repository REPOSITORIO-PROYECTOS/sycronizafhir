# sync-bridge

Sync middleware bidireccional entre PostgreSQL local (legacy) y Supabase.

## Estructura

- `cmd/app/main.go`: arranque y ciclo de vida.
- `internal/config`: carga/validacion de variables de entorno.
- `internal/db`: acceso a PostgreSQL local y cola de fallback SQLite.
- `internal/supabase`: clientes Postgres directo y Realtime WebSocket.
- `internal/sync`: workers outbound/inbound.
- `internal/models`: modelos de dominio.

## Requisitos

- Go 1.21+
- PostgreSQL local con tablas legacy y `sync_buzon_pedidos`
- Supabase (host/puerto/usuario/password DB)

## Configuracion

1. Copiar `.env.example` a `.env`.
2. Completar credenciales.
3. Configurar `SYNC_SOURCE_SCHEMA` y `SYNC_EXCLUDE_TABLES` para controlar que tablas se sincronizan.

### Sync analitico de esquema (outbound)

- El worker outbound ya no esta hardcodeado a tablas fijas.
- Descubre dinamicamente tablas del schema configurado que tengan `fecha_modificacion`.
- Excluye tablas tecnicas por `SYNC_EXCLUDE_TABLES`.
- Upsert en Supabase por conexion PostgreSQL directa usando claves primarias detectadas en la base local.

Esto permite adaptarse al esquema real legacy sin reescribir codigo por cada tabla nueva.

## Ejecucion

```bash
go mod tidy
go run ./cmd/app
```

## Escaneo completo de DB (schema snapshot)

Para tomar una "foto" completa de la estructura de la base local (tablas, columnas, PK, FK e indices):

```bash
go run ./cmd/dbscan
```

El comando genera un archivo JSON en `reports/` con prefijo `db-schema-scan-*.json`.
Ese archivo sirve como baseline para decidir cambios posteriores desde el modulo remoto.

### Envio del reporte por email

El `dbscan` puede enviar automaticamente el JSON por email (adjunto).
Por defecto envia a `ticianoat@gmail.com`, pero se puede cambiar con `DBSCAN_EMAIL_TO`.
Si falla la conexion a DB local, envia un mail de error (si SMTP esta configurado).

Variables requeridas para activar el envio:

- `DBSCAN_SMTP_HOST`
- `DBSCAN_SMTP_PORT`
- `DBSCAN_SMTP_USER`
- `DBSCAN_SMTP_PASS`
- `DBSCAN_EMAIL_FROM`

Opcional:

- `DBSCAN_EMAIL_TO` (default: `ticianoat@gmail.com`)

## Build

```bash
go build -o sync-bridge.exe ./cmd/app
```
