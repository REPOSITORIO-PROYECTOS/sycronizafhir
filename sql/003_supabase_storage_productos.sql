-- Supabase Storage: bucket público para fotos de productos.
-- Dónde: Supabase Dashboard → SQL Editor → pegar y ejecutar.
--
-- Convención de objetos: {prod_id}{ext}  (ej. 00202158.jpg)
-- El middleware sube con SUPABASE_SERVICE_ROLE_KEY (JWT role=service_role).
-- Esa clave bypass RLS; si ves "row-level security policy" en logs, la app
-- está usando la anon key o una clave incorrecta, no este SQL.
-- La lectura es pública por URL directa (bucket public=true).

INSERT INTO storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
VALUES (
  'productos',
  'productos',
  true,
  10485760,
  ARRAY['image/jpeg', 'image/png', 'image/webp', 'image/gif']
)
ON CONFLICT (id) DO UPDATE SET
  public = EXCLUDED.public,
  file_size_limit = EXCLUDED.file_size_limit,
  allowed_mime_types = EXCLUDED.allowed_mime_types;

-- No crear política SELECT para anon/authenticated/public.
-- Con public = true, cada archivo se abre por URL directa:
--   /storage/v1/object/public/productos/{prod_id}.jpg
-- Una política SELECT amplia permitiría LISTAR todo el bucket (riesgo de exposición).
DROP POLICY IF EXISTS "productos_public_read" ON storage.objects;

-- Escritura/actualización/borrado para service_role (middleware).
DROP POLICY IF EXISTS "productos_service_write" ON storage.objects;
CREATE POLICY "productos_service_write"
  ON storage.objects
  FOR ALL
  TO service_role
  USING (bucket_id = 'productos')
  WITH CHECK (bucket_id = 'productos');
