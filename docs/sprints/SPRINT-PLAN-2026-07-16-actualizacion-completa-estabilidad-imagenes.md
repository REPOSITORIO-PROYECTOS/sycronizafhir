---
tipo: sprint-plan
origen: analisis-codigo
fecha: 2026-07-16
estado: borrador
version_objetivo: 1.6.0
version_actual: 1.5.20
---

# Plan de Sprint: Actualización completa `sycronizafhir` (estabilidad + imágenes)

> Qué entra en la próxima actualización que hay que **subir a Mica**: dejar el
> sincronizador corriendo **siempre en segundo plano**, que **no se caiga si
> tocan la pantalla**, que **no cambie lo que no debe**, y aclarar la **demora
> de imágenes**. Consolidado desde el relevamiento del 16/07.

## 1. Resumen ejecutivo

- **Problema en una frase**: `sycronizafhir` es una app GUI de escritorio, no un servicio: se apaga al cerrar el monitor, no tiene watchdog de salud y el outbound puede pisar campos cuya fuente de verdad es la nube.
- **Impacto para usuario/negocio**: ventanas sin sincronización (stock/clientes/pedidos desactualizados en tienda y picking), destildados intermitentes (caso Riera) e imágenes que parecen no actualizarse.
- **Severidad / prioridad sugerida**: **alta**.

## 2. Contexto

