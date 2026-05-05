-- Sincronizacion legacy -> Supabase
-- Este script crea (si no existen) y completa columnas faltantes para:
--   - pedidos_d
--   - pedidos_d_tor
--   - productos
--   - clientes
-- Requisito del worker outbound: PK + fecha_modificacion.

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

CREATE TABLE IF NOT EXISTS public.pedidos_d_tor (
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
  fecha_modificacion date DEFAULT CURRENT_DATE,
  CONSTRAINT pk_pedidos_d_tor PRIMARY KEY (ped_id, ped_item)
);

ALTER TABLE public.pedidos_d_tor
  ADD COLUMN IF NOT EXISTS fecha_modificacion date DEFAULT CURRENT_DATE;

CREATE INDEX IF NOT EXISTS idx_pedidos_d_tor_fecha_modificacion
  ON public.pedidos_d_tor (fecha_modificacion);

CREATE TABLE IF NOT EXISTS public.productos (
  prod_id character(8) NOT NULL,
  prod_descripcion character varying(200),
  prod_codigo character(13),
  pres_id smallint,
  rubro_id character(8),
  prod_cant_x_pres numeric(8,3),
  prod_activo character(1) NOT NULL,
  prod_id_padre character(8) NOT NULL,
  prod_tipo character(1) NOT NULL,
  prod_precio numeric(15,2),
  prod_costo numeric(15,3),
  prod_ganancia numeric(15,2),
  prod_impuesto numeric(15,2),
  prod_imagen character varying(200),
  prod_rsp character(1) NOT NULL,
  prod_prov smallint,
  descuento numeric(15,2),
  prod_flete numeric(15,2),
  prod_incremento numeric(15,2),
  prod_fecha_modif character(19),
  prod_stock_min integer,
  prod_stock_max integer,
  prod_orden smallint,
  prod_lista character(1),
  prod_fecha_alta date,
  marca character(1),
  actu character(1) DEFAULT 'N'::bpchar,
  proveedor character varying(30),
  fecha_modif date,
  hora_modif time without time zone,
  pesable character(1) DEFAULT 'N'::bpchar,
  facturar character(1) DEFAULT 'S'::bpchar,
  consolidado character(1) DEFAULT 'N'::bpchar,
  promo character varying(500),
  id_rubro character(4) DEFAULT '0000'::bpchar,
  id_subrubro character(4) DEFAULT '0000'::bpchar,
  producto_veterinario character(1) DEFAULT 'N'::bpchar,
  sku character(6),
  detalle character varying(3000),
  bulto integer,
  destacado character(1) DEFAULT 'N'::bpchar,
  youtube character varying(100),
  evento character(1) DEFAULT 'N'::bpchar,
  fecha_modificacion date DEFAULT CURRENT_DATE,
  CONSTRAINT pk_productos PRIMARY KEY (prod_id)
);

ALTER TABLE public.productos
  ADD COLUMN IF NOT EXISTS fecha_modificacion date DEFAULT CURRENT_DATE;

CREATE INDEX IF NOT EXISTS idx_productos_fecha_modificacion
  ON public.productos (fecha_modificacion);

CREATE TABLE IF NOT EXISTS public.clientes (
  clien_id smallint NOT NULL,
  clien_nombre character varying(80),
  natj_id smallint,
  condiva_id smallint,
  clien_cuit character(13),
  regib_id smallint,
  clien_nro_ib character(12),
  clien_domicilio character varying(60),
  clien_cp character(4),
  clien_telefono character varying(40),
  clien_email character varying(40),
  clien_web character varying(40),
  clien_cumple date,
  clien_celular character varying(40),
  provi_id character(1),
  clien_localidad character varying(30),
  clien_imagen character varying(200),
  descuento numeric(15,2),
  viajante_id character(4),
  clien_activo character(1),
  clien_razon_social character varying(80),
  i_lav_ma_d character(5),
  i_lav_ma_h character(5),
  l_sab_ma_d character(5),
  l_sab_ma_h character(5),
  i_sab_ma_d character(5),
  i_sab_ma_h character(5),
  i_lav_ta_d character(5),
  i_lav_ta_h character(5),
  i_sab_ta_d character(5),
  i_sab_ta_h character(5),
  v_lav_ma_d character(5),
  v_lav_ma_h character(5),
  v_sab_ma_d character(5),
  v_sab_ma_h character(5),
  v_lav_ta_d character(5),
  v_lav_ta_h character(5),
  v_sab_ta_d character(5),
  v_sab_ta_h character(5),
  clien_tipo character(1),
  categoria character(1),
  limite_credito numeric(17,2),
  zona character(2),
  contacto character varying(80),
  actu character(1) DEFAULT 'N'::bpchar,
  clien_tipo_venta character(1) DEFAULT 'E'::bpchar,
  clien_facturar character(1) DEFAULT 'N'::bpchar,
  clien_encomienda character(1) DEFAULT 'N'::bpchar,
  clien_veterinario character(1) DEFAULT 'N'::bpchar,
  evento character(1) DEFAULT 'N'::bpchar,
  domicilio_fiscal character varying(200),
  web character(1) DEFAULT 'N'::bpchar,
  fecha_modificacion date DEFAULT CURRENT_DATE,
  CONSTRAINT pk_clientes PRIMARY KEY (clien_id)
);

