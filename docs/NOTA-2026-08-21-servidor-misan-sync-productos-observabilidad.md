# NOTA para el equipo `sycronizafhir` — Servidor Misan, sync de productos OK pero observabilidad degradada

- **Fecha:** 2026-08-21
- **Reportado desde:** verificación remota sobre `servidor` / `misan-servidor`
- **Severidad:** Media
- **Tipo:** Incidente operativo / brecha de observabilidad

---

## 1. Resumen ejecutivo

El proceso `sycronizafhir` en el **Servidor Misan** está **corriendo** y la sincronización de `productos` **no aparece caída** al momento de la revisión del **2026-08-21**:

- Base local `mascotas`: **4421** productos.
- Supabase REST (`public.productos`): **4421** productos.
- Conectividad desde el servidor hacia `stisgpofkdohzvrlsgep.supabase.co:443`: **OK**.
- `sync-tables.json` incluye `clientes`, `productos`, `productos_depositos`, `pedidos`, `pedidos_d`.

Sin embargo, hay una anomalía operativa que merece ticket:

1. El proceso actual (`PID 9576`) arrancó el **2026-08-18 11:15:15**.
2. `app.log` e `incidentes.log` visibles en `%APPDATA%\sycronizafhir\errores\` quedaron con último contenido del **2026-08-11**.
3. Los últimos errores registrados fueron:
   - `realtime websocket: bad handshake`
   - `inbound clientes initial cycle failed: ERROR: el valor es demasiado largo para el tipo character varying(50)`

Conclusión: **productos hoy están sincronizados**, pero la instalación quedó con **observabilidad débil/inconclusa**. Si vuelve a degradarse, hoy no hay un rastro confiable y reciente en logs para detectarlo rápido.

---

## 2. Evidencia tomada en la revisión del 2026-08-21

### Host correcto revisado

- Alias SSH: `servidor`
- Hostname remoto: `servidor`
- Usuario remoto: `servidor\servidor misan`

### Proceso y tareas

- `sycronizafhir` corriendo:
  - `PID=9576`
  - `StartTime=2026-08-18 11:15:15`
- Scheduled Tasks:
  - `sycronizafhir-auto-start` → `Running`
  - `sycronizafhir-auto-update` → `Ready`
  - `sycronizafhir-cc-sync-poller` → `Ready`

### Configuración efectiva

- `version.txt`: `v1.6.9`
- `%APPDATA%\sycronizafhir\sync-tables.json`:

```json
{
  "enabled_tables": [
    "clientes",
    "productos",
    "productos_depositos",
    "pedidos",
    "pedidos_d"
  ],
  "table_mappings": {
    "articulos": "productos"
  },
  "auto_audit_interval_hours": 6,
  "auto_sync_on_audit": true
}
```

- `.env` presente con claves modernas:
  - `SUPABASE_URL`
  - `SUPABASE_SERVICE_ROLE_KEY`
  - `LOCAL_POSTGRES_URL`
  - `OUTBOUND_INTERVAL_SECONDS`
  - `IMAGE_SYNC_ENABLED`
  - `SYNC_EXCLUDE_TABLES`

### Chequeo de sync de productos

- Local Postgres (`mascotas`, `public.productos`):
  - `count(*) = 4421`
  - último `hora_modificacion`: `01300895 | 18:10:26.394`
- Supabase REST:
  - `Content-Range: 0-0/4421`
  - HTTP status: `200`

Esto indica que **el catálogo de productos está alineado en cantidad** entre origen local y destino nube al momento de la revisión.

---

## 3. Hallazgo abierto

La brecha no está hoy en el conteo de productos sino en la **capacidad de diagnóstico**:

- `%APPDATA%\sycronizafhir\errores\app.log` visible quedó clavado en **2026-08-11 16:56:36 -03:00**.
- `%APPDATA%\sycronizafhir\errores\incidentes.log` visible quedó clavado en **2026-08-11 19:56:36Z**.
- El proceso fue reiniciado después, el **2026-08-18**, pero no dejó evidencia reciente en esos archivos.

Hipótesis razonables:

1. El proceso está sano pero **solo escribe logs bajo error**, y desde el 18/08 no hubo nuevos eventos relevantes.
2. El proceso sigue funcionando, pero **el path/rotación/flush de logs** quedó desalineado respecto de lo que operación revisa.
3. Algún componente no crítico (por ejemplo Realtime inbound o clientes) sigue degradado pero **sin surface operacional suficiente**.

---

## 4. Impacto operativo

- Si el equipo pregunta "¿sigue andando siempre prendido?", hoy la respuesta es:
  - **Sí parece seguir corriendo**
  - **Sí productos está sincronizado ahora**
  - **No tenemos observabilidad reciente suficientemente confiable**

- Si reaparece un faltante de productos, clientes o inbound, el tiempo de diagnóstico vuelve a crecer porque:
  - no está `compare-counts.exe`,
  - no está `sync-table.exe`,
  - y los logs visibles no muestran actividad nueva posterior al 11/08.

---

## 5. Acciones sugeridas

### P1 — Observabilidad mínima en servidor Misan

1. Confirmar dónde escribe logs `v1.6.9` después del reinicio del 2026-08-18.
2. Agregar un smoke operativo simple documentado:
   - proceso vivo,
   - reachability a Supabase,
   - conteo local vs nube,
   - timestamp de última evidencia.
3. Exponer una marca de "último ciclo exitoso" persistente y fácil de leer por operación.

### P1 — Tooling de soporte

1. Reponer o documentar ausencia de `compare-counts.exe`.
2. Reponer o documentar ausencia de `sync-table.exe`.
3. Alinear la guía operativa con lo que realmente instala `v1.6.9`.

### P2 — Follow-up funcional

1. Revisar si el problema histórico de `bad handshake` quedó resuelto o simplemente dejó de loguearse.
2. Revisar si el error de `clientes` por `varchar(50)` sigue existiendo en algún ciclo no visible.

---

## 6. Estado final del incidente

- **Sync de productos:** sin evidencia de falla actual al **2026-08-21**
- **Always-on del proceso:** aparente OK
- **Observabilidad / diagnósticos:** **pendiente**
- **Ticket recomendado:** **mantener abierto** hasta recuperar trazabilidad reciente del daemon
