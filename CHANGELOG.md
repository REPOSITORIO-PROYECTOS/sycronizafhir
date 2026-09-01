# Registro de cambios

Formato basado en [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/).
Versiones alineadas con el archivo [`VERSION`](VERSION) en la raíz del repositorio.

## [Unreleased]

## [1.6.15] - 2026-09-01

### Corregido

- **image_sync re-sube fotos pisadas**: el ciclo automático ya no omite un jpg de `Sys_Image\Fotos\Productos` solo porque hay URL en cache. Compara fecha y tamaño del archivo; si Mica mejoró la foto, vuelve a Storage.

## [1.6.14] - 2026-09-01

### Añadido

- **Perfil SYSTEM `pedido_pagina`**: kit `sync-tables.json` habilita la tabla. El outbound genérico **no** upserta cabeza ni detalle. Worker `pedido_pagina_estado_outbound` hace PATCH **solo `estado`** ERP→Supabase (no INSERT, no pisa email/líneas). `cloud_owned_fields.clientes` y `cloud_authoritative_fields.pedido_pagina` en el kit para que no se pisen.

### Corregido

- **Auto-update no baja de versión**: `actualizar-sycronizafhir.ps1` compara semver (con o sin `v`), no instala si `latest` < instalada, y rechaza cualquier latest por debajo del floor `1.6.12` (evita reactivar INSERT en `pedidos` ERP).

## [1.6.13] - 2026-08-25

### Corregido

- **`prod_orden` autoritativo en la nube**: el outbound ya no pisa el orden de vitrina/tienda escrito en Supabase con el valor viejo del ERP SQL. Campo `cloud_authoritative_fields` (default `productos: [prod_orden]`).

## [1.6.12] - 2026-08-25

### Eliminado

- **Worker `pedidos_tienda_inbound`**: ya no convierte `pedido_pagina` → INSERT en `pedidos`/`pedidos_d` de Mica. Las ventas nuevas viven solo en `pedido_pagina` (+ detail). También se borraron los helpers `UpsertPedidoCabeceraTienda` / `ReplacePedidosDetalleTienda`.

### Corregido

- **Inbound `pedido_pagina`**: no hidrata cabecera sin líneas de detalle (evita `0000-*` vacíos en Gestiona).

## [1.6.11] - 2026-08-21

### Corregido

- **Trazabilidad de productos en outbound**: `outbound.json` ahora guarda una muestra de hasta 20 `prod_id` enviados en el último ciclo (`productos_updated_ids`) y su cantidad (`productos_updated_count`). Permite ver desde soporte qué productos salieron sin depender del log efímero.

## [1.6.10] - 2026-08-21

### Corregido

- **Observabilidad outbound persistente**: el worker de background escribe `%APPDATA%\sycronizafhir\errores\estado\outbound.json` con fecha del último ciclo, tablas habilitadas, checkpoint de `productos` y detalle de errores. Operación ya no depende solo de `app.log` para saber si productos sigue subiendo.
- **Paquete Windows completo**: el release vuelve a incluir `sync-table.exe` y `compare-counts.exe`, y el instalador los copia a `Program Files\sycronizafhir`. Corrige el faltante detectado en `Servidor Misan`, donde no estaban las herramientas para forzar sync o comparar conteos.

## [1.6.0] - 2026-07-16

### Agregado

- **Subida manual desacoplada de la config permanente**: en el panel Sincronización, "Subir seleccionadas" ahora usa una selección efímera propia y **nunca** escribe `sync-tables.json`. La configuración de tablas de fondo se edita aparte y se aplica con el botón **"Guardar configuración"** (con confirmación si apaga tablas core). Corrige el incidente MICA donde una subida rápida dejaba `enabled_tables: ["clientes"]` y productos dejaba de subir.
- **Tablas core protegidas**: `clientes`, `productos`, `productos_depositos` no se pueden dejar deshabilitadas sin doble confirmación; el backend rechaza `enabled_tables` vacío y loguea cada cambio de configuración (origen GUI).
- **Always-on real**: al cerrar el monitor se **relanza** el sincronizador en segundo plano (`--background`) sin cortes ni depender solo del watchdog externo; se respeta el flujo de actualización (no relanza durante un update).
- **Allowlist genérica de campos "propiedad de la nube"** (`cloud_owned_fields` en `sync-tables.json`): el outbound no pisa esos campos con un valor local deshabilitado. Generaliza la guarda del flag `web` (default `{ "clientes": ["web"] }`).

### Corregido