ALTER TABLE public.clientes
  ADD COLUMN IF NOT EXISTS clien_nombre character varying(80),
  ADD COLUMN IF NOT EXISTS natj_id smallint,
  ADD COLUMN IF NOT EXISTS condiva_id smallint,
  ADD COLUMN IF NOT EXISTS clien_cuit character(13),
  ADD COLUMN IF NOT EXISTS regib_id smallint,
  ADD COLUMN IF NOT EXISTS clien_nro_ib character(12),
  ADD COLUMN IF NOT EXISTS clien_domicilio character varying(60),
  ADD COLUMN IF NOT EXISTS clien_cp character(4),
  ADD COLUMN IF NOT EXISTS clien_telefono character varying(40),
  ADD COLUMN IF NOT EXISTS clien_email character varying(40),
  ADD COLUMN IF NOT EXISTS clien_web character varying(40),
  ADD COLUMN IF NOT EXISTS clien_cumple date,
  ADD COLUMN IF NOT EXISTS clien_celular character varying(40),
  ADD COLUMN IF NOT EXISTS provi_id character(1),
  ADD COLUMN IF NOT EXISTS clien_localidad character varying(30),
  ADD COLUMN IF NOT EXISTS clien_imagen character varying(200),
  ADD COLUMN IF NOT EXISTS descuento numeric(15,2),
  ADD COLUMN IF NOT EXISTS viajante_id character(4),
  ADD COLUMN IF NOT EXISTS clien_activo character(1),
  ADD COLUMN IF NOT EXISTS clien_razon_social character varying(80),
  ADD COLUMN IF NOT EXISTS i_lav_ma_d character(5),
  ADD COLUMN IF NOT EXISTS i_lav_ma_h character(5),
  ADD COLUMN IF NOT EXISTS l_sab_ma_d character(5),
  ADD COLUMN IF NOT EXISTS l_sab_ma_h character(5),
  ADD COLUMN IF NOT EXISTS i_sab_ma_d character(5),
  ADD COLUMN IF NOT EXISTS i_sab_ma_h character(5),
  ADD COLUMN IF NOT EXISTS i_lav_ta_d character(5),
  ADD COLUMN IF NOT EXISTS i_lav_ta_h character(5),
  ADD COLUMN IF NOT EXISTS i_sab_ta_d character(5),
  ADD COLUMN IF NOT EXISTS i_sab_ta_h character(5),
  ADD COLUMN IF NOT EXISTS v_lav_ma_d character(5),
  ADD COLUMN IF NOT EXISTS v_lav_ma_h character(5),
  ADD COLUMN IF NOT EXISTS v_sab_ma_d character(5),
  ADD COLUMN IF NOT EXISTS v_sab_ma_h character(5),
  ADD COLUMN IF NOT EXISTS v_lav_ta_d character(5),
  ADD COLUMN IF NOT EXISTS v_lav_ta_h character(5),
  ADD COLUMN IF NOT EXISTS v_sab_ta_d character(5),
  ADD COLUMN IF NOT EXISTS v_sab_ta_h character(5),
  ADD COLUMN IF NOT EXISTS clien_tipo character(1),
  ADD COLUMN IF NOT EXISTS categoria character(1),
  ADD COLUMN IF NOT EXISTS limite_credito numeric(17,2),
  ADD COLUMN IF NOT EXISTS zona character(2),
  ADD COLUMN IF NOT EXISTS contacto character varying(80),
  ADD COLUMN IF NOT EXISTS actu character(1) DEFAULT 'N'::bpchar,
  ADD COLUMN IF NOT EXISTS clien_tipo_venta character(1) DEFAULT 'E'::bpchar,
  ADD COLUMN IF NOT EXISTS clien_facturar character(1) DEFAULT 'N'::bpchar,
  ADD COLUMN IF NOT EXISTS clien_encomienda character(1) DEFAULT 'N'::bpchar,
  ADD COLUMN IF NOT EXISTS clien_veterinario character(1) DEFAULT 'N'::bpchar,
  ADD COLUMN IF NOT EXISTS evento character(1) DEFAULT 'N'::bpchar,
  ADD COLUMN IF NOT EXISTS domicilio_fiscal character varying(200),
  ADD COLUMN IF NOT EXISTS web character(1) DEFAULT 'N'::bpchar,
  ADD COLUMN IF NOT EXISTS fecha_modificacion date DEFAULT CURRENT_DATE;

CREATE INDEX IF NOT EXISTS idx_clientes_fecha_modificacion
  ON public.clientes (fecha_modificacion);

COMMIT;
