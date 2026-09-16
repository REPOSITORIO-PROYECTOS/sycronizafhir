-- =============================================================================
-- productos_x_provincias en Postgres LOCAL (SERVIDOR / mascotas)
-- =============================================================================
-- Ejecutar UNA VEZ en la base mascotas antes de sincronizar.
-- Las filas actuales tienen fecha_modificacion NULL → el outbound incremental
-- no las sube. Este script backfillea stamps + trigger.
-- Después: DDL Supabase 007 + enabled_tables + bootstrap/reconcile.
-- =============================================================================

BEGIN;

-- Columnas ya existen en ERP Gestiona; no recrear.
UPDATE public.productos_x_provincias
SET fecha_modificacion = CURRENT_DATE
WHERE fecha_modificacion IS NULL;

UPDATE public.productos_x_provincias
SET hora_modificacion = CURRENT_TIME
WHERE hora_modificacion IS NULL;

CREATE OR REPLACE FUNCTION public.fn_productos_x_provincias_modificacion()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
  NEW.fecha_modificacion := CURRENT_DATE;
  NEW.hora_modificacion := CURRENT_TIME;
  RETURN NEW;
END;
$function$;

DROP TRIGGER IF EXISTS tr_productos_x_provincias_modificacion ON public.productos_x_provincias;
CREATE TRIGGER tr_productos_x_provincias_modificacion
  BEFORE INSERT OR UPDATE ON public.productos_x_provincias
  FOR EACH ROW
  EXECUTE PROCEDURE public.fn_productos_x_provincias_modificacion();

-- Marcar todo para carga inicial a Supabase (una sola vez).
UPDATE public.productos_x_provincias
SET fecha_modificacion = CURRENT_DATE,
    hora_modificacion = CURRENT_TIME;

COMMIT;
