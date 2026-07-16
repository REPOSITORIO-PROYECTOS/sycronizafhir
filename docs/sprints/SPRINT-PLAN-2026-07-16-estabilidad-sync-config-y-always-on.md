---
tipo: sprint-plan
origen: analisis-codigo
fecha: 2026-07-16
estado: borrador
---

# Plan de Sprint: Estabilidad de sync — config permanente, always-on e inbound

## 1. Resumen ejecutivo

- **Problema en una frase**: Productos nuevos dejaron de subir a Supabase (no aparecen en la tienda) y el agente quedó apagado; la raíz es que una "subida manual" de una tabla **pisa** la configuración permanente de background, sumado a que no hay supervisión real "siempre prendido".
- **Impacto para usuario/negocio**: Catálogo de la tienda desactualizado (faltan/no se reflejan productos), pedidos web que no vuelven al ERP (inbound caído), y horas de operación perdidas en diagnóstico manual. Es un problema recurrente ("esto no debería pasar más").
- **Severidad / prioridad sugerida**: **Alta / P0** para el desacople de config; **P1–P2** para supervisión e inbound.

## 2. Contexto

- **Área / servicio / módulos afectados**:
  - GUI Wails: `frontend/src/views/SyncView.tsx`, `frontend/src/lib/bridge.ts`.
  - Bridge Go: `app.go` (`SaveSyncTablesConfig`, `SyncSelectedTables`).
  - Config: `internal/config/sync_tables.go`, `internal/config/config.go`.
  - Workers: `internal/sync/outbound.go`, `internal/sync/inbound.go`, `internal/sync/audit_worker.go`.
  - Supabase: `internal/supabase/realtime.go`, `internal/supabase/realtime_errors.go`.
  - Supervisión: `main.go`, `installer/windows/instalar-sycronizafhir.ps1`.
- **Entorno**: Producción MICA (PC `MISAN`, Windows) + Supabase (proyecto `stisgpofkdohzvrlsgep`). Consumidor: `picking_app` (backend + tienda_virtual).
- **Enlaces** (evidencia y detalle):
  - Nota de incidente: `docs/NOTA-2026-07-16-solapamiento-config-permanente-vs-subida-manual.md`.
  - Backlog (callout P0): `docs/BACKLOG_PROXIMAS_ACTUALIZACIONES.md`.

## 3. Análisis técnico

### 3.1 Síntomas observados

1. Productos nuevos no se reflejan en Supabase → no aparecen en la tienda.
2. Agente `sycronizafhir` **NOT RUNNING** en MISAN (murió ~mediodía; la PC se reinició por Windows Update el 15/07).
3. `sync-tables.json` con `enabled_tables: ["clientes"]` (faltan `productos`, `productos_depositos`).
4. `compare-counts`: productos 100% (4421/4421) → no había faltantes de conteo, pero el outbound **ya no los sube** porque la tabla está deshabilitada.
5. `incidentes.log`: loop cada 30s `inbound | handshake realtime rechazado (revisar SUPABASE_SERVICE_ROLE_KEY y canal)`.
6. Faltantes históricos aparte: `pedidos` 99.7%, `pedidos_d` 98.2%, `cuenta_corriente` 74.5%.

### 3.2 Hipótesis de causa raíz