- **Área / servicio / módulos afectados**: `main.go` (ciclo de vida), `singleton_windows.go` (single-instance), `internal/sync/outbound.go` (drift), `internal/sync/images.go` (tiempos), instalador Windows + tareas programadas.
- **Entorno**: producción — máquina **Mica** (Tailscale `100.107.93.43`), `C:\Program Files\sycronizafhir\`. Versión en campo **1.5.20**.
- **Enlaces**: [[Operaciones/Misan/sycronizafhir-estabilidad-relevamiento-2026-07-16]], [[Operaciones/Misan/caso-riera-1358-web-destild-2026-07-16]], [[Operaciones/Misan/REVISION-fotos-storage-2026-07-09]]. `docs/REPORTE_ESTADO_PROYECTO.md` §5.1/5.2.

## 3. Análisis técnico

### 3.1 Síntomas observados

- "Se cae si tocan la pantalla" / "no queda en segundo plano".
- "Se apaga" (se vieron 3 procesos cuando debería haber 1).
- "No tiene un watch".
- "Cambia cosas que no deberían" (destildado `web`).
- "Hay imágenes que se cuelan / tardan".

### 3.2 Hipótesis de causa raíz

| Hipótesis | Evidencia en código / datos | Estado |
|-----------|-----------------------------|--------|
| Abrir el monitor mata el background sano | `singleton_windows.go` → `ensureBackgroundReleased()` hace `taskkill /F /IM sycronizafhir.exe` | **Confirmada** |
| Cerrar la ventana apaga los workers | `main.go` `runWithWindow`: al retornar `wails.Run` → `workerCancel()` | **Confirmada** |
| Crash-loop en background por errores transitorios | `runBackground` usa `log.Fatalf` en config/DB/boot | **Confirmada** |
| No hay watchdog de salud del sync | Solo `RestartCount` en `sycronizafhir-auto-start`; no hay tarea de watchdog | **Confirmada** |
| Outbound pisa campos propiedad de la nube | Upsert genérico de toda la fila ante stamp masivo `fecha_modificacion`; guarda solo para `web` (1.5.19/1.5.20) | **Confirmada (residual)** |
| `taskkill /F` sobre proceso con SQLite abierta puede corromper la cola | `REPORTE_ESTADO_PROYECTO.md` §5.1 lo marca prioridad alta | Confirmada (documental) |
| Imágenes que "no aparecen" = dato, no demora | `images.go` saltea `file_missing`/ruta inválida; 167 Storage≠disco | **Confirmada** |

### 3.3 Causa raíz confirmada

El sincronizador **está acoplado al ciclo de vida de la ventana GUI** y **no tiene supervisión de proceso**. Cualquier interacción humana con el monitor (abrir/cerrar) o un error transitorio deja el sync detenido. Sumado: el outbound genérico no distingue campos "propiedad de la nube".

### 3.4 Alcance y límites

- **Dentro del alcance**: watchdog de salud; que la UI no mate ni ate el background; cierre limpio de SQLite; backoff en background; guarda anti-drift generalizada; documentar/ajustar tiempos de imágenes.
- **Fuera del alcance**: reconciliación de datos de las **167 fotos Storage≠disco** (tarea de datos aparte); multi-base / multi-tabla (backlog previo); migraciones de schema en Supabase.

## 4. Objetivo del Sprint

- **Resultado esperado**: tras subir la actualización, el sync corre **siempre en segundo plano** de forma robusta; abrir/cerrar el monitor **no** lo detiene; no destilda ni pisa campos de la nube; y queda claro el tiempo de imágenes.
- **Criterios de aceptación (checklist)**:
  - [ ] Cerrar el monitor **no** detiene la sincronización (el background sigue o se relanza en segundos).
  - [ ] Abrir el monitor **no** deja el sistema sin background al cerrarlo.
  - [ ] Siempre **exactamente 1** proceso `--background` cuando no hay UI abierta.
  - [ ] Un corte transitorio de PG local/Supabase **no** mata el proceso (reintenta con backoff).
  - [ ] Un stamp masivo de `fecha_modificacion` **no** pisa campos propiedad de la nube (allowlist).
  - [ ] Documentado y validado: imagen nueva/cambiada visible en ≤ intervalo; "Subir ahora" procesa todo.

## 5. Plan de trabajo (backlog del sprint)

Orden: watch (ops, ya) → decouple (Go) → resiliencia → drift → imágenes/datos.

| ID | Tarea | Tipo | Estimación | Dependencias |
|----|-------|------|------------|--------------|
| S1 | **Watchdog de salud** (tarea SYSTEM 1 min): 1 sola instancia `--background` viva salvo UI abierta; dedup duplicados. *Ya creado:* `picking_app/scripts/mica-sycronizafhir-watchdog.ps1` + `mica-setup-sycronizafhir-watchdog.ps1`. | fix ops | S | — (deploy a Mica) |
| S2 | **UI no mata el background**: en `runWithWindow` no llamar `ensureBackgroundReleased()` (taskkill); el monitor coexiste o se conecta al background. | fix Go | M | — |
| S3 | **Decouple workers de la ventana**: al cerrar el monitor, relanzar `--background` (o no bajar los workers). | fix Go | M | S2 |
| S4 | **Cierre limpio de SQLite**: reemplazar `taskkill /F` por shutdown graceful (Named Pipe / `.lock`) para no corromper la cola. | fix Go | M | S2 |
| S5 | **Backoff en background**: reemplazar `log.Fatalf` de `runBackground` por reintento con backoff en errores transitorios de DB/Supabase/config. | fix Go | S | — |
| S6 | **Guarda anti-drift generalizada**: allowlist de campos "propiedad de la nube" que el outbound **no** debe pisar (hoy solo `web`). | fix Go | M | — |
| S7 | **Imágenes — tiempos**: dejar documentado (README/CHECKLIST) que auto = 5 min / 150 por ciclo (`IMAGE_SYNC_*`) y force = todo; opcional subir `IMAGE_SYNC_AUTO_BATCH` para backlogs. | docs/config | S | — |
| S8 | **Imágenes — datos (fuera de release Go)**: script que clasifique productos en `subida-ok` / `falta-en-disco` / `nombre≠prod_id` para atacar los 167. | ops/datos | M | — |

## 6. Riesgos y mitigaciones

| Riesgo | Prob. | Impacto | Mitigación |
|--------|-------|---------|------------|
| No se puede validar comportamiento GUI Wails desde el notebook | A | M | Probar en Mica/VM limpia antes de marcar hecho; watchdog (S1) como red de seguridad |
| Redeploy del `.exe` en prod rompe el sync | M | A | Backup del `.exe` y de `%APPDATA%\sycronizafhir\*` antes; rollback por reemplazo binario |
| `taskkill /F` corrompe la cola SQLite durante el corte | M | M | S4 (graceful) + backup de `sync_queue.db` |
| Allowlist anti-drift deja fuera un campo que sí debía subir | M | M | Lista explícita revisada con Javier/negocio; log de campos omitidos |
| Deploy con Mica offline | M | B | Requiere Mica online (Tailscale); watchdog S1 se instala en el mismo pase |

## 7. Pruebas y validación

- **Manuales (en Mica/VM)**:
  1. `go build ./...` + `go test ./...` OK antes de empaquetar.
  2. Abrir monitor → cerrar → verificar que el background sigue vivo y sincroniza (log + `watchdog.log`).
  3. Matar el proceso a mano → verificar que reaparece 1 sola instancia en ≤1 min.
  4. Cortar PG local unos segundos → verificar reintento (no muere el proceso).
  5. Forzar stamp masivo de `fecha_modificacion` en un cliente con `web=S` en nube → verificar que **no** se destilda.
  6. Imagen nueva → visible en ≤5 min; "Subir ahora" → sube todo.
- **Automáticas a añadir**: test de la allowlist anti-drift (S6); test de decisión single-instance (S2/S3) si es aislable.
- **Rollback**: reemplazo del `.exe` anterior + restore de `sync_queue.db`.

## 8. Despliegue y seguimiento

- **Estrategia de release**: bump **1.6.0**; sincronizar `VERSION`, `wails.json`, `build/windows/info.json`, `installer/*.iss`, `frontend/package.json`; `wails build -platform windows/amd64` (sin UPX); empaquetar ZIP; subir con `picking_app/scripts/mica-deploy-sycronizafhir.ps1 -TargetVersion v1.6.0`.
- **Watchdog (S1)**: en el mismo pase, `mica-setup-sycronizafhir-watchdog.ps1` en Mica.
- **Métricas a vigilar (24 h)**: tamaño de `failed_sync_queue`, cantidad de procesos `sycronizafhir` (=1), reconexiones realtime, `watchdog.log`, sin destildados.
- **Criterio de hecho operativo**: 24 h con 1 proceso estable, sin drift y con imágenes al día.

## 9. Notas y decisiones

- **Decisiones**: el "watch" (S1) va como script externo, sin recompilar, para tener red de seguridad ya. Los fixes de fondo (S2–S6) van en el binario 1.6.0.
- **Preguntas abiertas**:
  - ¿La UI del monitor debe seguir existiendo como ventana o pasar a monitor read-only conectado al background? (define S2/S3).
  - ¿Lista definitiva de campos "propiedad de la nube" para la allowlist (S6)? (confirmar con Javier/negocio).
  - ¿Subimos `IMAGE_SYNC_AUTO_BATCH` por defecto o solo ante backlog?
