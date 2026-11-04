package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
)

// Tablas con muchas filas cambiadas (p. ej. un UPDATE masivo que re-fecha toda
// productos_x_provincias) no entran en el tope de 90s por tabla. Se suben por
// tramos, cada tramo con su propio timeout, salteando filas que ya son iguales
// en la nube y avanzando el checkpoint al terminar cada tramo.
const (
	outboundChunkThreshold = 5000
	outboundChunkSize      = 2000
	outboundChunkTimeout   = 90 * time.Second
)

// chunkRows parte rows en tramos de hasta size filas (sin copiar).
func chunkRows(rows []map[string]interface{}, size int) [][]map[string]interface{} {
	if size <= 0 {
		size = len(rows)
	}
	chunks := make([][]map[string]interface{}, 0, (len(rows)+size-1)/max(size, 1))
	for start := 0; start < len(rows); start += size {
		end := start + size
		if end > len(rows) {
			end = len(rows)
		}
		chunks = append(chunks, rows[start:end])
	}
	return chunks
}

// sortRowsByModifiedAt ordena de forma estable por fecha/hora de modificacion
// (filas sin fecha al final) para poder avanzar el checkpoint por tramos.
func sortRowsByModifiedAt(rows []map[string]interface{}, meta db.TableModifiedAtMeta) {
	sort.SliceStable(rows, func(i, j int) bool {
		ai, okI := db.RowModifiedAt(rows[i], meta)
		aj, okJ := db.RowModifiedAt(rows[j], meta)
		if okI != okJ {
			return okI
		}
		return ai.Before(aj)
	})
}

// chunkCheckpoint decide hasta donde se puede avanzar el checkpoint despues de
// subir chunk. El filtro de lectura es estricto (> checkpoint), asi que solo se
// avanza si el tramo siguiente empieza despues; si no, se perderian filas con
// la misma marca de tiempo.
func chunkCheckpoint(chunk, next []map[string]interface{}, meta db.TableModifiedAtMeta) (time.Time, bool) {
	maxAt, ok := db.MaxRowModifiedAt(chunk, meta)
	if !ok {
		return time.Time{}, false
	}
	if len(next) == 0 {
		return maxAt, true
	}
	nextAt, nextOK := db.RowModifiedAt(next[0], meta)
	if !nextOK || !nextAt.After(maxAt) {
		return time.Time{}, false
	}
	return maxAt, true
}

func pkKey(row map[string]interface{}, pk []string) (string, bool) {
	parts := make([]string, 0, len(pk))
	for _, column := range pk {
		value, ok := row[column]
		if !ok || value == nil {
			return "", false
		}
		parts = append(parts, comparableValue(value))
	}
	return strings.Join(parts, "\x1f"), true
}

// comparableValue lleva valores del ERP y de Supabase a un texto comparable.
// Ante la duda devuelve algo distinto: peor caso se reenvia una fila igual.
func comparableValue(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return "<nil>"
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano)
	case pgtype.Numeric:
		f, err := v.Float64Value()
		if err != nil || !f.Valid {
			return "<numeric?>"
		}
		return strconv.FormatFloat(f.Float64, 'f', -1, 64)
	case pgtype.Date:
		if !v.Valid {
			return "<nil>"
		}
		return v.Time.UTC().Format(time.RFC3339Nano)
	case pgtype.Time:
		if !v.Valid {
			return "<nil>"
		}
		return pgTimeClock(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 64)
	case int, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case bool:
		return strconv.FormatBool(v)
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(raw)
	}
}

func pgTimeClock(v pgtype.Time) string {
	usecs := v.Microseconds
	h := usecs / 3_600_000_000
	usecs %= 3_600_000_000
	m := usecs / 60_000_000
	usecs %= 60_000_000
	s := usecs / 1_000_000
	frac := usecs % 1_000_000
	if frac == 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return strings.TrimRight(fmt.Sprintf("%02d:%02d:%02d.%06d", h, m, s, frac), "0")
}

