-- Supabase: fecha/hora modificacion en pedidos (inbound Picking K/V/E → Mica)
ALTER TABLE public.pedidos
  ADD COLUMN IF NOT EXISTS fecha_modificacion date DEFAULT CURRENT_DATE;

ALTER TABLE public.pedidos
  ADD COLUMN IF NOT EXISTS hora_modificacion time without time zone;

CREATE INDEX IF NOT EXISTS idx_pedidos_fecha_modificacion
  ON public.pedidos (fecha_modificacion);

-- Aplicar: apply-supabase-sql.exe (sycronizafhir) o psql contra pooler Supabase.

ALTER TABLE public.productos
  ADD COLUMN IF NOT EXISTS hora_modificacion time without time zone;

ALTER TABLE public.clientes
  ADD COLUMN IF NOT EXISTS hora_modificacion time without time zone;

COMMENT ON COLUMN public.productos.hora_modificacion IS
  'Hora de última modificación (par con fecha_modificacion); alineado a Mica mascotas.';

COMMENT ON COLUMN public.clientes.hora_modificacion IS
  'Hora de última modificación (par con fecha_modificacion); inbound Picking/tienda → Mica.';
