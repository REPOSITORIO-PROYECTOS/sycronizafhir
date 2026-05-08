package supabase

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGClient struct {
	pool *pgxpool.Pool
}

var safeIdentifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// PoolConfigFromDSN parses a DSN for use with Supabase Postgres (incl. pooler en modo transacción),
// evitando prepared statements persistentes que provocan SQLSTATE 42P05.
func PoolConfigFromDSN(dsn string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.StatementCacheCapacity = 0
	cfg.ConnConfig.DescriptionCacheCapacity = 0
	return cfg, nil
}

func NewPGClient(ctx context.Context, dsn string) (*PGClient, error) {
	cfg, err := PoolConfigFromDSN(dsn)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
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
	allowedColumns, err := c.readTableColumns(ctx, schemaName, tableName)
	if err != nil {
		return err
	}
	columnNames = filterAllowedColumns(columnNames, allowedColumns)
	if len(columnNames) == 0 {
		return fmt.Errorf("no compatible columns for target table %s.%s", schemaName, tableName)
	}

	filteredConflicts := filterAllowedColumns(conflictColumns, allowedColumns)
	if len(filteredConflicts) == 0 {
		return fmt.Errorf("no compatible conflict columns for target table %s.%s", schemaName, tableName)
	}
	for _, name := range filteredConflicts {
		if !slices.Contains(columnNames, name) {
			return fmt.Errorf("conflict column %s missing in payload for %s.%s", name, schemaName, tableName)
		}
	}

	for _, row := range rows {
		if err = c.upsertSingleRow(ctx, schemaName, tableName, columnNames, filteredConflicts, row); err != nil {
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

func (c *PGClient) readTableColumns(ctx context.Context, schemaName, tableName string) (map[string]bool, error) {
	const query = `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = $1
		  AND table_name = $2`

	rows, err := c.pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var columnName string
		if err = rows.Scan(&columnName); err != nil {
			return nil, err
		}
		columns[columnName] = true
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("target table has no columns or does not exist: %s.%s", schemaName, tableName)
	}

	return columns, nil
}

func filterAllowedColumns(columns []string, allowed map[string]bool) []string {
	filtered := make([]string, 0, len(columns))
	for _, columnName := range columns {
		if allowed[columnName] {
			filtered = append(filtered, columnName)
		}
	}
	return filtered
}
