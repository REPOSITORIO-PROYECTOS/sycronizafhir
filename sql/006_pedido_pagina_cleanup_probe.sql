-- Limpieza datos de prueba E2E (pedido_pagina id 999001 / WEB-000999001).
-- Idempotente.

BEGIN;

DELETE FROM public.pedido_pagina_detail WHERE pedido_id = 999001;
DELETE FROM public.pedido_pagina WHERE pedido_id = 999001;

COMMIT;
