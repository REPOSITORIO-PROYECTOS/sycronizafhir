# Reporte de estado del proyecto `sycronizafhir`

Fecha: 2026-04-29
Tipo de analisis: revision estatica + reevaluacion de compilacion Wails/Go.

## 1) Estado general del proyecto

El proyecto se encuentra en un estado **funcional y con enfoque operativo**, con componentes claros para:

- Sincronizacion bidireccional entre PostgreSQL local (legacy) y Supabase.
- Monitoreo runtime embebido con UI web local.
- Diagnostico de conectividad y de errores frecuentes.
- Snapshot estructural de base de datos (schema scan) con opcion de envio por email.
- Instalacion/actualizacion para Windows mediante scripts y launcher.

En esta revision no se ejecuto el binario ni pruebas de integracion; por lo tanto, el estado reflejado es de arquitectura y capacidades implementadas.

## 2) Arquitectura observada

Segun `README.md` y estructura interna:

- `cmd/app/main.go`: arranque, wiring de dependencias, monitor HTTP, ciclo de vida y shutdown.
- `cmd/dbscan/main.go`: escaneo completo de schema local (`reports/db-schema-scan-*.json`) y envio de correo opcional.
- `internal/config`: carga de `.env`, defaults y validaciones.
- `internal/db`: Postgres local y cola de fallback SQLite.
- `internal/supabase`: cliente Postgres remoto + cliente Realtime.
- `internal/sync`: workers outbound/inbound con reintentos y manejo de errores.
- `internal/monitor`: runtime monitor con endpoints de estado/scan/comparacion/export.

## 3) Mecanismos actuales para analizar el sistema

### 3.1 Monitor runtime en vivo

Implementado en `internal/monitor/runtime.go` e inicializado desde `cmd/app/main.go`.

Capacidades:

- Vista web local con estado de componentes.
- Endpoint `GET /status` con snapshot JSON de:
  - estado por componente,
  - metadata de conexion,
  - ultimo scan,
  - logs recientes.
- Endpoint `POST /scan` para escaneo on-demand.
- Endpoint `POST /scan/compare` para comparar contra el scan anterior.
- Endpoint `GET /scan/export` para descargar reporte JSON del ultimo escaneo.

Se observa ademas una heuristica de diagnostico para `bad handshake` de Realtime (mensaje orientado a causa probable en credenciales/canal/schema/table).

### 3.2 Escaneo de schema (`dbscan`)

Implementado en `cmd/dbscan/main.go`.

Capacidades:

- Lee tablas de `public`.
- Extrae columnas, PK, FK e indices.
- Genera snapshot JSON versionable en `reports/`.
- Resume metricas basicas de estructura (tablas/columnas/FK/indices).
- Envia el reporte por email como adjunto cuando SMTP esta configurado.
- En caso de fallo temprano (config, conexion, ping, lectura) intenta enviar email de error.

### 3.3 Diagnostico operativo para PostgreSQL local

Script `docs/diagnostico-postgres.ps1`:

- Carga `.env`.
- Parsea `LOCAL_POSTGRES_URL`.
- Verifica conectividad TCP al host/puerto.
- Intenta autenticacion real con `psql`.
- Devuelve mensajes accionables para troubleshooting.

### 3.4 Guia de errores conocida

Documento `docs/ERRORES_MONITOR.md`:

- `realtime websocket: bad handshake`
- `password authentication failed (supabase_postgres)`
- `Tenant or user not found`
- colision de puertos del monitor (fallback automatico)

Este documento reduce tiempo de diagnostico de primer nivel.

### 3.5 Resiliencia de sincronizacion

Por inspeccion de `internal/sync`:

- Workers `outbound` e `inbound` exponen estado al monitor.
- Existe cola SQLite para fallback/reintento de eventos fallidos.
- Se realiza retry periodico de pendientes.
- Outbound usa descubrimiento de tablas sincronizables (no hardcodeado a tablas fijas).

## 4) Indicadores de madurez tecnica

Fortalezas:

- Separacion por capas y responsabilidades.
- Instrumentacion operativa embebida (monitor + scan + export).
- Mecanismo de resiliencia local (queue SQLite).
- Diagnostico explicito y documentado para incidentes comunes.
- Scripts de instalador/actualizacion para entorno Windows.

Brechas detectadas:

