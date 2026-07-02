-- =============================================================================
-- productos_depositos en Postgres LOCAL (Mica / mascotas)
-- =============================================================================
-- Ejecutar UNA VEZ en la base mascotas local antes de sincronizar stock.
-- Después: panel sycronizafhir → sync/bootstrap de productos_depositos.
-- =============================================================================

BEGIN;

ALTER TABLE public.productos_depositos
  ADD COLUMN IF NOT EXISTS fecha_modificacion date NOT NULL DEFAULT CURRENT_DATE;

UPDATE public.productos_depositos
SET fecha_modificacion = COALESCE(fecha, CURRENT_DATE)
WHERE fecha_modificacion IS NULL;

CREATE OR REPLACE FUNCTION public.fn_productos_depositos_modificacion()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
  NEW.fecha_modificacion := CURRENT_DATE;
  RETURN NEW;
END;
$function$;

DROP TRIGGER IF EXISTS tr_productos_depositos_modificacion ON public.productos_depositos;
CREATE TRIGGER tr_productos_depositos_modificacion
  BEFORE INSERT OR UPDATE ON public.productos_depositos
  FOR EACH ROW
  EXECUTE PROCEDURE public.fn_productos_depositos_modificacion();

-- Marcar todo para carga inicial a Supabase (una sola vez).
UPDATE public.productos_depositos SET fecha_modificacion = CURRENT_DATE;

COMMIT;
