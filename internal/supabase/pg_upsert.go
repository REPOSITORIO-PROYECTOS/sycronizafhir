package supabase

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGClient struct {
	pool *pgxpool.Pool
}

var safeIdentifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type requiredInsertColumn struct {
	Name     string
	DataType string
}

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

	columnSet := make(map[string]bool)
	for _, row := range rows {
		for key := range row {
			if !safeIdentifierRegex.MatchString(key) {
				return fmt.Errorf("invalid column name %s", key)
			}
			columnSet[key] = true
		}
	}
	columnNames := make([]string, 0, len(columnSet))
	for key := range columnSet {
		columnNames = append(columnNames, key)
	}
	sort.Strings(columnNames)

	allowedColumns, err := c.readTableColumns(ctx, schemaName, tableName)
	if err != nil {
		return err
	}
	columnNames = filterAllowedColumns(columnNames, allowedColumns)
	if len(columnNames) == 0 {
		return fmt.Errorf("no compatible columns for target table %s.%s", schemaName, tableName)
	}

	requiredColumns, err := c.readRequiredInsertColumns(ctx, schemaName, tableName)
	if err != nil {
		return err
	}
	fillMissingRequired := strings.EqualFold(strings.TrimSpace(os.Getenv("SYNC_FILL_MISSING_REQUIRED")), "1") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("SYNC_FILL_MISSING_REQUIRED")), "true")

	filteredConflicts := filterAllowedColumns(conflictColumns, allowedColumns)
	if len(filteredConflicts) == 0 {
		if remotePK, pkErr := c.readPrimaryKeys(ctx, schemaName, tableName); pkErr == nil && len(remotePK) > 0 {
			filteredConflicts = filterAllowedColumns(remotePK, allowedColumns)
		}
	}
	if len(filteredConflicts) == 0 {
		if uniqueConstraints, uniqErr := c.readUniqueConstraints(ctx, schemaName, tableName); uniqErr == nil && len(uniqueConstraints) > 0 {
			best := pickBestConflict(uniqueConstraints, columnNames, allowedColumns)
			if len(best) > 0 {
				filteredConflicts = best
			}
		}
	}
	if len(filteredConflicts) == 0 {
		return fmt.Errorf("no compatible conflict columns for target table %s.%s", schemaName, tableName)
	}

	for _, row := range rows {
		if len(filteredConflicts) == 1 &&
			filteredConflicts[0] == "id" &&
			allowedColumns["id"] &&
			len(conflictColumns) == 1 {
			if _, hasID := row["id"]; !hasID {
				if v, ok := row[conflictColumns[0]]; ok {
					row["id"] = v
				}
			}
		}

		rowColumns := make([]string, 0, len(row))
		for key := range row {
			if allowedColumns[key] && row[key] != nil {
				rowColumns = append(rowColumns, key)
			}
		}
		sort.Strings(rowColumns)
		if len(rowColumns) == 0 {
			continue
		}
		for _, name := range filteredConflicts {
			if _, ok := row[name]; !ok {
				return fmt.Errorf("conflict column %s missing in payload for %s.%s", name, schemaName, tableName)
			}
		}

		missingRequired := false
		for _, required := range requiredColumns {
			if _, ok := row[required.Name]; !ok || row[required.Name] == nil {
				missingRequired = true
				break
			}
		}

		if missingRequired {
			if fillMissingRequired {
				for _, required := range requiredColumns {
					if _, ok := row[required.Name]; ok && row[required.Name] != nil {
						continue
					}
					row[required.Name] = mockValueForRequired(required, row, filteredConflicts)
				}

				rowColumns = make([]string, 0, len(row))
				for key := range row {
					if allowedColumns[key] && row[key] != nil {
						rowColumns = append(rowColumns, key)
					}
				}
				sort.Strings(rowColumns)
			} else {
				updated, updateErr := c.updateExistingRow(ctx, schemaName, tableName, rowColumns, filteredConflicts, row)
				if updateErr != nil {
					return updateErr
				}
				if !updated {
					return fmt.Errorf("row missing required columns for insert into %s.%s and does not exist for update", schemaName, tableName)
				}
				continue
			}
		}

		if err = c.upsertSingleRow(ctx, schemaName, tableName, rowColumns, filteredConflicts, row); err != nil {
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

func (c *PGClient) readPrimaryKeys(ctx context.Context, schemaName, tableName string) ([]string, error) {
	const query = `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		   AND tc.table_schema = kcu.table_schema
		   AND tc.table_name = kcu.table_name
		WHERE tc.constraint_type = 'PRIMARY KEY'
		  AND tc.table_schema = $1
		  AND tc.table_name = $2
		ORDER BY kcu.ordinal_position`

	rows, err := c.pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]string, 0)
	for rows.Next() {
		var columnName string
		if err = rows.Scan(&columnName); err != nil {
			return nil, err
		}
		keys = append(keys, columnName)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}

func (c *PGClient) readRequiredInsertColumns(ctx context.Context, schemaName, tableName string) ([]requiredInsertColumn, error) {
	const query = `
		SELECT column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = $1
		  AND table_name = $2
		  AND is_nullable = 'NO'
		  AND column_default IS NULL
		  AND is_identity = 'NO'
		ORDER BY ordinal_position`

	rows, err := c.pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := make([]requiredInsertColumn, 0)
	for rows.Next() {
		var columnName string
		var dataType string
		if err = rows.Scan(&columnName, &dataType); err != nil {
			return nil, err
		}
		cols = append(cols, requiredInsertColumn{Name: columnName, DataType: dataType})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return cols, nil
}

func (c *PGClient) readUniqueConstraints(ctx context.Context, schemaName, tableName string) ([][]string, error) {
	const query = `
		SELECT tc.constraint_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		   AND tc.table_schema = kcu.table_schema
		   AND tc.table_name = kcu.table_name
		WHERE tc.constraint_type = 'UNIQUE'
		  AND tc.table_schema = $1
		  AND tc.table_name = $2
		ORDER BY tc.constraint_name, kcu.ordinal_position`

	rows, err := c.pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grouped := map[string][]string{}
	order := make([]string, 0)
	seen := map[string]bool{}
	for rows.Next() {
		var constraintName string
		var columnName string
		if err = rows.Scan(&constraintName, &columnName); err != nil {
			return nil, err
		}
		if !seen[constraintName] {
			seen[constraintName] = true
			order = append(order, constraintName)
		}
		grouped[constraintName] = append(grouped[constraintName], columnName)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	constraints := make([][]string, 0, len(order))
	for _, name := range order {
		constraints = append(constraints, grouped[name])
	}
	return constraints, nil
}

func pickBestConflict(candidates [][]string, payloadColumns []string, allowedColumns map[string]bool) []string {
	valid := make([][]string, 0)
	for _, cols := range candidates {
		filtered := filterAllowedColumns(cols, allowedColumns)
		if len(filtered) == 0 {
			continue
		}
		ok := true
		for _, col := range filtered {
			if !slices.Contains(payloadColumns, col) {
				ok = false
				break
			}
		}
		if ok {
			valid = append(valid, filtered)
		}
	}
	if len(valid) == 0 {
		return nil
	}

	sort.Slice(valid, func(i, j int) bool {
		if len(valid[i]) != len(valid[j]) {
			return len(valid[i]) < len(valid[j])
		}
		return strings.Join(valid[i], ",") < strings.Join(valid[j], ",")
	})
	return valid[0]
}

func (c *PGClient) updateExistingRow(ctx context.Context, schemaName, tableName string, rowColumns, conflictColumns []string, row map[string]interface{}) (bool, error) {
	whereParts := make([]string, 0, len(conflictColumns))
	whereValues := make([]interface{}, 0, len(conflictColumns))

	for i, name := range conflictColumns {
		if row[name] == nil {
			return false, fmt.Errorf("conflict column %s missing in payload for %s.%s", name, schemaName, tableName)
		}
		whereParts = append(whereParts, fmt.Sprintf("%s = $%d", quoteIdentifier(name), i+1))
		whereValues = append(whereValues, row[name])
	}

	updateColumns := make([]string, 0, len(rowColumns))
	for _, name := range rowColumns {
		if slices.Contains(conflictColumns, name) {
			continue
		}
		updateColumns = append(updateColumns, name)
	}

	if len(updateColumns) == 0 {
		query := fmt.Sprintf(
			`SELECT 1 FROM %s.%s WHERE %s LIMIT 1`,
			quoteIdentifier(schemaName),
			quoteIdentifier(tableName),
			strings.Join(whereParts, " AND "),
		)
		var one int
		err := c.pool.QueryRow(ctx, query, whereValues...).Scan(&one)
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return err == nil, err
	}

	setParts := make([]string, 0, len(updateColumns))
	values := make([]interface{}, 0, len(updateColumns)+len(whereValues))
	for i, name := range updateColumns {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", quoteIdentifier(name), i+1))
		values = append(values, row[name])
	}

	wherePartsUpdate := make([]string, 0, len(conflictColumns))
	for i, name := range conflictColumns {
		wherePartsUpdate = append(wherePartsUpdate, fmt.Sprintf("%s = $%d", quoteIdentifier(name), len(updateColumns)+i+1))
	}
	values = append(values, whereValues...)

	query := fmt.Sprintf(
		`UPDATE %s.%s SET %s WHERE %s`,
		quoteIdentifier(schemaName),
		quoteIdentifier(tableName),
		strings.Join(setParts, ", "),
		strings.Join(wherePartsUpdate, " AND "),
	)

	tag, err := c.pool.Exec(ctx, query, values...)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func mockValueForRequired(required requiredInsertColumn, row map[string]interface{}, conflictColumns []string) interface{} {
	name := strings.ToLower(required.Name)
	dataType := strings.ToLower(required.DataType)

	if required.Name == "id" && row["id"] == nil && len(conflictColumns) > 0 && row[conflictColumns[0]] != nil {
		return row[conflictColumns[0]]
	}

	if strings.Contains(dataType, "double") || strings.Contains(dataType, "numeric") || strings.Contains(dataType, "real") ||
		strings.Contains(dataType, "integer") || strings.Contains(dataType, "bigint") || strings.Contains(dataType, "smallint") {
		return 0
	}
	if strings.Contains(dataType, "boolean") {
		return false
	}
	if strings.Contains(name, "fecha") {
		return "1970-01-01"
	}
	if strings.Contains(name, "estado") {
		return "mock"
	}
	if strings.Contains(name, "tipo") {
		return "mock"
	}
	if strings.Contains(name, "numero") {
		return "0"
	}
	return "mock"
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
