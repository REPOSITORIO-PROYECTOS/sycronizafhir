package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// TableModifiedAtMeta describe cómo interpretar fecha_modificacion / hora_modificacion.
type TableModifiedAtMeta struct {
	FechaIsDate         bool
	HasHoraModificacion bool
}

func (db *LocalPG) LoadTableModifiedAtMeta(ctx context.Context, schemaName, tableName string) (TableModifiedAtMeta, error) {
	if !safeIdentifierPattern.MatchString(schemaName) {
		return TableModifiedAtMeta{}, fmt.Errorf("invalid schema name: %s", schemaName)
	}
	if !safeIdentifierPattern.MatchString(tableName) {
		return TableModifiedAtMeta{}, fmt.Errorf("invalid table name: %s", tableName)
	}

	const query = `
		SELECT column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = $1
		  AND table_name = $2
		  AND column_name IN ('fecha_modificacion', 'hora_modificacion')`

	rows, err := db.pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return TableModifiedAtMeta{}, err
	}
	defer rows.Close()

	meta := TableModifiedAtMeta{}
	for rows.Next() {
		var columnName, dataType string
		if scanErr := rows.Scan(&columnName, &dataType); scanErr != nil {
			return TableModifiedAtMeta{}, scanErr
		}
		switch columnName {
		case "fecha_modificacion":
			meta.FechaIsDate = dataType == "date"
		case "hora_modificacion":
			meta.HasHoraModificacion = true
		}
	}
	if err = rows.Err(); err != nil {
		return TableModifiedAtMeta{}, err
	}
	return meta, nil
}

// ModifiedAtFilterExpr devuelve la expresión SQL y el ORDER BY para cursores incrementales.
func ModifiedAtFilterExpr(meta TableModifiedAtMeta) (filterExpr string, orderBy string) {
	if meta.HasHoraModificacion {
		return `(fecha_modificacion::timestamp + COALESCE(hora_modificacion, TIME '00:00:00'))`,
			`fecha_modificacion ASC, hora_modificacion ASC`
	}
	if meta.FechaIsDate {
		return `fecha_modificacion`, `fecha_modificacion ASC`
	}
	return `fecha_modificacion`, `fecha_modificacion ASC`
}

// ModifiedAtWhereClause compara contra el checkpoint $1 (timestamptz).
func ModifiedAtWhereClause(meta TableModifiedAtMeta) string {
	expr, _ := ModifiedAtFilterExpr(meta)
	if meta.HasHoraModificacion {
		return fmt.Sprintf("%s > $1", expr)
	}
	if meta.FechaIsDate {
		return fmt.Sprintf("%s > $1::date", expr)
	}
	return fmt.Sprintf("%s > $1", expr)
}

func RowModifiedAt(row map[string]interface{}, meta TableModifiedAtMeta) (time.Time, bool) {
	fecha, ok := parseFechaModificacion(row["fecha_modificacion"])
	if !ok {
		return time.Time{}, false
	}

	if meta.HasHoraModificacion {
		hora, horaOK := parseHoraModificacion(row["hora_modificacion"])
		if horaOK {
			combined := time.Date(
				fecha.Year(), fecha.Month(), fecha.Day(),
				hora.Hour(), hora.Minute(), hora.Second(), hora.Nanosecond(),
				time.UTC,
			)
			return combined, true
		}
	}

	if meta.FechaIsDate {
		return time.Date(fecha.Year(), fecha.Month(), fecha.Day(), 0, 0, 0, 0, time.UTC), true
	}
	return fecha.UTC(), true
}

func MaxRowModifiedAt(rows []map[string]interface{}, meta TableModifiedAtMeta) (time.Time, bool) {
	var maxAt time.Time
	found := false
	for _, row := range rows {
		modifiedAt, ok := RowModifiedAt(row, meta)
		if !ok {
			continue
		}
		if !found || modifiedAt.After(maxAt) {
			maxAt = modifiedAt
			found = true
		}
	}
	return maxAt, found
}

func parseFechaModificacion(raw interface{}) (time.Time, bool) {
	switch value := raw.(type) {
	case time.Time:
		return value, true
	case pgtype.Date:
		if !value.Valid {
			return time.Time{}, false
		}
		return value.Time, true
	case pgtype.Timestamp:
		if !value.Valid {
			return time.Time{}, false
		}
		return value.Time, true
	case pgtype.Timestamptz:
		if !value.Valid {
			return time.Time{}, false
		}
		return value.Time, true
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return time.Time{}, false
		}
		layouts := []string{
			time.RFC3339Nano,
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, layout := range layouts {
			parsed, err := time.Parse(layout, text)
			if err == nil {
				return parsed, true
			}
		}
	case []byte:
		return parseFechaModificacion(string(value))
	}
	return time.Time{}, false
}

func parseHoraModificacion(raw interface{}) (time.Time, bool) {
	switch value := raw.(type) {
	case time.Time:
		return value, true
	case pgtype.Time:
		if !value.Valid {
			return time.Time{}, false
		}
		return pgTimeToClock(value), true
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return time.Time{}, false
		}
		layouts := []string{
			"15:04:05.999999",
			"15:04:05",
			time.RFC3339Nano,
		}
		for _, layout := range layouts {
			parsed, err := time.Parse(layout, text)
			if err == nil {
				return parsed, true
			}
		}
	case []byte:
		return parseHoraModificacion(string(value))
	}
	return time.Time{}, false
}

func pgTimeToClock(value pgtype.Time) time.Time {
	usecs := value.Microseconds
	if usecs < 0 {
		usecs = 0
	}
	hours := usecs / 3_600_000_000
	usecs %= 3_600_000_000
	minutes := usecs / 60_000_000
	usecs %= 60_000_000
	seconds := usecs / 1_000_000
	nanos := (usecs % 1_000_000) * 1_000
	return time.Date(0, 1, 1, int(hours), int(minutes), int(seconds), int(nanos), time.UTC)
}
