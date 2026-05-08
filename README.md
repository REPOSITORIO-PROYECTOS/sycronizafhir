# sycronizafhir

Sync middleware bidireccional entre PostgreSQL local (legacy) y Supabase, con
Control Center desktop embebido en Wails (sin browser y sin puertos HTTP
locales).

Versión actual del producto: archivo [`VERSION`](VERSION). Historial de releases: [`CHANGELOG.md`](CHANGELOG.md).

## Estructura

- `main.go`: arranque y ciclo de vida de la app Wails + workers.
- `app.go`: bindings Go expuestos al frontend Wails.
- `frontend/`: UI React + TypeScript + Vite + Tailwind.
- `internal/config`: carga/validacion de variables de entorno.
- `internal/db`: acceso a PostgreSQL local y cola de fallback SQLite.
- `internal/supabase`: clientes Postgres directo y Realtime WebSocket.
- `internal/sync`: workers outbound/inbound.
- `internal/models`: modelos de dominio.
- `cmd/dbscan/main.go`: scanner de schema y reporte por email.

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

## Ejecucion (desktop)

```bash
go mod tidy
cd frontend && npm install && cd ..
go run .
```

### Modo background

```bash
go run . --background
```

En background no abre ventana; solamente ejecuta workers de sync.

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
# Build de frontend
cd frontend && npm run build && cd ..

# Build de backend/app desktop
go build -o sycronizafhir.exe .
```

## Instalador Windows (producción)

- Build release completo:

```powershell
.\scripts\build-release.ps1
```

- Resultado esperado:
  - `dist/sycronizafhir-installer/` con binarios y scripts de instalación.
  - `dist/sycronizafhir-installer-package.zip` para distribución/auto-update.
  - `dist/agencia-ta-soluciones-setup.exe` si Inno Setup 6 está instalado en la máquina de build.

- Requisitos de build desktop:
  - Go 1.24+
  - Node 20+
  - npm (usado por Wails para compilar `frontend/`)
  - Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

- La compilación de release **no usa UPX por defecto** (recomendado para minimizar falsos positivos de AV/SmartScreen en Windows).

- El setup incluye bootstrapper de WebView2 (`MicrosoftEdgeWebview2Setup.exe`) y lo instala en modo silencioso si el runtime no está presente.

- El setup instala en `Program Files\sycronizafhir`, solicita UAC, registra autoarranque en segundo plano (SYSTEM), crea acceso directo de escritorio y deja desinstalación disponible en “Aplicaciones instaladas” de Windows.
