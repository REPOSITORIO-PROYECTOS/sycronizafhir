# NOTA para el equipo `sycronizafhir` — Solapamiento entre "subida manual" y "config permanente"

- **Fecha:** 2026-07-16
- **Reportado desde:** operación MICA (misan / picking_app)
- **Severidad:** Alta (productos dejaron de subir a Supabase → no aparecen en la tienda)
- **Tipo:** Bug de diseño (UX/estado compartido) + brecha de supervisión (always-on)

---

## 1. Resumen ejecutivo

En operación real, **los productos nuevos dejaron de subir a Supabase** y el agente apareció **apagado**. Investigando el código y la máquina de producción (MISAN) se identificaron **tres causas independientes que se combinan**:

1. **(Causa raíz principal) La selección de tablas para una "subida rápida" y la lista permanente `enabled_tables` del daemon son el MISMO estado.** Al destildar una tabla en la UI para preparar una subida manual, se **persiste al instante** `sync-tables.json` con la lista recortada, apagando esa tabla en el background **para siempre**. Así quedó `enabled_tables: ["clientes"]` y productos dejó de sincronizarse.
2. **No hay watchdog/servicio interno.** El "siempre prendido" depende de una Scheduled Task externa que solo reinicia si el **proceso muere**, no si queda **vivo pero degradado**.
3. **(Secundaria) El canal Realtime inbound queda en estado degradado permanente** ante `bad handshake` (service role key), reintentando cada ~60s sin conectar, sin reflejarse como "caído".

El pedido concreto del negocio: **la subida manual de una tabla debe ser una acción efímera y aparte; las tablas que quedan habilitadas en background deben permanecer habilitadas siempre.** Hoy se solapan.

---

## 2. Evidencia de campo (MISAN, 2026-07-16)

- `compare-counts.exe`: `productos 4421/4421 (100%)`, `productos_depositos 4346/4346 (100%)` → a nivel conteo no faltaban, pero **el outbound ya no los sube** porque la tabla está deshabilitada.
- `%APPDATA%\sycronizafhir\sync-tables.json`:
  ```json
  { "enabled_tables": ["clientes"], "table_mappings": { "articulos": "productos" },
    "auto_audit_interval_hours": 12, "auto_sync_on_audit": false }
  ```
  (debería incluir `productos` y `productos_depositos`).
- Proceso `sycronizafhir`: **NOT RUNNING** (murió ~mediodía; la PC se había reiniciado por Windows Update el 15/07).
- `errores\incidentes.log`: loop cada 30s → `error | inbound | handshake realtime rechazado (revisar SUPABASE_SERVICE_ROLE_KEY y canal)`.

---

## 3. Causa raíz #1 — La UI guarda `enabled_tables` al tildar/destildar (solapamiento)

En `frontend/src/views/SyncView.tsx`, `toggleTable` **persiste la config permanente en cada click**:

```tsx
const toggleTable = (name: string) => {
  setSelectedTables((prev) => {
    const next = prev.includes(name)
      ? prev.filter((item) => item !== name)
      : [...prev, name];
    saveConfigMutation.mutate(next);   // <-- escribe enabled_tables en disco AL INSTANTE
    return next;
  });
};
```

Y el mismo `selectedTables` alimenta la subida manual:

```tsx
// mismo estado usado para la acción efímera
const syncMutation = useMutation({ mutationFn: () => bridge.syncSelectedTables(selectedTables) });
...
<Button onClick={() => syncMutation.mutate()}>Subir seleccionadas</Button>
```

**Consecuencia:** un operador que quiere "subir solo clientes rápido" destilda `productos`/`productos_depositos` → `saveConfigMutation` escribe `enabled_tables: ["clientes"]`. El daemon **relee `sync-tables.json` en caliente cada ciclo (~60s)** y **deja de subir** las tablas removidas, sin reinicio ni aviso.

Confirmación en el backend (Go):
- Única función que escribe el archivo: `config.SaveSyncTablesConfig` (`internal/config/sync_tables.go`), invocada solo por `App.SaveSyncTablesConfig` (`app.go`).
- El sync manual **NO** toca la config: `cmd/sync-table/main.go` y `App.SyncSelectedTables` solo leen y ejecutan. Es decir, el problema **no está en el CLI**, está en que la **UI acopla el "tildado para subir" con el guardado permanente**.
- El daemon relee en caliente: `OutboundWorker.runCycle` → `config.LoadSyncTablesConfig()` + `syncCfg.IsEnabled(table.Name)` (`internal/sync/outbound.go`).

