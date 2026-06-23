# Checklist operativo — sincronización sycronizafhir

Documento de consulta rápida: qué está listo en el código, qué falta en la máquina de producción (ej. Mica) y dónde hacer cada paso.

**Versión repo:** 1.5.10 · **Última revisión:** 2026-06-23

---

## 1. Estado del código (repo) — LISTO

| Componente | Estado | Dónde |
|------------|--------|-------|
| Control incremental por `fecha_modificacion` | ✅ Listo | `internal/db/local_pg.go` |
| Soporte `fecha_modificacion` tipo `date` (`>=` mismo día) | ✅ Listo | `internal/db/local_pg.go` → `isFechaModificacionDate` |
| Auditoría por hash (todas las columnas comunes) | ✅ Listo | `internal/sync/reconcile.go` |
| Hash normalizado (char, numéricos, fechas) | ✅ Listo desde v1.5.4 | `internal/sync/rowhash.go` |
| Upsert en sub-batches de 75 filas | ✅ Listo | `internal/supabase/pg_upsert.go` |
| Fix clave embebida `sb_publishable_` → `sb_secret_` (RLS imágenes) | ✅ v1.5.10 | `CHANGELOG.md` |
| Omitir `file_missing` sin reencolar | ✅ v1.5.10 | `internal/sync/image_errors.go` |
| `.env` desde carpeta del ejecutable | ✅ v1.5.10 | `internal/config/config.go` |
| SQL bucket Storage `productos` | ✅ Listo | `sql/003_supabase_storage_productos.sql` |
| SQL tablas con `fecha_modificacion` | ✅ Listo | `sql/000_supabase_prep_completo.sql` |

---

## 2. Estado en máquina de producción — PENDIENTE

Basado en informe v1.5.9 en `C:\Program Files\sycronizafhir` (Mica).

| Área | Síntoma | Causa probable | Prioridad |
|------|---------|----------------|-----------|
| Imágenes | 4276× `rls_auth`, cola ~442k | `SUPABASE_SERVICE_ROLE_KEY` inválida/anon o v1.5.9 sin fix | **P0** |
| Sync tablas | `context deadline exceeded` | Timeout 10 min UI + volumen (pedidos_d ~932k) | **P1** |
| Auditoría | 1440/1441 clientes “cambiados” | Drift real o `fecha_modificacion` distinta local vs nube | **P2** |
| Red | 1× DNS puntual | No bloqueante | P4 |

---

## 3. Qué usar para control de sync (decisión tomada)

| Columna | Rol | Usar ya |
|---------|-----|---------|
| `fecha_modificacion` | Cursor incremental (outbound) + elegibilidad de tabla | **Sí** |
| `fecha_modif` + `hora_modif` | Datos de negocio legacy (productos) | Solo via hash en auditoría, **no** como cursor |
| `ped_fecha` + `ped_hora` | Datos de negocio (pedidos) | Igual que arriba |

**Requisito en BD local:** trigger que actualice `fecha_modificacion` en cada `UPDATE` real.

---

## 4. Pasos en orden (máquina Mica)

### P0 — Imágenes (15 min)

1. **Actualizar app a v1.5.10** (o superior) desde el instalador / auto-update.
2. Supabase Dashboard → **Settings → API** → copiar clave **`service_role`** (no anon).
3. Editar `C:\Program Files\sycronizafhir\.env`:
   ```
   SUPABASE_SERVICE_ROLE_KEY=<service_role real>
   ```
4. Reiniciar: ejecutar `detener-sycronizafhir.ps1` y abrir de nuevo.
5. Panel → **Sincronización** → **Subir imágenes ahora** (probar 1 ciclo).
6. Si sigue `rls_auth`: Supabase → SQL Editor → ejecutar `sql/003_supabase_storage_productos.sql`.

### P1 — Sync sin saturar (30–60 min)

1. Editar `%APPDATA%\sycronizafhir\sync-tables.json`:
   ```json
   "auto_sync_on_audit": false
   ```