- No se observaron tests automatizados (`*_test.go`) en el repositorio.
- No se evidencio pipeline CI/CD en los archivos revisados.
- El analisis de salud actual depende de scans puntuales; no hay evidencia en esta revision de series historicas o alerting externo.

## 5) Mecanismos recomendados para ampliar el analisis

Sin cambiar arquitectura base, los siguientes pasos mejorarian observabilidad y confiabilidad:

1. Agregar tests unitarios minimos en:
   - carga de config,
   - comparacion de scans,
   - parseo/normalizacion de datos de sync.
2. Incorporar smoke test automatizado de arranque (`cmd/app`) en CI.
3. Persistir historico de scans (no solo ultimo en memoria) para trend analysis.
4. Definir umbrales/alertas sobre errores recurrentes de inbound/outbound.
5. Estandarizar un checklist de release usando `dbscan` + `diagnostico-postgres.ps1` + `scan/export`.

## 5.1) Recomendaciones de desarrollo (incorporadas)

1. **Single-instance con cierre limpio (prioridad alta)**
   - Reemplazar estrategia de kill agresivo para traspaso entre instancia background y UI.
   - Implementar señal local (Named Pipe o `.lock` con comando) para solicitar `graceful shutdown`.
   - Asegurar orden de cierre: detener workers, vaciar/confirmar cola, cerrar SQLite y luego relanzar.
   - Motivo: reducir riesgo de corrupcion de la cola SQLite en cortes forzados.

2. **Evitar UPX en Windows para releases normales (prioridad alta)**
   - No usar `wails build -upx` salvo excepcion justificada y testeada.
   - Mantener binarios sin compresion UPX para disminuir falsos positivos en Defender/SmartScreen.
   - Motivo: la reduccion de tamano no compensa el impacto de reputacion y detecciones.

3. **`bridge.ts` con fallback de mocks para acelerar UI (prioridad media-alta)**
   - Si no existe runtime Wails (`!window.go`), devolver respuestas mock JSON.
   - Permitir iteracion de UI con `pnpm dev` en navegador sin recompilar backend Go.
   - Definir mocks minimos de estados operativos y de error para validar UX temprana.

4. **Secuencia de ejecucion sugerida (prioridad media)**
   - Separar primero logica de Go y contratos del bridge.
   - Validar puente con una accion minima extremo a extremo.
   - Construir UI completa sobre mocks y luego conectar a runtime real.
   - Ajustar instalador al final, con validacion en VM limpia.

## 5.2) Puntos concretos a corregir

- El traspaso de control background/UI no debe depender de `taskkill /F` sobre proceso activo con SQLite abierta.
- La documentacion de build debe explicitar que UPX queda deshabilitado por defecto en Windows.
- El frontend debe contar con modo mock oficial en `bridge.ts` para desarrollo local desacoplado.

## 6) Reevaluacion de compilacion (actualizada)

Estado actual verificado en esta corrida:

- `frontend`: `npm run build` OK (TypeScript + Vite).
- `backend`: `go build ./...` OK.
- `desktop`: `wails build -platform windows/amd64` OK.
- `release`: `scripts/build-release.ps1` OK.
- `setup`: Inno Setup OK, artefacto generado en `dist/agencia-ta-soluciones-setup.exe`.

Correcciones aplicadas para estabilizar build:

- Se agrego tipado de Node para `vite.config.ts` (`@types/node` + `types: ["node"]`).
- Se corrigio un error de inferencia TypeScript en `DashboardView.tsx`.
- Se ajusto `tsconfig.json` para evitar chequeo de JS generado en `wailsjs/`.
- Se actualizo pipeline de release para compilar con `wails build` e incluir bootstrapper de WebView2.

## 7) Conclusion

`sycronizafhir` muestra una base solida para operacion real: sincronizacion, fallback, monitoreo y diagnostico estan presentes. El principal siguiente salto de calidad es incorporar verificacion automatizada (tests/CI) y trazabilidad historica de salud para detectar regresiones antes de incidentes.

## 8) Backlog y datos (actualizado 2026-04-30)

Para requisitos de esquema (`fecha_modificacion`, PK), alineacion con Supabase, seleccion futura de bases/tablas y checklist operativo Windows, ver `docs/BACKLOG_PROXIMAS_ACTUALIZACIONES.md`.