---

## 4. Causa raíz #2 — "Siempre prendido" sin watchdog interno

- No existe watchdog, servicio de Windows, ni `recover()` en el proyecto (0 coincidencias). La disponibilidad depende 100% de la Scheduled Task externa `sycronizafhir-auto-start` (instalador: `RestartCount 999`, `RestartInterval 1min`, triggers AtStartup/AtLogon).
- **Limitación:** la task reinicia solo si el **proceso termina**. Si el proceso queda **vivo pero degradado** (ej. inbound en `error` permanente), la task **no** lo reinicia.
- En MISAN el proceso murió y **no volvió** hasta intervención manual (la task había corrido por última vez el 15/07 al login; sin nuevo login/boot, no reintentó).

---

## 5. Causa raíz #3 — Inbound `bad handshake` degradado (no transitorio)

- `internal/sync/inbound.go` usa `cfg.SupabaseServiceRole` para el handshake Realtime.
- `bad handshake` **no** está clasificado como transitorio (`internal/supabase/realtime_errors.go`), así que el componente queda en estado `"error"` reintentando cada ~60s **eternamente**, sin ingresar pedidos, pero **sin tumbar el proceso**. Operativamente "parece prendido" pero el inbound no funciona.

---

## 6. Recomendaciones (priorizadas) para el equipo

### P0 — Desacoplar "subida manual" de "config permanente" (arregla el solapamiento)
Elegir una de estas (en orden de preferencia):

1. **Separar los dos estados en la UI:** un `selectedForManualUpload` (efímero, para "Subir seleccionadas") **distinto** de `enabledTables` (permanente). "Subir seleccionadas" **nunca** debe escribir `sync-tables.json`.
2. **Quitar el auto-save de `toggleTable`** y agregar un botón explícito **"Guardar configuración"** (con confirmación). El tildado no persiste hasta guardar.
3. **Concepto de "tablas core protegidas"** (`clientes`, `productos`, `productos_depositos`) que **no puedan** deshabilitarse desde una acción de subida rápida; requerir confirmación explícita para bajarlas.

### P1 — Guardas de seguridad sobre `enabled_tables`
- **Confirmación** al reducir el set (y aviso claro: "esto apaga la sincronización en background de X").
- **No permitir persistir un set que remueva tablas core** sin doble confirmación.
- Considerar loguear/auditar cambios de `enabled_tables` (quién/cuándo) para trazabilidad.

### P2 — Supervisión real "always-on"
- Que un estado **degradado persistente** (inbound en `error` > N minutos) se refleje como **no saludable** y dispare reinicio: o bien `os.Exit(non-zero)` controlado (para que la Scheduled Task lo relance), o un **watchdog interno**/health-check.
- Evaluar `recover()` por worker (relanzar goroutine) para que un panic no tumbe todo el proceso.
- Alternativa robusta: registrar el agente como **servicio de Windows** con recuperación nativa.

### P3 — Higiene de config
- `bad handshake` inbound: mensaje ya es claro; sumar validación temprana de `SUPABASE_SERVICE_ROLE_KEY` al arranque (fail-fast o alerta visible).
- **Ambigüedad `auto_audit_interval_hours` (JSON) vs `SYNC_AUDIT_INTERVAL_HOURS` (env):** el worker programado usa el del `.env`; el JSON solo se muestra/edita en la GUI. Unificar o documentar para evitar confusión operativa.

---

## 7. Mitigación operativa inmediata (lado operación, no requiere release)

1. Restaurar `enabled_tables` a `["clientes","productos","productos_depositos"]` en `sync-tables.json`.
2. Reiniciar el agente (`--background`) y verificar `sycronizafhir` corriendo.
3. Instalar/validar un watchdog externo (task cada 1–2 min que relance si no está corriendo).
4. Revisar/renovar `SUPABASE_SERVICE_ROLE_KEY` para el canal Realtime inbound.

> Nota: mientras la UI siga con el comportamiento de la Causa #1, cualquier "subida rápida" de una sola tabla puede volver a apagar productos. La corrección definitiva es P0.