2. Panel → sync **una tabla a la vez** (timeout 10 min c/u):
   - `rubro` → `subrubro` → `clientes` → `productos` → `pedidos` → `pedidos_d`
3. Para `pedidos_d`: solo ~13.271 faltantes; no full dump.

### P2 — Investigar “casi todo cambiado” (antes de subir masivo)

1. Elegir 1 PK de `clientes` marcado como cambiado.
2. Comparar local vs Supabase campo a campo.
3. Si solo difiere `fecha_modificacion` → alinear triggers/valores, no re-subir todo.

### P3 — Triggers `fecha_modificacion` (BD local)

En cada tabla sync (`clientes`, `productos`, `pedidos`, `pedidos_d`, `rubro`, `subrubro`):

```sql
-- Ejemplo si la columna es DATE:
CREATE OR REPLACE FUNCTION touch_fecha_modificacion()
RETURNS TRIGGER AS $$
BEGIN
  NEW.fecha_modificacion := CURRENT_DATE;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_productos_fecha_mod ON public.productos;
CREATE TRIGGER trg_productos_fecha_mod
  BEFORE UPDATE ON public.productos
  FOR EACH ROW EXECUTE FUNCTION touch_fecha_modificacion();
```

Repetir por tabla. Ajustar a `CURRENT_TIMESTAMP` si la columna es `timestamp`.

---

## 5. Archivos y rutas de consulta en la máquina

| Qué | Ruta |
|-----|------|
| App instalada | `C:\Program Files\sycronizafhir\` |
| Config `.env` | `C:\Program Files\sycronizafhir\.env` |
| Log principal | `%APPDATA%\sycronizafhir\debug-475a38.log` |
| Tablas habilitadas | `%APPDATA%\sycronizafhir\sync-tables.json` |
| Postgres local | `%APPDATA%\sycronizafhir\local-db.json` |
| Cola SQLite | ver `SQLITE_QUEUE_PATH` en `.env` |
| Fotos productos | `C:\Sys_Image\Fotos\Productos` |
| Guía errores UI | `C:\Program Files\sycronizafhir\ERRORES_MONITOR.md` |

---

## 6. Timeouts a tener en cuenta

| Acción | Límite |
|--------|--------|
| Auditar / Subir seleccionadas (UI) | 10 minutos |
| Subir imágenes ahora | 30 minutos |
| Auditoría programada (background) | Sin límite global |
| Upload Storage por archivo | 60 segundos HTTP |

---

## 7. Dejar para después (no bloquea operación)

- Comparar solo `fecha_modif` + `hora_modif` en vez de hash completo.
- Excluir `fecha_modificacion` del hash de auditoría.
- Normalizar `time` (`hora_modif`) en hash local vs remoto.
- Limpiar cola SQLite de 442k reintentos (solo tras confirmar fix RLS).

---

## 8. Verificación rápida post-fix

```powershell
# Rol de la clave (debe ser service_role)
$key = (Get-Content "C:\Program Files\sycronizafhir\.env" | Select-String "SUPABASE_SERVICE_ROLE_KEY").ToString().Split("=",2)[1]
$payload = $key.Split(".")[1]
[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($payload + ("=" * ((4 - $payload.Length % 4) % 4)))) | ConvertFrom-Json | Select role

# Versión instalada
& "C:\Program Files\sycronizafhir\sycronizafhir.exe" -version
```

**OK si:** `role = service_role`, versión ≥ 1.5.10, 1 ciclo de imágenes con `subidas > 0` y `rls_auth = 0`.

---

## Enlaces repo

- Este checklist: `docs/CHECKLIST_OPERACION_SYNC.md`
- Backlog técnico: `docs/BACKLOG_PROXIMAS_ACTUALIZACIONES.md`
- Errores monitor: `docs/ERRORES_MONITOR.md`
- Instalación Windows: `docs/WINDOWS-INSTALACION.md`
