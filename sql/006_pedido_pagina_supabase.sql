-- Tablas ERP para pedidos tienda web (estado N = nuevo / pendiente).
-- Fuente: backend/app/services/pedido_pagina_service.py

BEGIN;

CREATE TABLE IF NOT EXISTS public.pedido_pagina (
  pedido_id integer NOT NULL,
  email character varying(255),
  fecha date DEFAULT CURRENT_DATE,
  mailed boolean DEFAULT false,
  razonsocial character varying(255),
  cuit character varying(20),
  estado character(1) DEFAULT 'N',
  comentario character varying(500),
  CONSTRAINT pk_pedido_pagina PRIMARY KEY (pedido_id)
);

CREATE TABLE IF NOT EXISTS public.pedido_pagina_detail (
  pedido_id integer NOT NULL,
  prod_id character varying(32) NOT NULL DEFAULT '00000001',
  prod_descripcion character varying(255),
  prod_codigo character varying(64),
  prod_precio numeric(15, 2) DEFAULT 0,
  prod_cant integer DEFAULT 0,
  fecha_modificacion date DEFAULT CURRENT_DATE,
  hora_modificacion time without time zone,
  CONSTRAINT pk_pedido_pagina_detail PRIMARY KEY (pedido_id, prod_id),
  CONSTRAINT fk_pedido_pagina_detail_head
    FOREIGN KEY (pedido_id) REFERENCES public.pedido_pagina (pedido_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_pedido_pagina_estado ON public.pedido_pagina (estado);
CREATE INDEX IF NOT EXISTS idx_pedido_pagina_fecha ON public.pedido_pagina (fecha);

COMMIT;
