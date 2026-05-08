-- Atajo Supabase: solo public.pedidos_d (prueba rápida / primer sync).
-- Esquema completo legacy en nube (pedidos_d, pedidos_d_tor, productos, clientes):
--   sql/000_supabase_prep_completo.sql
--
-- Ejecutá esto en: Supabase Dashboard → SQL Editor → Run (base `postgres`).
--
-- Si tenés RLS en `public`, el rol de la conexión del bridge debe poder escribir
-- (típicamente usuario `postgres` por pooler directo, o políticas acordes).

BEGIN;

CREATE TABLE IF NOT EXISTS public.pedidos_d (
  ped_id character(13) NOT NULL,
  ped_item smallint NOT NULL,
  prod_id character(8),
  ped_prod_descripcion character varying(200),
  ped_cantidad numeric(10,2) NOT NULL,
  ped_precio_unitario numeric(15,2) NOT NULL,
  ped_importe numeric(15,2),
  ped_descuento numeric(15,2),
  ped_iva numeric(15,2),
  marca character(1),
  pendiente smallint,
  estado character(1),
  elimina character(1),
  comision numeric(5,2) DEFAULT 6,
  area character(2),
  estanteria character(2),
  cuerpo character(2),
  estante character(2),
  observacion character varying(400),
  fecha_modificacion date DEFAULT CURRENT_DATE,
  CONSTRAINT pk_pedidos_d PRIMARY KEY (ped_id, ped_item)
);

ALTER TABLE public.pedidos_d
  ADD COLUMN IF NOT EXISTS observacion character varying(400),
  ADD COLUMN IF NOT EXISTS fecha_modificacion date DEFAULT CURRENT_DATE;

CREATE INDEX IF NOT EXISTS idx_pedidos_d_fecha_modificacion
  ON public.pedidos_d (fecha_modificacion);

COMMIT;