| Hipótesis | Evidencia en código / datos | Estado |
|-----------|-----------------------------|--------|
| La subida manual y la config permanente comparten estado; tildar tablas guarda `enabled_tables` | `SyncView.tsx` `toggleTable` llama `saveConfigMutation.mutate(next)` en cada click; `Subir seleccionadas` usa el mismo `selectedTables` | **Confirmada** |
| El daemon aplica el recorte en caliente sin reinicio | `outbound.go` `runCycle` relee `LoadSyncTablesConfig()` + `IsEnabled()` cada ~60s | **Confirmada** |
| El CLI/`sync-table.exe` NO reescribe la config (no es el culpable) | `cmd/sync-table/main.go` y `App.SyncSelectedTables` solo leen | **Confirmada** |
| No hay watchdog/servicio interno; always-on depende de Scheduled Task externa | 0 `recover()`, sin Windows Service; `instalar-sycronizafhir.ps1` crea task `RestartCount 999` | **Confirmada** |
| La task reinicia solo si el proceso muere, no si queda degradado | Task Scheduler restart on failure; inbound queda en estado `error` con proceso vivo | **Confirmada** |
| Inbound `bad handshake` no es transitorio → loop infinito degradado | `realtime_errors.go` no lo clasifica como transitorio; `inbound.go` backoff 5s→60s eterno | **Confirmada** |
| `SUPABASE_SERVICE_ROLE_KEY` inválida/vencida causa el bad handshake | `incidentes.log` repetido; `realtime.go` usa service role en el handshake | **Probable (pendiente verificar key)** |
| Outbound puramente incremental por `fecha_modificacion` puede dejar altas sin subir | `outbound.go` `WHERE fecha_modificacion > $1`; recuperación depende del audit | **Confirmada (comportamiento)** |
| Faltantes de `cuenta_corriente`/`pedidos_d` = tema aparte (histórico/bootstrap) | `compare-counts` 74.5%/98.2% | **Pendiente de verificación** |

### 3.3 Causa raíz confirmada

**Causa raíz principal:** el diseño de la UI **acopla dos responsabilidades distintas en un solo estado** (`selectedTables`): (a) selección efímera para "Subir seleccionadas" y (b) `enabled_tables` permanente del daemon. Al preparar una subida rápida de una tabla, se destildan las demás y `toggleTable` **persiste** el recorte, apagando esas tablas en el background para siempre.

**Causas raíz agravantes:**
- No existe supervisión que garantice "siempre prendido" ante proceso muerto **o** degradado.
- El inbound Realtime entra en estado degradado permanente ante `bad handshake` sin señalizarse como caído ni forzar recuperación.

### 3.4 Alcance y límites

- **Dentro del alcance**: desacople config/subida manual, guardas sobre `enabled_tables`, supervisión always-on, manejo de inbound degradado, higiene de config.
- **Fuera del alcance (este sprint)**: reconciliación de faltantes históricos de `cuenta_corriente`/`pedidos_d` (bootstrap aparte); rediseño multi-DSN (ya en backlog §4).

## 4. Objetivo del Sprint

- **Resultado esperado**: Que una subida manual **nunca** modifique la config permanente, que las tablas core no puedan apagarse por accidente, y que el agente se mantenga saludable y prendido (o se recupere solo) incluyendo el canal inbound.
- **Criterios de aceptación (checklist)**:
  - [ ] "Subir seleccionadas" no escribe `sync-tables.json` bajo ninguna circunstancia.
  - [ ] Cambiar `enabled_tables` requiere acción explícita ("Guardar") con confirmación al reducir el set.
  - [ ] Las tablas core (`clientes`, `productos`, `productos_depositos`) no se pueden dejar deshabilitadas sin confirmación doble.
  - [ ] Un estado degradado persistente (>N min) se refleja como "no saludable" y dispara recuperación/reinicio.
  - [ ] `bad handshake` inbound genera alerta visible y validación temprana de la key al arranque.
  - [ ] Cambios de `enabled_tables` quedan logueados (quién/cuándo/qué).

## 5. Plan de trabajo (backlog del sprint)

Orden sugerido: investigación → implementación → pruebas → despliegue/observabilidad.

