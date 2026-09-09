package supabase

import (
	"context"
	"fmt"
	"time"
)

const syncInboundWakeTable = "sync_inbound_wake"

// InboundWake es una señal de Picking (Contabo) para bajar estado ya, sin esperar el poll.
type InboundWake struct {
	ID    int64
	Kind  string
	PedID string
}

// ListPendingInboundWakes lee wakes sin consumir. Si la tabla no existe, ok=false.
func (c *PGClient) ListPendingInboundWakes(ctx context.Context, limit int) (rows []InboundWake, tableOK bool, err error) {
	if limit <= 0 {
		limit = 50
	}
	exists, err := c.TableExists(ctx, "public", syncInboundWakeTable)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}

	query := fmt.Sprintf(`
		SELECT id, COALESCE(kind, ''), COALESCE(ped_id, '')
		FROM public.%s
		WHERE consumed_at IS NULL
		ORDER BY id ASC
		LIMIT $1`, quoteIdentifier(syncInboundWakeTable))

	rs, err := c.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, true, err
	}
	defer rs.Close()

	out := make([]InboundWake, 0, limit)
	for rs.Next() {
		var row InboundWake
		if scanErr := rs.Scan(&row.ID, &row.Kind, &row.PedID); scanErr != nil {
			return nil, true, scanErr
		}
		out = append(out, row)
	}
	return out, true, rs.Err()
}

// MarkInboundWakesConsumed marca wakes procesados (o descartados).
func (c *PGClient) MarkInboundWakesConsumed(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := c.pool.Exec(ctx, `
		UPDATE public.sync_inbound_wake
		SET consumed_at = $1
		WHERE id = ANY($2) AND consumed_at IS NULL`, time.Now().UTC(), ids)
	return err
}
