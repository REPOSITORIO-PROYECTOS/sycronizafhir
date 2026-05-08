package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sycronizafhir/internal/models"
)

type LocalPG struct {
	pool *pgxpool.Pool
}

type SyncTable struct {
	Name        string
	PrimaryKeys []string
}

var safeIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func NewLocalPG(ctx context.Context, dsn string) (*LocalPG, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &LocalPG{pool: pool}, nil
}

func (db *LocalPG) Close() {
	db.pool.Close()
}

func (db *LocalPG) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

func (db *LocalPG) LoadUpdatedClientes(ctx context.Context, since time.Time) ([]models.Cliente, error) {
	const query = `
		SELECT id, nombre, email, fecha_modificacion
		FROM clientes
		WHERE fecha_modificacion > $1
		ORDER BY fecha_modificacion ASC`

	rows, err := db.pool.Query(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	clientes := make([]models.Cliente, 0)
	for rows.Next() {
		var cliente models.Cliente
		if err = rows.Scan(&cliente.ID, &cliente.Nombre, &cliente.Email, &cliente.FechaModificacion); err != nil {
			return nil, err
		}

		clientes = append(clientes, cliente)
	}

	return clientes, rows.Err()
}

func (db *LocalPG) LoadUpdatedArticulos(ctx context.Context, since time.Time) ([]models.Articulo, error) {
	const query = `
		SELECT id, nombre, precio, stock, fecha_modificacion
		FROM articulos
		WHERE fecha_modificacion > $1
		ORDER BY fecha_modificacion ASC`

	rows, err := db.pool.Query(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	articulos := make([]models.Articulo, 0)
	for rows.Next() {
		var articulo models.Articulo
		if err = rows.Scan(&articulo.ID, &articulo.Nombre, &articulo.Precio, &articulo.Stock, &articulo.FechaModificacion); err != nil {
			return nil, err
		}

		articulos = append(articulos, articulo)
	}

	return articulos, rows.Err()
}

func (db *LocalPG) InsertPedidoIntoBuzon(ctx context.Context, pedido models.Pedido) error {
	detalleJSON, err := json.Marshal(pedido.Detalle)
	if err != nil {
		return fmt.Errorf("marshal pedido detalle: %w", err)
	}

	const query = `
		INSERT INTO sync_buzon_pedidos (id_pedido_nube, id_cliente, total, fecha_creacion, json_detalle, procesado)
		VALUES ($1, $2, $3, $4, $5, FALSE)
		ON CONFLICT (id_pedido_nube) DO NOTHING`

	_, err = db.pool.Exec(ctx, query, pedido.IDPedidoNube, pedido.IDCliente, pedido.Total, pedido.FechaCreacion, detalleJSON)
	if err != nil {
		return fmt.Errorf("insert pedido into sync_buzon_pedidos: %w", err)
	}

	return nil
}

func (db *LocalPG) ListSyncTables(ctx context.Context, schemaName string, excludeTables []string) ([]SyncTable, error) {
	if !safeIdentifierPattern.MatchString(schemaName) {
		return nil, fmt.Errorf("invalid schema name: %s", schemaName)
	}

	excluded := map[string]bool{"sync_buzon_pedidos": true}
	for _, tableName := range excludeTables {
		excluded[tableName] = true
	}

	const tablesQuery = `
		SELECT t.table_name
		FROM information_schema.tables t
		WHERE t.table_schema = $1
		  AND t.table_type = 'BASE TABLE'
		  AND EXISTS (
			SELECT 1
			FROM information_schema.columns c
			WHERE c.table_schema = t.table_schema
			  AND c.table_name = t.table_name
			  AND c.column_name = 'fecha_modificacion'
		  )
		ORDER BY t.table_name`

	rows, err := db.pool.Query(ctx, tablesQuery, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]SyncTable, 0)
	for rows.Next() {
		var tableName string
		if err = rows.Scan(&tableName); err != nil {
			return nil, err
		}
		if excluded[tableName] {
			continue
		}

		primaryKeys, pkErr := db.readPrimaryKeys(ctx, schemaName, tableName)
		if pkErr != nil {
			return nil, pkErr
		}
		if len(primaryKeys) == 0 {
			continue
		}

		tables = append(tables, SyncTable{
			Name:        tableName,
			PrimaryKeys: primaryKeys,
		})
	}

	return tables, rows.Err()
}

func (db *LocalPG) LoadUpdatedRows(ctx context.Context, schemaName, tableName string, since time.Time) ([]map[string]interface{}, error) {
	if !safeIdentifierPattern.MatchString(schemaName) {
		return nil, fmt.Errorf("invalid schema name: %s", schemaName)
	}
	if !safeIdentifierPattern.MatchString(tableName) {
		return nil, fmt.Errorf("invalid table name: %s", tableName)
	}

	isDateColumn, err := db.isFechaModificacionDate(ctx, schemaName, tableName)
	if err != nil {
		return nil, err
	}

	whereClause := "fecha_modificacion > $1"
	if isDateColumn {
		// Legacy tables often persist only DATE precision. Using >= date avoids
		// losing same-day changes after checkpoint moves to current timestamp.
		whereClause = "fecha_modificacion >= $1::date"
	}

	query := fmt.Sprintf(
		`SELECT * FROM %s.%s WHERE fecha_modificacion IS NOT NULL AND %s ORDER BY fecha_modificacion ASC`,
		schemaName,
		tableName,
		whereClause,
	)

	rows, err := db.pool.Query(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fieldDescriptions := rows.FieldDescriptions()
	result := make([]map[string]interface{}, 0)
	for rows.Next() {
		values, valuesErr := rows.Values()
		if valuesErr != nil {
			return nil, valuesErr
		}

		item := make(map[string]interface{}, len(values))
		for index, field := range fieldDescriptions {
			item[string(field.Name)] = values[index]
		}

		result = append(result, item)
	}

	return result, rows.Err()
}

func (db *LocalPG) CountTableRows(ctx context.Context, schemaName, tableName string) (int64, error) {
	if !safeIdentifierPattern.MatchString(schemaName) {
		return 0, fmt.Errorf("invalid schema name: %s", schemaName)
	}
	if !safeIdentifierPattern.MatchString(tableName) {
		return 0, fmt.Errorf("invalid table name: %s", tableName)
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s`, schemaName, tableName)
	var total int64
	if err := db.pool.QueryRow(ctx, query).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (db *LocalPG) LoadTableRowsChunk(ctx context.Context, schemaName, tableName string, offset, limit int, orderBy []string) ([]map[string]interface{}, error) {
	if !safeIdentifierPattern.MatchString(schemaName) {
		return nil, fmt.Errorf("invalid schema name: %s", schemaName)
	}
	if !safeIdentifierPattern.MatchString(tableName) {
		return nil, fmt.Errorf("invalid table name: %s", tableName)
	}
	if offset < 0 || limit <= 0 {
		return nil, errors.New("invalid pagination values")
	}

	orderColumns := make([]string, 0, len(orderBy))
	for _, column := range orderBy {
		if !safeIdentifierPattern.MatchString(column) {
			return nil, fmt.Errorf("invalid order column: %s", column)
		}
		orderColumns = append(orderColumns, column)
	}
	if len(orderColumns) == 0 {
		orderColumns = append(orderColumns, "fecha_modificacion")
	}

	query := fmt.Sprintf(
		`SELECT * FROM %s.%s ORDER BY %s ASC LIMIT $1 OFFSET $2`,
		schemaName,
		tableName,
		strings.Join(orderColumns, ", "),
	)
	rows, err := db.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fieldDescriptions := rows.FieldDescriptions()
	result := make([]map[string]interface{}, 0, limit)
	for rows.Next() {
		values, valuesErr := rows.Values()
		if valuesErr != nil {
			return nil, valuesErr
		}

		item := make(map[string]interface{}, len(values))
		for index, field := range fieldDescriptions {
			item[string(field.Name)] = values[index]
		}
		result = append(result, item)
	}

	return result, rows.Err()
}

func (db *LocalPG) isFechaModificacionDate(ctx context.Context, schemaName, tableName string) (bool, error) {
	const query = `
		SELECT data_type
		FROM information_schema.columns
		WHERE table_schema = $1
		  AND table_name = $2
		  AND column_name = 'fecha_modificacion'
		LIMIT 1`

	var dataType string
	if err := db.pool.QueryRow(ctx, query, schemaName, tableName).Scan(&dataType); err != nil {
		return false, err
	}

	return dataType == "date", nil
}

func (db *LocalPG) readPrimaryKeys(ctx context.Context, schemaName, tableName string) ([]string, error) {
	const query = `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		   AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
		  AND tc.table_schema = $1
		  AND tc.table_name = $2
		ORDER BY kcu.ordinal_position`

	rows, err := db.pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	primaryKeys := make([]string, 0)
	for rows.Next() {
		var columnName string
		if err = rows.Scan(&columnName); err != nil {
			return nil, err
		}
		primaryKeys = append(primaryKeys, columnName)
	}

	return primaryKeys, rows.Err()
}
