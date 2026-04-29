package db

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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

	query := fmt.Sprintf(
		`SELECT * FROM %s.%s WHERE fecha_modificacion > $1 ORDER BY fecha_modificacion ASC`,
		schemaName,
		tableName,
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
