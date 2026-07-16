BEGIN;

INSERT INTO public.pedido_pagina (
  pedido_id, email, fecha, mailed, razonsocial, cuit, estado, comentario
) VALUES (
  999001, 'e2e-once@test.local', CURRENT_DATE, false,
  'E2E Once', '20123456789', 'N', 'e2e_once_v1518'
) ON CONFLICT (pedido_id) DO NOTHING;

INSERT INTO public.pedido_pagina_detail (
  pedido_id, prod_id, prod_descripcion, prod_codigo, prod_precio, prod_cant,
  fecha_modificacion, hora_modificacion
) VALUES (
  999001, '00202304', 'Item E2E', '00PSF4', 50.00, 1,
  CURRENT_DATE, TIME '15:45:00'
) ON CONFLICT (pedido_id, prod_id) DO UPDATE SET
  prod_cant = EXCLUDED.prod_cant,
  fecha_modificacion = EXCLUDED.fecha_modificacion,
  hora_modificacion = EXCLUDED.hora_modificacion;

COMMIT;
