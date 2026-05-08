# Registro de cambios

Formato basado en [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/).
Versiones alineadas con el archivo [`VERSION`](VERSION) en la raíz del repositorio.

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