// filterUnchangedRows saca las filas locales que en la nube ya tienen los
// mismos valores en todas sus columnas.
func filterUnchangedRows(local, remote []map[string]interface{}, pk []string) []map[string]interface{} {
	if len(remote) == 0 {
		return local
	}
	byKey := make(map[string]map[string]interface{}, len(remote))
	for _, row := range remote {
		if key, ok := pkKey(row, pk); ok {
			byKey[key] = row
		}
	}
	out := make([]map[string]interface{}, 0, len(local))
	for _, row := range local {
		key, ok := pkKey(row, pk)
		if !ok {
			out = append(out, row)
			continue
		}
		cloud, exists := byKey[key]
		if !exists || !rowEqual(row, cloud) {
			out = append(out, row)
		}
	}
	return out
}

func rowEqual(local, cloud map[string]interface{}) bool {
	for column, value := range local {
		cloudValue, ok := cloud[column]
		if !ok {
			return false
		}
		if comparableValue(value) != comparableValue(cloudValue) {
			return false
		}
	}
	return true
}

// uploadChunked sube una tabla grande por tramos. Devuelve filas subidas,
// filas salteadas por iguales y el primer error (el checkpoint queda en el
// ultimo tramo seguro, asi el ciclo siguiente retoma desde ahi).
func (w *OutboundWorker) uploadChunked(
	ctx context.Context,
	syncCfg config.SyncTablesConfig,
	table db.SyncTable,
	rows []map[string]interface{},
) (int, int, error) {
	meta, err := w.localPG.LoadTableModifiedAtMeta(ctx, w.sourceSchema, table.Name)
	if err != nil {
		return 0, 0, err
	}
	sortRowsByModifiedAt(rows, meta)
	chunks := chunkRows(rows, outboundChunkSize)
	sent, skipped := 0, 0
	for index, chunk := range chunks {
		chunkCtx, cancel := context.WithTimeout(ctx, outboundChunkTimeout)
		pkRows := make([]map[string]interface{}, 0, len(chunk))
		for _, row := range chunk {
			pkOnly := make(map[string]interface{}, len(table.PrimaryKeys))
			for _, column := range table.PrimaryKeys {
				pkOnly[column] = row[column]
			}
			pkRows = append(pkRows, pkOnly)
		}
		toSend := chunk
		remote, loadErr := w.pgClient.LoadRowsByPrimaryKeys(chunkCtx, "public", table.Name, table.PrimaryKeys, pkRows)
		if loadErr != nil {
			log.Printf("outbound %s chunk %d: no se pudo comparar con nube, se envia completo: %v", table.Name, index+1, loadErr)
		} else {
			toSend = filterUnchangedRows(chunk, remote, table.PrimaryKeys)
		}
		skipped += len(chunk) - len(toSend)
		if len(toSend) > 0 {
			toSend = w.applyOutboundGuards(chunkCtx, syncCfg, table, toSend)
			if upErr := w.pgClient.UpsertRows(chunkCtx, "public", table.Name, toSend, table.PrimaryKeys); upErr != nil {
				cancel()
				return sent, skipped, fmt.Errorf("chunk %d/%d: %w", index+1, len(chunks), upErr)
			}
			sent += len(toSend)
		}
		cancel()
		var next []map[string]interface{}
		if index+1 < len(chunks) {
			next = chunks[index+1]
		}
		if at, ok := chunkCheckpoint(chunk, next, meta); ok {
			if at.After(w.tableSinceFor(table.Name)) {
				w.tableSince[table.Name] = at.UTC()
				if persistErr := w.persistTableCheckpoint(ctx, table.Name, at.UTC()); persistErr != nil {
					log.Printf("persist outbound checkpoint for %s failed: %v", table.Name, persistErr)
				}
			}
		}
	}
	return sent, skipped, nil
}

// replaceQueuedTable deja una sola entrada en la cola por tabla: borra las
// anteriores de esa tabla antes de encolar la nueva.
func (w *OutboundWorker) replaceQueuedTable(ctx context.Context, tableName, payload string) error {
	jobs, err := w.queue.PeekByDirection(ctx, outboundGenericDirection, 1000)
	if err == nil {
		for _, job := range jobs {
			if queuedTableName(job.PayloadJSON) == tableName {
				_ = w.queue.Delete(ctx, job.ID)
			}
		}
	}
	return w.queue.Enqueue(ctx, outboundGenericDirection, payload)
}

func queuedTableName(payloadJSON string) string {
	var head struct {
		TableName string `json:"table_name"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &head); err != nil {
		return ""
	}
	return head.TableName
}
