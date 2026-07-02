-- Corregir tipos: en PostgreSQL `character` sin longitud = character(1).
-- Alinear con mascotas local y public.productos (prod_id character(8)).

BEGIN;

ALTER TABLE public.productos_depositos
  ALTER COLUMN prod_id TYPE character(8),
  ALTER COLUMN local_id TYPE character(4),
  ALTER COLUMN usu_id TYPE character varying(20),
  ALTER COLUMN contador TYPE character varying(20);

COMMIT;
