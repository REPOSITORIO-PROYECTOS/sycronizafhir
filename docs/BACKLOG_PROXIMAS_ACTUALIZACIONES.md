# Backlog y notas para próximas actualizaciones (`sycronizafhir`)

Documento vivo: decisiones de producto, contrato de datos local ↔ Supabase, y pendientes que surgieron en despliegue (Wails, Windows, auto-update). Sirve para planificar sprints sin re-discutir el contexto operativo.

---

## 1) Qué “bases de datos” intervienen hoy

| Rol | Qué es en la práctica | Cómo se configura |
|-----|------------------------|-------------------|
| **PostgreSQL local (legacy)** | **Una** base por instalación: el nombre va en el DSN (`LOCAL_POSTGRES_URL` o override en `%APPDATA%\sycronizafhir\local-db.json`). Ejemplo histórico: base `mascotas` en el servidor local. | UI **Conexiones** + archivo `local-db.json`; variables de entorno en build/instalación. |
| **Supabase (nube)** | Proyecto Supabase: Postgres administrado (típicamente base `postgres`, schema `public` para el upsert) + Realtime (canal/schema/tabla configurables). | `HOST_SUPABASE`, `USUARIO_SUPABASE`, `CONTRASENA_SUPABASE`, `SUPABASE_DB_URL` / DSN, `SUPABASE_REALTIME_*`, `SUPABASE_SERVICE_ROLE_KEY`, etc. |

**Importante:** el worker de salida (`internal/sync/outbound.go`) hace **upsert** hacia Supabase en el schema **`public`** usando el **mismo nombre de tabla** que en el origen (`SYNC_SOURCE_SCHEMA`, por defecto `public`). No hay hoy un mapeo tabla-local → tabla-remota distinto.

---

## 2) Contrato de esquema en PostgreSQL local (por qué “no detecta tablas”)

La detección de tablas sincronizables está en `internal/db/local_pg.go` (`ListSyncTables`). Una tabla **solo entra** si cumple **todo** esto:

1. Está en el schema configurado (`SYNC_SOURCE_SCHEMA`, default `public`).
2. Tiene columna **`fecha_modificacion`** (nombre exacto).
3. Tiene **clave primaria** definida.
4. No está en la lista de exclusión (`SYNC_EXCLUDE_TABLES`) ni es la tabla interna `sync_buzon_pedidos` (excluida en código).

Las filas a enviar se leen con `SELECT * ... WHERE fecha_modificacion > $1`, ordenadas por esa columna.

**Para el administrador de base de datos:** además de crear la columna, conviene:

- Índice en `fecha_modificacion` para escaneos incrementales.
- Trigger(s) `BEFORE/AFTER UPDATE` que mantengan `fecha_modificacion` en cada cambio de negocio (si no existe ya).

Referencia de arranque en el repo: `sql/001_sync_bridge_setup.sql` (incluye `fecha_modificacion` para `clientes` y `articulos`, buzón `sync_buzon_pedidos`, etc.). **Cada tabla de negocio** que deba subir debe alinearse a ese contrato (columna + PK + trigger según política).

---

## 3) Coherencia local ↔ Supabase (evitar “quilombo” en columnas y tipos)

Antes de ampliar tablas o bases, conviene un **inventario coordinado**:

- **Mismos nombres de tabla** en local (`SYNC_SOURCE_SCHEMA`) y en Supabase `public` (comportamiento actual).
- **Mismas columnas relevantes** o una fase explícita de **migración en Supabase** (SQL editor / migraciones versionadas) para agregar columnas nuevas con tipos compatibles con lo que envía el cliente.
- **Tipos y nullability:** el upsert envía los valores tal cual salen del driver local; discrepancias (UUID vs texto, `numeric` vs `float`, campos NOT NULL sin default) generan errores en destino o colas SQLite.
- **Claves y unicidad:** las columnas de `ON CONFLICT` son las **PK** detectadas en local; en Supabase debe existir la misma PK (o unique equivalente acordada).
- **Realtime (pedidos / nube → local):** revisar que `SUPABASE_REALTIME_SCHEMA`, `SUPABASE_REALTIME_TABLE` y el canal coincidan con lo publicado en Supabase; credencial incorrecta suele manifestarse como “bad handshake”.

Recomendación de proceso: acordar con quien administra Supabase un **checklist por tabla** (columnas, PK, índices, RLS si aplica) **antes** de incluirla en la sincronización genérica.

---

## 4) Pendiente de producto: módulo para elegir qué se envía (“multi-selección”)

**Estado hoy:** una sola URL de Postgres local activa; el worker recorre **todas** las tablas que pasan el filtro `fecha_modificacion` + PK.

**Falta (backlog):**

- **Selección de bases de datos:** si en el futuro hay que sincronizar **varias bases PostgreSQL** (varios DSN), hace falta:
  - modelo de configuración (perfiles: nombre, DSN, schema, tablas incluidas/excluidas),
  - UI para alta/edición/prueba de cada perfil,
  - workers o rondas por perfil sin mezclar checkpoints.
- **Selección fina de tablas:** más allá de `SYNC_EXCLUDE_TABLES` por env, conviene persistencia en archivo/UI (allowlist o denylist por base) y reflejo en el monitor.

Interpretación del pedido original (“módulo”): capacidad explícita de **elegir bases y/o tablas** a sincronizar, con persistencia y sin depender solo de variables de entorno.

---

## 5) Notas operativas Windows (no perder en el próximo release)

- **WebView2 Runtime:** requerido para la UI Wails; el instalador puede bootstrappear si falta.
- **Cola SQLite:** usar ruta estable y con permisos (p. ej. `SQLITE_QUEUE_PATH` apuntando a `C:\ProgramData\sycronizafhir\sync_queue.db`); errores “out of memory (14)” a menudo son ruta/permiso, no RAM.
- **Una sola instancia:** mutex global evita dos procesos compitiendo.
- **Auto-update GitHub:** `github-update-config.json` debe tener `enabled: true` y release con tag válido + asset `sycronizafhir-installer-package.zip`; 404 suele ser tag/asset/nombre incorrecto.
- **Config incrustada vs `.env`:** el binario puede llevar defaults embebidos; la conexión local prioriza `local-db.json` cuando existe.
- **Build:** `SUPABASE_SERVICE_ROLE_KEY` no puede ser placeholder en tiempo de build si la validación falla al cargar config.

---

## 6) Riesgos y regresiones a vigilar

- Añadir `fecha_modificacion` sin trigger puede dejar filas “viejas” que nunca se re-envían o comportamiento inconsistente según cómo se rellene la columna.
- Cambiar PK en local o en Supabase sin migración coordinada rompe el upsert.
- Habilitar RLS en Supabase sin políticas para el rol del conector bloquea escrituras silenciosas o con errores genéricos.
- Multi-base / multi-tabla sin checkpoints separados puede duplicar trabajo o mezclar estado.

---

## 7) Referencias de código

- Descubrimiento de tablas y lectura incremental: `internal/db/local_pg.go` (`ListSyncTables`, `LoadUpdatedRows`).
- Envío a Supabase: `internal/sync/outbound.go` (`UpsertRows` en schema `public`).
- SQL de ejemplo: `sql/001_sync_bridge_setup.sql`.
