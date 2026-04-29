-- Columnas/indices de control de subida
ALTER TABLE clientes
ADD COLUMN IF NOT EXISTS fecha_modificacion TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_clientes_fecha_mod
ON clientes (fecha_modificacion);

ALTER TABLE articulos
ADD COLUMN IF NOT EXISTS fecha_modificacion TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_articulos_fecha_mod
ON articulos (fecha_modificacion);

-- Buzon de pedidos nube -> local
CREATE TABLE IF NOT EXISTS sync_buzon_pedidos (
    id_buzon SERIAL PRIMARY KEY,
    id_pedido_nube UUID NOT NULL,
    id_cliente INT NOT NULL,
    total DECIMAL(10,2),
    fecha_creacion TIMESTAMP,
    json_detalle JSONB,
    procesado BOOLEAN DEFAULT FALSE,
    error_log TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_sync_buzon_pedidos_id_pedido_nube
ON sync_buzon_pedidos (id_pedido_nube);

-- Trigger seguro de integracion (fase inicial)
-- Nota: en esta etapa no inserta en tablas legacy de PowerBuilder para evitar
-- supuestos de esquema. Solo valida json_detalle y marca/error en el buzon.
CREATE OR REPLACE FUNCTION process_sync_buzon_pedido()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.json_detalle IS NULL THEN
        UPDATE sync_buzon_pedidos
        SET procesado = FALSE,
            error_log = 'json_detalle nulo'
        WHERE id_buzon = NEW.id_buzon;
        RETURN NEW;
    END IF;

    UPDATE sync_buzon_pedidos
    SET procesado = TRUE,
        error_log = NULL
    WHERE id_buzon = NEW.id_buzon;

    RETURN NEW;
EXCEPTION WHEN OTHERS THEN
    UPDATE sync_buzon_pedidos
    SET procesado = FALSE,
        error_log = SQLERRM
    WHERE id_buzon = NEW.id_buzon;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_process_sync_buzon_pedido ON sync_buzon_pedidos;
CREATE TRIGGER trg_process_sync_buzon_pedido
AFTER INSERT ON sync_buzon_pedidos
FOR EACH ROW
EXECUTE FUNCTION process_sync_buzon_pedido();
