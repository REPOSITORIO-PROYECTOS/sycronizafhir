package supabase

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PGClient struct {
	pool *pgxpool.Pool
}

var safeIdentifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func NewPGClient(ctx context.Context, dsn string) (*PGClient, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &PGClient{pool: pool}, nil
}

func (c *PGClient) Close() {
	c.pool.Close()
}

func (c *PGClient) Ping(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

func (c *PGClient) UpsertRows(ctx context.Context, schemaName, tableName string, rows []map[string]interface{}, conflictColumns []string) error {
	if len(rows) == 0 {
		return nil
	}
	if !safeIdentifierRegex.MatchString(schemaName) || !safeIdentifierRegex.MatchString(tableName) {
		return fmt.Errorf("invalid target identifier %s.%s", schemaName, tableName)
	}
	if len(conflictColumns) == 0 {
		return fmt.Errorf("conflict columns are required for table %s", tableName)
	}

	firstRow := rows[0]
	columnNames := make([]string, 0, len(firstRow))
	for key := range firstRow {
		if !safeIdentifierRegex.MatchString(key) {
			return fmt.Errorf("invalid column name %s", key)
		}
		columnNames = append(columnNames, key)
	}

	for _, row := range rows {
		if err := c.upsertSingleRow(ctx, schemaName, tableName, columnNames, conflictColumns, row); err != nil {
			return err
		}
	}

	return nil
}

func (c *PGClient) upsertSingleRow(ctx context.Context, schemaName, tableName string, columnNames, conflictColumns []string, row map[string]interface{}) error {
	quotedColumns := make([]string, 0, len(columnNames))
	valuePlaceholders := make([]string, 0, len(columnNames))
	values := make([]interface{}, 0, len(columnNames))
	for index, name := range columnNames {
		quotedColumns = append(quotedColumns, quoteIdentifier(name))
		valuePlaceholders = append(valuePlaceholders, fmt.Sprintf("$%d", index+1))
		values = append(values, row[name])
	}

	conflictQuoted := make([]string, 0, len(conflictColumns))
	conflictSet := map[string]bool{}
	for _, name := range conflictColumns {
		if !safeIdentifierRegex.MatchString(name) {
			return fmt.Errorf("invalid conflict column %s", name)
		}
		conflictQuoted = append(conflictQuoted, quoteIdentifier(name))
		conflictSet[name] = true
	}

	updates := make([]string, 0, len(columnNames))
	for _, name := range columnNames {
		if conflictSet[name] {
			continue
		}
		quoted := quoteIdentifier(name)
		updates = append(updates, fmt.Sprintf("%s = EXCLUDED.%s", quoted, quoted))
	}

	var conflictClause string
	if len(updates) == 0 {
		conflictClause = "DO NOTHING"
	} else {
		conflictClause = "DO UPDATE SET " + strings.Join(updates, ", ")
	}

	query := fmt.Sprintf(
		"INSERT INTO %s.%s (%s) VALUES (%s) ON CONFLICT (%s) %s",
		quoteIdentifier(schemaName),
		quoteIdentifier(tableName),
		strings.Join(quotedColumns, ", "),
		strings.Join(valuePlaceholders, ", "),
		strings.Join(conflictQuoted, ", "),
		conflictClause,
	)

	_, err := c.pool.Exec(ctx, query, values...)
	return err
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
