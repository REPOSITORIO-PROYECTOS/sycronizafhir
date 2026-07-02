-- =============================================================================
-- productos_depositos en Supabase (stock ERP mascotas)
-- =============================================================================
-- Dónde: Supabase Dashboard → SQL Editor, o:
--   apply-supabase-sql.exe sql/004_productos_depositos_supabase.sql
--
-- Requisito en Postgres LOCAL (Mica): columna fecha_modificacion + trigger
--   sql/001_productos_depositos_local.sql
-- Luego habilitar tabla en sync-tables.json y correr bootstrap/sync de
--   productos_depositos desde el panel sycronizafhir.
-- =============================================================================

BEGIN;

CREATE TABLE IF NOT EXISTS public.productos_depositos (
  prod_id character(8) NOT NULL,
  local_id character(4) NOT NULL,
  stock numeric,
  fecha date,
  usu_id character varying(20),
  ultimo_ingreso numeric,
  stock_anterior numeric,
  contador character varying(20),
  fecha_control date,
  fecha_modificacion date NOT NULL DEFAULT CURRENT_DATE,
  CONSTRAINT pk_productos_depositos PRIMARY KEY (prod_id, local_id)
);

ALTER TABLE public.productos_depositos
  ADD COLUMN IF NOT EXISTS stock numeric,
  ADD COLUMN IF NOT EXISTS fecha date,
  ADD COLUMN IF NOT EXISTS usu_id character,
  ADD COLUMN IF NOT EXISTS ultimo_ingreso numeric,
  ADD COLUMN IF NOT EXISTS stock_anterior numeric,
  ADD COLUMN IF NOT EXISTS contador character,
  ADD COLUMN IF NOT EXISTS fecha_control date,
  ADD COLUMN IF NOT EXISTS fecha_modificacion date NOT NULL DEFAULT CURRENT_DATE;

CREATE INDEX IF NOT EXISTS idx_productos_depositos_prod_id
  ON public.productos_depositos (prod_id);

CREATE INDEX IF NOT EXISTS idx_productos_depositos_fecha_modificacion
  ON public.productos_depositos (fecha_modificacion);

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.productos_depositos TO postgres;
GRANT SELECT ON TABLE public.productos_depositos TO anon, authenticated, service_role;

COMMIT;
