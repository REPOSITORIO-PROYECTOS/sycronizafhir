# Registro de cambios

Formato basado en [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/).
Versiones alineadas con el archivo [`VERSION`](VERSION) en la raíz del repositorio.

## [1.5.12] - 2026-06-30

### Corregido

- **Panel Sincronización**: `GetLastDataAudit` devuelve siempre la auditoría más reciente (memoria o SQLite), no una copia vieja en memoria.
- **Panel Sincronización**: refresco automático al terminar auditoría programada, subidas outbound o ciclos de imágenes; botón **Actualizar panel**.
- **Panel Sincronización**: columnas renombradas (Tabla local / Filas nube) y texto que distingue outbound (~1 min) vs auditoría (~6 h).

## [1.5.11] - 2026-06-23

### Agregado

- **Soporte**: vista **Soporte** en el Control Center para generar reportes ZIP (estado, config, escaneo, logs) y abrir carpeta de errores/incidentes.
- **Registro de incidentes**: errores/warns de componentes y escaneos se guardan en `%APPDATA%\sycronizafhir\errors\`.
- **Log a archivo**: salida adicional del runtime en AppData para diagnóstico en campo.
- **Checklist operativo**: `docs/CHECKLIST_OPERACION_SYNC.md` con pasos P0–P3 para máquinas en producción.

## [1.5.10] - 2026-06-10

### Corregido

- **image_sync**: imágenes inexistentes en disco (`file_missing` / ruta inválida) se omiten sin cola de reintentos ni contarse como fallo.
- **Config instalada**: `.env` se carga desde la carpeta del ejecutable (`Program Files\sycronizafhir`), no solo del directorio de trabajo del proceso.
- **Autoarranque**: tarea programada `sycronizafhir-auto-start` usa `WorkingDirectory` en la carpeta de instalación.
- **Clave embebida**: `SUPABASE_SERVICE_ROLE_KEY` actualizada a `sb_secret_` (antes estaba `sb_publishable_` y causaba `rls_auth`).

## [1.5.5] - 2026-06-09

### Agregado

- **Sync de imágenes de productos**: sube fotos desde rutas locales Windows (`C:\Sys_Image\...`) a Supabase Storage y reemplaza `prod_imagen` por URL pública solo en Supabase (PostgreSQL local intacto).
- Worker `image_sync` automático (default cada 5 min) con cola de reintentos SQLite y cache por archivo.
- Integración en outbound/reconcile: los upserts de `productos` ya no vuelven a escribir rutas `C:\...` en la nube.
- UI **Sincronización** → card **Imágenes de productos** → botón **Subir imágenes ahora**.
- SQL `003_supabase_storage_productos.sql`: bucket público `productos` (sin política SELECT amplia que permita listar todo el bucket).

### Configuración

- `IMAGE_SYNC_ENABLED`, `IMAGE_SYNC_INTERVAL_SECONDS`, `SUPABASE_STORAGE_BUCKET_PRODUCTOS`, `IMAGE_LOCAL_BASE_PATH`, `SUPABASE_URL` (requerida con image sync activo).

## [1.5.4] - 2026-06-09

### Corregido

- Auditoría de datos: hash de filas normaliza padding `char`, numéricos y fechas para evitar miles de falsos "Diff" en `clientes`/`productos`.
- **Subir seleccionadas** re-audita al terminar y actualiza la tabla de pendientes.
- **Auditar y subir diffs** sincroniza siempre en acción manual (no depende solo de Auto-sync).
- Sync diff de `clientes`/`productos` ya no aborta con mensaje genérico de "revisión de detalle".

## [1.5.3] - 2026-06-05

### Corregido

- Upsert a Supabase: normalización flexible de columnas array (`[]interface{}`, slices tipados, literales `{1,2,3}` y JSON) en lectura local y antes de cada INSERT; evita `cannot find encode plan` / OID 0 con pgx en protocolo simple.

## [1.5.2] - 2026-06-02

### Corregido

- Upsert a Supabase: columnas array (`integer[]`, `text[]`) leídas como `[]interface{}` ya se normalizan antes del INSERT (evita `cannot find encode plan` / OID 0).

## [1.5.1] - 2026-06-02

### Corregido

- Auto-update: espera cierre del proceso, reintentos al copiar `sycronizafhir.exe`, verifica tamano y limpia cache WebView2.
- Deteccion de version usa el binario en ejecucion (no solo `version.txt`) para evitar loop "actualizacion disponible" con UI vieja.
- Sidebar muestra version real del ejecutable; build sincroniza `VERSION`, `package.json` y `wails.json`.

## [1.5.0] - 2026-06-02

### Agregado

- Módulo **Sincronización**: compara local vs Supabase por tabla (conteos, faltantes, cambios por hash de contenido).
- Selector de tablas habilitadas persistido en `%APPDATA%\sycronizafhir\sync-tables.json` (default: `clientes`, `productos`; mapeo `articulos` → `productos`).
- Botones **Auditar ahora**, **Auditar y subir diffs** y **Subir seleccionadas**.
- Worker de **auditoría automática cada 6 h** (`SYNC_AUDIT_INTERVAL_HOURS`, default 6) con auto-sync opcional.
- Errores de upsert en `clientes`/`productos`/`articulos` indican que requieren revisión de detalle; otras tablas fallan directo.

## [1.4.7] - 2026-06-01

### Corregido

- Progreso de bootstrap en archivo dedicado `%APPDATA%\\sycronizafhir\\bootstrap_state.db` (ya no compite con outbound en `sync_queue.db`).
- Mutex por proceso, `busy_timeout` 30 s y mas reintentos en SQLite; persistencia intermedia no aborta la carga si falla un guardado.

## [1.4.6] - 2026-06-01

### Corregido

- Cola SQLite: una sola conexion compartida por proceso (WAL, `busy_timeout`, reintentos en `sync_state`) para evitar `database is locked (SQLITE_BUSY)` durante bootstrap cuando outbound/UI escribian al mismo `sync_queue.db`.
- Bootstrap persiste progreso en SQLite como maximo cada 2 s por chunk (siempre al iniciar, fallar, completar tabla o terminar la carga).

## [1.4.5] - 2026-05-28

### Corregido

- Bootstrap reanuda automaticamente al abrir la app si quedo en estado `running` (antes la UI mostraba progreso pero el worker no arrancaba tras reiniciar/actualizar).

## [1.4.4] - 2026-05-28

### Cambiado

- Bootstrap mucho más rápido: upsert por lotes (75 filas por query) en lugar de 1 INSERT por fila.
- Tamaño de lote configurable con `BOOTSTRAP_CHUNK_SIZE` (default 500, antes 200).
- Cache de metadatos de tablas Supabase durante la carga inicial.
- Logs de bootstrap cada 1000 filas (menos ruido en tablas grandes).
- Auto-update copia el binario a `sycronizafhir.exe` tras descargar el ZIP.

## [1.4.3] - 2026-05-28

### Añadido

- Logs visibles en bootstrap, outbound e inbound cuando se suben o reciben filas/pedidos.
- Progreso de carga inicial en Conexiones leído en vivo desde SQLite (filas/tablas ya no quedan en 0/0).

## [1.4.2] - 2026-05-28

### Corregido

- Cola SQLite (`SQLITE_QUEUE_PATH`): rutas relativas se resuelven a `%APPDATA%\\sycronizafhir\\sync_queue.db` para evitar fallos al iniciar bootstrap en la app Wails (error SQLite 14 / "out of memory" por CWD sin permisos de escritura).
- Se crea el directorio padre de la base SQLite antes de abrirla.

## [1.4.0] - 2026-05-07

### Añadido

- Resolución automática de fuente PostgreSQL local con fallback configurable (`DB_SOURCE_MODE=auto-fallback`, prioridad `docker,local`) y diagnóstico de candidatos.
- Carga inicial completa (bootstrap) desde local hacia Supabase con procesamiento por lotes, persistencia de estado y métricas de progreso.
- Nueva sección en `Conexiones` para ejecutar descubrimiento de fuente DB e iniciar/monitorear bootstrap desde la UI.
- Scripts SQL de preparación para Supabase (`sql/000_supabase_prep_completo.sql` y `sql/prep_supabase_minimo.sql`) y pruebas de integración para validar upsert real.

### Cambiado

- Arranque de workers sincroniza metadatos de runtime usando la fuente DB efectivamente resuelta (local o Docker).
- `sql/002_supabase_sync_tables_pedidos_productos_clientes.sql` pasa a ser un redirect compatible hacia el script consolidado `000`.
- `sql/001_sync_bridge_setup.sql` incorpora advertencias explícitas de alcance para evitar ejecución en el entorno incorrecto.

### Corregido

- Evitado el hardcode de fuente local única: ahora la app puede continuar operando cuando `LOCAL_POSTGRES_URL` falla pero existe una fuente saludable alternativa.
- Sidebar del frontend muestra la versión real del `package.json` en lugar de una versión fija.

## [1.3.0] - 2026-05-07

### Añadido

- Archivo `VERSION` como referencia única de versión de producto.
- Este `CHANGELOG.md` para publicar cambios entre releases.

### Cambiado

- Versión unificada en metadatos de build: Wails (`wails.json`), recurso Windows (`build/windows/info.json`), instalador Inno Setup (`installer/windows/sycronizafhir-setup.iss`) y `frontend/package.json`.
- Pie del monitor (`Sidebar`) muestra la versión leída del `package.json` del frontend.

## [1.2.0] - 2026-05-07

### Corregido

- Conexión Postgres hacia Supabase detrás del pooler (PgBouncer): uso de protocolo de consulta simple y desactivación del caché de sentencias preparadas en `pgx`, evitando errores `SQLSTATE 42P05` (`stmtcache_* already exists`).

## [1.1.0] - 2026-05-07

### Cambiado

- Valores por defecto de conexión local orientados a la base **mascotas** (incl. ejemplos de puerto para Postgres en Docker).
- Sustitución del DSN embebido legado (`bot_user` / `bot_carpsa`) por configuración coherente con el entorno mascotas.

## [1.0.1] - 2026-05-07

### Cambiado

- Incremento de versión de producto e instalador (1.0.0 → 1.0.1) en artefactos Windows.

## [1.0.0] - 2026-05

### Añadido

- Monitor Wails + WebView2 (Control Center).
- Sincronización bidireccional Postgres local ↔ Supabase (outbound genérico, Realtime inbound, cola SQLite).
- Instalador Windows (Inno Setup + scripts PowerShell) y paquete ZIP de release.
