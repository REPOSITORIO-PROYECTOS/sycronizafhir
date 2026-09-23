-- =============================================================================
-- productos_x_provincias en Supabase (autorización venta por provincia)
-- =============================================================================
-- Dónde: Supabase Dashboard → SQL Editor, o apply-supabase-sql.exe
--
-- Requisito en Postgres LOCAL (SERVIDOR): stamps + trigger
--   sql/007_productos_x_provincias_local.sql
-- Luego habilitar tabla en sync-tables.json y bootstrap/reconcile.
-- =============================================================================

BEGIN;

CREATE TABLE IF NOT EXISTS public.productos_x_provincias (
  prod_id character(8) NOT NULL,
  provi_id character(1) NOT NULL,
  activo character(1) DEFAULT 'S',
  fecha_modificacion date NOT NULL DEFAULT CURRENT_DATE,
  hora_modificacion time without time zone,
  CONSTRAINT pk_productos_x_provincias PRIMARY KEY (prod_id, provi_id)
);

ALTER TABLE public.productos_x_provincias
  ADD COLUMN IF NOT EXISTS activo character(1) DEFAULT 'S',
  ADD COLUMN IF NOT EXISTS fecha_modificacion date NOT NULL DEFAULT CURRENT_DATE,
  ADD COLUMN IF NOT EXISTS hora_modificacion time without time zone;

CREATE INDEX IF NOT EXISTS idx_productos_x_provincias_provi_activo
  ON public.productos_x_provincias (provi_id, activo);

CREATE INDEX IF NOT EXISTS idx_productos_x_provincias_fecha_modificacion
  ON public.productos_x_provincias (fecha_modificacion);

CREATE INDEX IF NOT EXISTS idx_productos_x_provincias_prod_id
  ON public.productos_x_provincias (prod_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.productos_x_provincias TO postgres;
GRANT SELECT ON TABLE public.productos_x_provincias TO anon, authenticated, service_role;

COMMIT;
