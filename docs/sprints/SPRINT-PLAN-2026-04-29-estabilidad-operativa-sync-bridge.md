---
tipo: sprint-plan
origen: analisis-codigo
fecha: 2026-04-29
estado: borrador
---

# Plan de Sprint: Estabilidad operativa de sync-bridge

## 1. Resumen ejecutivo

- **Problema en una frase**: la base inicial del bridge sincroniza, pero no garantiza continuidad robusta tras reinicios ni control de duplicados end-to-end.
- **Impacto para usuario/negocio**: riesgo de reprocesamientos, pedidos duplicados o ventanas de datos re-enviadas en caídas/restarts.
- **Severidad / prioridad sugerida**: alta.

## 2. Contexto

- **Área / servicio / módulos afectados**: workers `outbound`/`inbound`, persistencia SQLite local, script SQL de integración local.
- **Entorno** (prod, staging, local): local/VM on-prem + Supabase cloud.
- **Enlaces** (PR, ticket, logs, trazas): N/A (sin ticket formal al momento).

## 3. Análisis técnico

### 3.1 Síntomas observados

- El `outbound` dependía de checkpoint en memoria (`lastRun`) y perdía referencia tras reinicio.
- El `inbound` podía insertar pedidos repetidos si llegaban eventos duplicados.
- El buzón local no tenía índice único por `id_pedido_nube`.

### 3.2 Hipótesis de causa raíz

| Hipótesis | Evidencia en código / datos | Estado |
|-----------|-----------------------------|--------|
| Falta de estado persistente de sincronización provoca relecturas | `outbound.go` usaba `lastRun` no persistido | Confirmada |
| Falta de restricción única permite duplicados inbound | SQL base sin unique index; insert sin `ON CONFLICT` | Confirmada |
| Ausencia de lazo de reintentos periódicos en inbound deja payloads en cola | reintento sólo al inicio del worker | Confirmada |

### 3.3 Causa raíz confirmada (si aplica)

Carencia de invariantes operativos mínimas (checkpoint durable, idempotencia DB y retry continuo) en la primera versión.

### 3.4 Alcance y límites

- **Qué está dentro del alcance**: checkpoint persistente, idempotencia inbound, reintento periódico de cola inbound, trigger SQL seguro de fase inicial.
- **Qué queda explícitamente fuera**: mapeo final a tablas reales PowerBuilder (depende del esquema productivo exacto).

## 4. Objetivo del Sprint

- **Resultado esperado al cerrar el sprint**: bridge resistente a reinicios y duplicados en el flujo base nube -> buzón local y local -> nube.
- **Criterios de aceptación (checklist)**:
  - [ ] El worker outbound retoma desde checkpoint persistido tras reinicio.
  - [ ] Un pedido con `id_pedido_nube` repetido no duplica registros en `sync_buzon_pedidos`.
  - [ ] La cola inbound se drena periódicamente aunque la conexión realtime esté estable.
  - [ ] Script SQL provee índice único y trigger de procesamiento seguro.

## 5. Plan de trabajo (backlog del sprint)

Orden sugerido: investigación -> implementación -> pruebas -> despliegue/observabilidad.

| ID | Tarea | Tipo (fix/feature/tech-debt/docs) | Estimación | Responsable (si se conoce) | Dependencias |
|----|-------|-----------------------------------|------------|----------------------------|--------------|
| T1 | Persistir estado de sincronización outbound en SQLite (`sync_state`) | fix | M | Equipo backend | Base SQLite |
| T2 | Aplicar idempotencia de pedidos inbound (`ON CONFLICT` + índice único SQL) | fix | M | Equipo backend | T1 |
| T3 | Agregar retry loop periódico para cola inbound | fix | S | Equipo backend | T2 |
| T4 | Documentar sprint, decisiones y guía de ejecución | docs | S | Equipo backend | T1-T3 |

## 6. Riesgos y mitigaciones

| Riesgo | Probabilidad (B/M/A) | Impacto (B/M/A) | Mitigación |
|--------|----------------------|-----------------|------------|
| Protocolo realtime difiere por versión de Supabase | M | A | Validación en staging y logs de mensajes recibidos |
| Trigger SQL fase inicial no cubre negocio legacy final | A | M | Mantener trigger seguro y planificar fase 3 con mapeo real |
| Carga alta en cola local en caída prolongada | M | M | Limitar lotes por ciclo y monitorear tamaño de cola |

## 7. Pruebas y validación

- **Casos de prueba manuales**: reiniciar proceso y verificar continuidad outbound; enviar eventos duplicados y validar no duplicación en buzón; simular caída local y observar reintentos.
- **Pruebas automáticas a añadir o actualizar**: tests unitarios de parse/persist de checkpoint y cola SQLite (pendiente siguiente iteración).
- **Feature flags / rollback** (si aplica): rollback por reemplazo binario + backup de SQLite.

## 8. Despliegue y seguimiento

- **Estrategia de release**: despliegue incremental en entorno de staging LAN antes de prod.
- **Métricas / dashboards a vigilar**: tamaño de `failed_sync_queue`, tasa de errores en upsert/insert, reconexiones realtime.
- **Criterio de “hecho” operativo** (post-release): 24h sin crecimiento sostenido de cola ni duplicados en buzón.

## 9. Notas y decisiones

- **Decisiones tomadas durante el análisis**: idempotencia se resuelve en DB (source of truth) y no en heurística de aplicación.
- **Preguntas abiertas**: estructura final de tablas PowerBuilder para completar trigger de integración definitiva.
