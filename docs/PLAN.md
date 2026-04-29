DOCUMENTO DE ARQUITECTURA: SYNC-BRIDGE (Legacy PostgreSQL <-> Supabase)

## 1. DESCRIPCIÓN DEL PROYECTO

Middleware de sincronización bidireccional desarrollado en Go. Conecta una base de datos PostgreSQL local (sistema legacy PowerBuilder) con una base de datos en la nube (Supabase).
Diseñado para alta velocidad, bajo consumo de recursos, y tolerancia a caídas de red, sin requerir modificaciones en el código fuente de PowerBuilder.

## 2. REQUISITOS DEL SISTEMA

### Entorno Local (LAN)

- **Servidor Middleware:** Máquina física o VM en la misma red que la DB local (Windows/Linux).
- **Lenguaje:** Go (Golang) versión 1.21 o superior.
- **Base de Datos Local:** PostgreSQL (Versión existente).
- **Red:** Conexión a Internet de salida. (Para escuchar pedidos nuevos sin abrir puertos en el router, usaremos Supabase Realtime / WebSockets).

### Entorno Cloud

- **Supabase:** Proyecto activo, URL y `service_role_key` (para saltar RLS en el middleware).

### Dependencias Go (Librerías principales)

- `github.com/jackc/pgx/v5`: Driver hiper-rápido para PostgreSQL.
- `github.com/joho/godotenv`: Para leer variables de entorno (.env).
- `github.com/mattn/go-sqlite3`: Para la base de datos local de reintentos (Cola de fallos).
- Librería WebSocket/Supabase Realtime para Go (ej. `github.com/supabase-community/supabase-go`).

---

## 3. ARQUITECTURA DE LA BASE DE DATOS LOCAL

Para no impactar el sistema PowerBuilder, se crean objetos aislados en la DB de Postgres local:

### A. Control de Subida (Lectura)

Las tablas `clientes` y `articulos` deben tener:

- Columna: `fecha_modificacion TIMESTAMP DEFAULT CURRENT_TIMESTAMP`
- Índice: `CREATE INDEX idx_fecha_mod ON clientes(fecha_modificacion);`

### B. Tablas Buzón (Escritura)

```sql
CREATE TABLE sync_buzon_pedidos (
    id_buzon SERIAL PRIMARY KEY,
    id_pedido_nube UUID NOT NULL,
    id_cliente INT NOT NULL,
    total DECIMAL(10,2),
    fecha_creacion TIMESTAMP,
    json_detalle JSONB, -- Trae los items del pedido
    procesado BOOLEAN DEFAULT FALSE,
    error_log TEXT
);
C. Trigger de Integración
Un Trigger en Postgres que escuche INSERT en sync_buzon_pedidos. Este trigger lee el json_detalle, verifica stock, inserta en las tablas reales de PowerBuilder (pedidos y pedidos_detalle) y marca procesado = true.

4. ESTRUCTURA DEL PROYECTO EN GO
Plaintext
sync-bridge/
├── .env                        # Credenciales (NO SUBIR A GIT)
├── go.mod                      # Módulos y dependencias
├── go.sum
├── cmd/
│   └── app/
│       └── main.go             # Punto de entrada. Arranca Goroutines.
├── internal/
│   ├── config/
│   │   └── config.go           # Carga de variables de entorno (.env)
│   ├── db/
│   │   ├── local_pg.go         # Conexión y queries a Postgres Local (pgx)
│   │   └── queue_sqlite.go     # Conexión a SQLite para tolerancia a fallos
│   ├── supabase/
│   │   ├── rest.go             # Cliente API REST (para enviar clientes/articulos)
│   │   └── realtime.go         # Cliente WebSocket (para escuchar pedidos)
│   ├── sync/
│   │   ├── outbound.go         # Worker: Local -> Nube (Polling)
│   │   └── inbound.go          # Worker: Nube -> Local (Inserta en Buzón)
│   └── models/
│       ├── articulo.go         # Estructuras de datos
│       ├── cliente.go
│       └── pedido.go
5. FLUJOS DE EJECUCIÓN (GOROUTINES)
La app corre como un servicio en segundo plano e inicia dos procesos paralelos (Goroutines) que no se bloquean entre sí.

PROCESO A: WORKER DE SUBIDA (Outbound - Clientes/Artículos)
Despierta cada X segundos (configurable, ej. 60s).

Consulta Postgres local: SELECT * FROM clientes WHERE fecha_modificacion > ultima_ejecucion.

Si hay datos, formatea a JSON.

Intenta POST/UPSERT a Supabase vía REST API.

Circuit Breaker: Si no hay internet, guarda los IDs en SQLite (queue_sqlite.go).

Si hay éxito, actualiza la variable ultima_ejecucion.

PROCESO B: WORKER DE BAJADA (Inbound - Pedidos)
Inicia conexión persistente por WebSocket (Supabase Realtime) al canal de la tabla pedidos en la nube.

Al recibir un evento INSERT desde Supabase:

Deserializa el JSON del nuevo pedido.

Ejecuta INSERT INTO sync_buzon_pedidos (...) en Postgres local.

Gestión de Errores: Si el motor local está caído (muy raro si están en el mismo server), guarda el payload en SQLite para reintentar la inserción en el buzón más tarde.

6. ESTRATEGIA DE DEPLOYMENT
Ejecutar go build -o sync-bridge.exe ./cmd/app/main.go (En Windows) o go build -o sync-bridge ./cmd/app/main.go (En Linux).

Colocar el ejecutable y el archivo .env en una carpeta segura del servidor de base de datos local.

Registrar como servicio del sistema operativo:

Windows: Usar NSSM (nssm install SyncBridge).

Linux: Crear un archivo de servicio en /etc/systemd/system/syncbridge.service.

Configurar para que inicie automáticamente junto con el SO.
```
