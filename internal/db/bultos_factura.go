package db

import (
	"context"
	"fmt"
	"time"
)

// BultosFacturaCandidate es la factura más reciente de un pedido
// cuyo pedidos.bultos está vacío (NULL o 0).
type BultosFacturaCandidate struct {
	PedID              string
	PedidoBultos       *int16
	FacturaBultos      int16
	FacturaComprobante string
	FacturaFecha       time.Time
}

// ListBultosFacturaForEmptyPedidos lista, en una ventana de N días, la factura
// más reciente por ped_id (fvc_fecha, luego fvc_comprobante) cuando el pedido
// no tiene bultos cargados. Solo lectura. Compatible con PostgreSQL 9.1.
func (db *LocalPG) ListBultosFacturaForEmptyPedidos(ctx context.Context, schemaName string, windowDays int) ([]BultosFacturaCandidate, error) {
	if !safeIdentifierPattern.MatchString(schemaName) {
		return nil, fmt.Errorf("invalid schema identifier %s", schemaName)
	}
	if windowDays <= 0 {
		return nil, fmt.Errorf("windowDays must be greater than zero")
	}

	query := fmt.Sprintf(`
WITH latest AS (
  SELECT DISTINCT ON (TRIM(f.ped_id))
    TRIM(f.ped_id) AS ped_id_trim,
    f.fvc_comprobante,
    f.fvc_fecha,
    f.bultos AS factura_bultos
  FROM %s.facturas_vtas f
  WHERE f.ped_id IS NOT NULL
    AND TRIM(f.ped_id) <> ''
    AND f.bultos IS NOT NULL
    AND f.bultos > 0
    AND f.fvc_fecha >= CURRENT_DATE - $1::integer
  ORDER BY TRIM(f.ped_id), f.fvc_fecha DESC, f.fvc_comprobante DESC
)
SELECT p.ped_id,
       p.bultos AS pedido_bultos,
       l.factura_bultos,
       l.fvc_comprobante,
       l.fvc_fecha
FROM latest l
JOIN %s.pedidos p ON TRIM(p.ped_id) = l.ped_id_trim
WHERE p.bultos IS NULL OR p.bultos = 0
`, schemaName, schemaName)

	rows, err := db.pool.Query(ctx, query, windowDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]BultosFacturaCandidate, 0)
	for rows.Next() {
		var c BultosFacturaCandidate
		if err := rows.Scan(&c.PedID, &c.PedidoBultos, &c.FacturaBultos, &c.FacturaComprobante, &c.FacturaFecha); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ApplyPedidoBultosIfEmpty escribe bultos en el pedido solo si sigue vacío.
// Nunca pisa un valor ya cargado y no toca facturas_vtas. Devuelve true si
// actualizó una fila.
func (db *LocalPG) ApplyPedidoBultosIfEmpty(ctx context.Context, schemaName, pedID string, bultos int16) (bool, error) {
	if !safeIdentifierPattern.MatchString(schemaName) {
		return false, fmt.Errorf("invalid schema identifier %s", schemaName)
	}
	if bultos <= 0 {
		return false, fmt.Errorf("bultos must be greater than zero")
	}

	nowLocal := ERPWallClock(time.Now())
	fecha := nowLocal.Truncate(24 * time.Hour)
	hora := nowLocal

	query := fmt.Sprintf(`
UPDATE %s.pedidos
SET bultos = $2,
    fecha_modificacion = $3,
    hora_modificacion = $4
WHERE ped_id = $1
  AND (bultos IS NULL OR bultos = 0)
`, schemaName)

	tag, err := db.pool.Exec(ctx, query, pedID, bultos, fecha, hora)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