| ID | Tarea | Tipo | Estimación | Responsable | Dependencias |
|----|-------|------|------------|-------------|--------------|
| T1 | Separar estado UI: `selectedForUpload` (efímero) vs `enabledTables` (persistente) en `SyncView.tsx` | fix | 0.5 d | — | — |
| T2 | Quitar `saveConfigMutation` de `toggleTable`; agregar botón "Guardar configuración" con diff/confirmación | fix | 0.5 d | — | T1 |
| T3 | Concepto de tablas core protegidas (no deshabilitables sin doble confirmación) en front + validación en `SaveSyncTablesConfig` | feature | 1 d | — | T1 |
| T4 | Validación backend: rechazar/avisar guardado que remueva tablas core o deje `enabled_tables` vacío | fix | 0.5 d | — | — |
| T5 | Auditoría de cambios de `enabled_tables` (log con timestamp y origen GUI) | tech-debt | 0.5 d | — | — |
| T6 | Supervisión: exit(non-zero) controlado o watchdog interno ante estado degradado persistente (inbound `error` > N min) | feature | 1.5 d | — | — |
| T7 | `recover()` por worker + relanzado de goroutine para que un panic no tumbe todo | tech-debt | 1 d | — | — |
| T8 | Inbound: validación temprana de `SUPABASE_SERVICE_ROLE_KEY` al arranque (fail-fast/alerta) + reflejar "inbound caído" en telemetría | fix | 0.5 d | — | — |
| T9 | Unificar/documentar `auto_audit_interval_hours` (JSON) vs `SYNC_AUDIT_INTERVAL_HOURS` (env) | docs | 0.25 d | — | — |
| T10 | Instalador: confirmar recovery robusto (evaluar Windows Service nativo vs Scheduled Task) | tech-debt | 1 d | — | T6 |
| T11 | Pruebas + release + verificación en MISAN | tests | 1 d | — | T1–T10 |

## 6. Riesgos y mitigaciones

| Riesgo | Prob. | Impacto | Mitigación |
|--------|-------|---------|------------|
| Cambiar la UI rompe el flujo actual de operadores | M | M | Mantener "Subir seleccionadas" familiar; separar solo el guardado; textos claros |
| Migrar a Windows Service complica el instalador | M | M | Hacerlo opcional (T10); mantener Scheduled Task mejorada como fallback |
| Forzar exit ante degradado genere reinicios en loop si la key sigue mal | M | A | Backoff + alerta; no reiniciar si la causa es config inválida (fail-fast visible en su lugar) |
| `recover()` oculte bugs reales | B | M | Loguear siempre el panic recuperado; métrica de panics |

## 7. Pruebas y validación

- **Casos de prueba manuales**:
  - Destildar productos y hacer "Subir seleccionadas" → `sync-tables.json` **no** cambia; daemon sigue subiendo productos.
  - Guardar config quitando una core → pide doble confirmación; si se cancela, no persiste.
  - Simular `SUPABASE_SERVICE_ROLE_KEY` inválida → alerta visible + telemetría "inbound caído"; recuperación al corregir.
  - Matar el proceso → se reinicia solo; dejarlo degradado → se detecta y recupera.
- **Pruebas automáticas a añadir/actualizar**: unit de `SaveSyncTablesConfig` (rechazo de set inválido/vacío/core removida); test de `IsEnabled`/hot-reload; test de clasificación de errores realtime (bad handshake = no transitorio + señal de salud).
- **Feature flags / rollback**: release versionada + auto-update; rollback al binario anterior si falla.

## 8. Despliegue y seguimiento

- **Estrategia de release**: build Wails → validar en 1 equipo (MISAN) → auto-update.
- **Métricas / dashboards a vigilar**: estado de componentes (outbound/inbound/audit) en el monitor; `compare-counts` de core tables; `incidentes.log` sin loops de handshake.
- **Criterio de "hecho" operativo**: 48 h sin caídas del agente; productos suben tras alta; inbound conectado; ninguna subida manual altera `enabled_tables`.

## 9. Notas y decisiones

- **Decisiones tomadas durante el análisis**:
  - La causa NO está en el CLI `sync-table.exe` (no toca la config); está en el acople de la UI.
  - El daemon relee la config en caliente (~60s), por eso el recorte impacta sin reinicio.
- **Preguntas abiertas**:
  - ¿La `SUPABASE_SERVICE_ROLE_KEY` está vencida/rotada? (verificar y renovar).
  - ¿Los faltantes de `cuenta_corriente`/`pedidos_d` requieren un bootstrap/reconcile aparte? (fuera de este sprint).
  - ¿Se adopta Windows Service o se robustece la Scheduled Task? (definir en T10).
