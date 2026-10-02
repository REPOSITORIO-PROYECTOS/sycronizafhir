-- pendientes + cliente_sucursal en Supabase (réplica ERP Misan).
-- Esquema medido en mascotas (SERVIDOR) 2026-10-02.
-- sucursal_id es integer (el ERP lo genera). En la nube no hay sequence:
-- la clave la manda el ERP.

BEGIN;

CREATE TABLE IF NOT EXISTS public.pendientes (
  ped_id character(13) NOT NULL,
  prod_id character(8) NOT NULL,
  prod_descripcion character varying(200),
  cantidad numeric(15,2),
  estado character(1),
  clien_id smallint,
  fecha date,
  cantidad_original numeric(15,2),
  actu character(1),
  comprobante character(13),
  descuento numeric(15,2),
  fecha_modificacion date,
  hora_modificacion time without time zone,
  CONSTRAINT pk_pendientes PRIMARY KEY (ped_id, prod_id)
);

CREATE TABLE IF NOT EXISTS public.cliente_sucursal (
  sucursal_id integer NOT NULL,
  clien_id smallint NOT NULL,
  sucursal_nombre character varying(80) NOT NULL,
  domicilio character varying(100),
  localidad character varying(50),
  provi_id character(1),
  cp character varying(10),
  telefono character varying(50),
  sistema_instalado character(1) DEFAULT 'S',
  activa character(1) DEFAULT 'S',
  fecha_modificacion date,
  hora_modificacion time without time zone,
  CONSTRAINT pk_cliente_sucursal PRIMARY KEY (sucursal_id)
);

CREATE INDEX IF NOT EXISTS idx_cliente_sucursal_clien_id
  ON public.cliente_sucursal (clien_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.pendientes TO postgres, service_role;
GRANT SELECT ON TABLE public.pendientes TO anon, authenticated;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.cliente_sucursal TO postgres, service_role;
GRANT SELECT ON TABLE public.cliente_sucursal TO anon, authenticated;

NOTIFY pgrst, 'reload schema';

COMMIT;
