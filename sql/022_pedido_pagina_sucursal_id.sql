-- Destino del pedido de tienda: sucursal elegida por el cliente.
-- El id sale de public.cliente_sucursal.sucursal_id (integer, lo genera el ERP).
-- Nullable: un pedido sin sucursal sigue grabándose como hasta ahora.
-- Postgres 9.1 (ERP mascotas) no acepta ADD COLUMN IF NOT EXISTS.

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'pedido_pagina'
      AND column_name = 'sucursal_id'
  ) THEN
    ALTER TABLE public.pedido_pagina ADD COLUMN sucursal_id integer;
  END IF;
END $$;