- **Resiliencia del background**: `runBackground` ya no muere con `log.Fatalf` ante errores transitorios de config/DB/Supabase; reintenta con backoff (5 s → 2 min) y deja rastro en el log de incidentes.
- **Aislamiento de workers**: cada worker corre con `recover()` y se relanza ante un panic, para que una falla puntual no tumbe todo el proceso.
- **clientes mapeo web**: `web` (S/N) es el único flag de alta tienda; `clien_web` queda como URL/contacto legacy y ya no se trata como alias del flag en inbound ni en la guarda outbound.

### Documentación

- Checklist operativo: tiempos de imágenes (auto 5 min / 150 por ciclo vía `IMAGE_SYNC_*`; "Subir ahora" = todo) y aclaración de que el intervalo de auditoría efectivo es `SYNC_AUDIT_INTERVAL_HOURS` del `.env` (no `auto_audit_interval_hours` del JSON).

## [1.5.20] - 2026-07-16

### Corregido

- **clientes mapeo web**: `web` (S/N) es el único flag de alta tienda; `clien_web` queda como URL/contacto legacy y ya no se trata como alias del flag en inbound ni en la guarda outbound.

## [1.5.19] - 2026-07-16

### Corregido

- **clientes / flag web**: outbound y reconcile ya no pisan un `web=S` (o alta equivalente) en Supabase con `N`/vacío local. Evita el destildado intermitente tras stamps masivos de `fecha_modificacion` (caso Riera 1358).

## [1.5.18] - 2026-07-08

### Agregado

- **Inbound pedidos tienda**: `pedido_pagina` (estado N) → `pedidos` + `pedidos_d` en Mica (`PedidosTiendaInboundWorker`).
- CLIs de sonda: `probe-pedidos-tienda`, `probe-product`.
- SQL `006_pedido_pagina_*.sql` para schema/cleanup/e2e en Supabase.

## [1.5.17] - 2026-07-08

### Agregado

- **Inbound clientes**: baja contacto completado en tienda web (Supabase → Mica).
- **Inbound pedidos**: replica estados K/V/E (y bultos) desde Picking/Supabase → Mica.
- **Inbound pedido_pagina**: replica cabecera + detalle Supabase → Mica.
- Meta `fecha_modificacion`/`hora_modificacion` por tabla (`modified_at.go`) para cursor compuesto.
- Intervalos configurables: `INBOUND_*_INTERVAL_SECONDS`.

## [1.5.16] - 2026-07-08

### Corregido

- **cuenta_corriente**: maps JSON con `Valid`+`Microseconds` (float64) y `pgtype.Interval` → `HH:MM:SS` (compatible varchar(12)).

## [1.5.15] - 2026-07-08

### Corregido

- **Upsert `cuenta_corriente`**: normalización de `pgtype.Time`, `pgtype.Interval` y maps JSON con `Microseconds` a strings codificables por pgx (elimina `cannot find encode plan`).

## [1.5.14] - 2026-07-08

### Corregido

- **Outbound incremental**: cursor por `fecha_modificacion` + `hora_modificacion` (ya no se pierden cambios del mismo día ni filas del día anterior si falló el upsert).
- **Checkpoint outbound**: un cursor **por tabla**, avanza solo con el máximo `(fecha+hora)` de filas **upsert OK** (no más `time.Now()` global).
- **Outbound automático**: respeta `enabled_tables` de `sync-tables.json` (p. ej. excluir `cuenta_corriente` hasta arreglar encoding).

### Agregado

- Tests unitarios para cursor compuesto y checkpoints por tabla.

## [1.5.13] - 2026-07-02

### Agregado

- **Stock ERP**: DDL Supabase `sql/004_productos_depositos_supabase.sql` y prep local `sql/001_productos_depositos_local.sql`.
- **CLI** `apply-supabase-sql`: aplica DDL en Supabase usando `.env` del instalador.
- **Sync por defecto**: `productos_depositos` en `DefaultSyncTablesConfig` (junto con `clientes` y `productos`).
- **compare-counts**: incluye conteo `productos_depositos` local vs nube.
- **image_sync automático**: `IMAGE_SYNC_AUTO_BATCH` (default 150) — el worker recorre todo el backlog por offset, no solo productos con `fecha_modificacion` reciente.

### Corregido

- **image_sync automático**: deja de usar `fecha_modificacion` como filtro incremental; imágenes viejas (ej. sin modificar recientemente) se suben solas en ciclos de 5 min. Las ya cacheadas en Storage se omiten.

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
