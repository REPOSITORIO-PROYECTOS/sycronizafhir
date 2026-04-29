package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sycronizafhir/internal/config"
)

type columnInfo struct {
	Name          string `json:"name"`
	DataType      string `json:"data_type"`
	IsNullable    bool   `json:"is_nullable"`
	ColumnDefault string `json:"column_default,omitempty"`
}

type foreignKeyInfo struct {
	Name             string `json:"name"`
	Column           string `json:"column"`
	ReferencedTable  string `json:"referenced_table"`
	ReferencedColumn string `json:"referenced_column"`
}

type indexInfo struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

type tableInfo struct {
	Schema      string           `json:"schema"`
	Name        string           `json:"name"`
	Columns     []columnInfo     `json:"columns"`
	PrimaryKeys []string         `json:"primary_keys"`
	ForeignKeys []foreignKeyInfo `json:"foreign_keys"`
	Indexes     []indexInfo      `json:"indexes"`
}

type schemaSnapshot struct {
	ScannedAt time.Time   `json:"scanned_at"`
	Database  string      `json:"database"`
	Tables    []tableInfo `json:"tables"`
}

type mailSettings struct {
	smtpHost  string
	smtpPort  string
	smtpUser  string
	smtpPass  string
	fromEmail string
	toEmail   string
	ccEmail   string
	bccEmail  string
	subject   string
	body      string
}

const defaultReportRecipient = "ticianoat@gmail.com"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	recipient := readEnvWithDefault("DBSCAN_EMAIL_TO", defaultReportRecipient)

	cfg, err := config.Load()
	if err != nil {
		fatalWithEmailReport("load config", err, recipient)
	}

	pool, err := pgxpool.New(ctx, cfg.LocalPostgresURL)
	if err != nil {
		fatalWithEmailReport("connect local postgres", err, recipient)
	}
	defer pool.Close()

	if err = pool.Ping(ctx); err != nil {
		fatalWithEmailReport("ping local postgres", err, recipient)
	}

	databaseName, err := readDatabaseName(ctx, pool)
	if err != nil {
		fatalWithEmailReport("read database name", err, recipient)
	}

	tables, err := readTableNames(ctx, pool)
	if err != nil {
		fatalWithEmailReport("read tables", err, recipient)
	}

	snapshot := schemaSnapshot{
		ScannedAt: time.Now().UTC(),
		Database:  databaseName,
		Tables:    make([]tableInfo, 0, len(tables)),
	}

	for _, table := range tables {
		info, readErr := readTableInfo(ctx, pool, "public", table)
		if readErr != nil {
			fatalWithEmailReport(fmt.Sprintf("read table info for %s", table), readErr, recipient)
		}
		snapshot.Tables = append(snapshot.Tables, info)
	}

	sort.Slice(snapshot.Tables, func(i, j int) bool {
		return snapshot.Tables[i].Name < snapshot.Tables[j].Name
	})

	outputPath, err := writeSnapshot(snapshot)
	if err != nil {
		fatalWithEmailReport("write snapshot", err, recipient)
	}

	printSummary(snapshot, outputPath)

	if err = maybeSendEmail(outputPath, snapshot.Database, recipient); err != nil {
		log.Fatalf("send email report: %v", err)
	}
}

func fatalWithEmailReport(stage string, sourceErr error, recipient string) {
	_ = maybeSendFailureEmail(stage, sourceErr, recipient)
	log.Fatalf("%s: %v", stage, sourceErr)
}

func readDatabaseName(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	const query = `SELECT current_database()`
	var databaseName string
	if err := pool.QueryRow(ctx, query).Scan(&databaseName); err != nil {
		return "", err
	}

	return databaseName, nil
}

func readTableNames(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	const query = `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type = 'BASE TABLE'
		ORDER BY table_name`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tableNames := make([]string, 0)
	for rows.Next() {
		var tableName string
		if err = rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, tableName)
	}

	return tableNames, rows.Err()
}

func readTableInfo(ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) (tableInfo, error) {
	columns, err := readColumns(ctx, pool, schemaName, tableName)
	if err != nil {
		return tableInfo{}, err
	}

	primaryKeys, err := readPrimaryKeys(ctx, pool, schemaName, tableName)
	if err != nil {
		return tableInfo{}, err
	}

	foreignKeys, err := readForeignKeys(ctx, pool, schemaName, tableName)
	if err != nil {
		return tableInfo{}, err
	}

	indexes, err := readIndexes(ctx, pool, schemaName, tableName)
	if err != nil {
		return tableInfo{}, err
	}

	return tableInfo{
		Schema:      schemaName,
		Name:        tableName,
		Columns:     columns,
		PrimaryKeys: primaryKeys,
		ForeignKeys: foreignKeys,
		Indexes:     indexes,
	}, nil
}

func readColumns(ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) ([]columnInfo, error) {
	const query = `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = $1
		  AND table_name = $2
		ORDER BY ordinal_position`

	rows, err := pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]columnInfo, 0)
	for rows.Next() {
		var (
			column       columnInfo
			nullable     string
			defaultValue *string
		)
		if err = rows.Scan(&column.Name, &column.DataType, &nullable, &defaultValue); err != nil {
			return nil, err
		}
		column.IsNullable = strings.EqualFold(nullable, "YES")
		if defaultValue != nil {
			column.ColumnDefault = *defaultValue
		}
		columns = append(columns, column)
	}

	return columns, rows.Err()
}

func readPrimaryKeys(ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) ([]string, error) {
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

	rows, err := pool.Query(ctx, query, schemaName, tableName)
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

func readForeignKeys(ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) ([]foreignKeyInfo, error) {
	const query = `
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS referenced_table_name,
			ccu.column_name AS referenced_column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		   AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
		   AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
		  AND tc.table_schema = $1
		  AND tc.table_name = $2
		ORDER BY tc.constraint_name, kcu.ordinal_position`

	rows, err := pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	foreignKeys := make([]foreignKeyInfo, 0)
	for rows.Next() {
		var fk foreignKeyInfo
		if err = rows.Scan(&fk.Name, &fk.Column, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, err
		}
		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, rows.Err()
}

func readIndexes(ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) ([]indexInfo, error) {
	const query = `
		SELECT indexname, indexdef
		FROM pg_indexes
		WHERE schemaname = $1
		  AND tablename = $2
		ORDER BY indexname`

	rows, err := pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make([]indexInfo, 0)
	for rows.Next() {
		var idx indexInfo
		if err = rows.Scan(&idx.Name, &idx.Definition); err != nil {
			return nil, err
		}
		indexes = append(indexes, idx)
	}

	return indexes, rows.Err()
}

func writeSnapshot(snapshot schemaSnapshot) (string, error) {
	if err := os.MkdirAll("reports", 0o755); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("db-schema-scan-%s.json", snapshot.ScannedAt.Format("20060102-150405"))
	outputPath := filepath.Join("reports", filename)

	content, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", err
	}

	if err = os.WriteFile(outputPath, content, 0o644); err != nil {
		return "", err
	}

	return outputPath, nil
}

func printSummary(snapshot schemaSnapshot, outputPath string) {
	totalColumns := 0
	totalForeignKeys := 0
	totalIndexes := 0
	for _, table := range snapshot.Tables {
		totalColumns += len(table.Columns)
		totalForeignKeys += len(table.ForeignKeys)
		totalIndexes += len(table.Indexes)
	}

	fmt.Printf("Database scan finished.\n")
	fmt.Printf("- database: %s\n", snapshot.Database)
	fmt.Printf("- tables: %d\n", len(snapshot.Tables))
	fmt.Printf("- columns: %d\n", totalColumns)
	fmt.Printf("- foreign_keys: %d\n", totalForeignKeys)
	fmt.Printf("- indexes: %d\n", totalIndexes)
	fmt.Printf("- output: %s\n", outputPath)
}

func maybeSendEmail(attachmentPath, databaseName, recipient string) error {
	settings := readMailSettings(recipient, fmt.Sprintf("DB Schema Scan - %s", databaseName), "Adjunto envio snapshot del schema para revision.")
	if !settings.isConfigured() {
		fmt.Println("- email: skipped (MAIL_* or DBSCAN_* SMTP variables are required)")
		return nil
	}

	message, receivers, err := buildEmailWithAttachment(settings, attachmentPath)
	if err != nil {
		return err
	}

	address := fmt.Sprintf("%s:%s", settings.smtpHost, settings.smtpPort)
	auth := smtp.PlainAuth("", settings.smtpUser, settings.smtpPass, settings.smtpHost)
	if err = smtp.SendMail(address, auth, settings.fromEmail, receivers, message); err != nil {
		return err
	}

	fmt.Printf("- email: sent to %s\n", strings.Join(receivers, ", "))
	return nil
}

func maybeSendFailureEmail(stage string, sourceErr error, recipient string) error {
	settings := readMailSettings(
		recipient,
		fmt.Sprintf("DBSCAN ERROR - %s", stage),
		fmt.Sprintf("El proceso dbscan fallo.\nEtapa: %s\nError: %v\nFecha: %s\n", stage, sourceErr, time.Now().Format(time.RFC3339)),
	)
	if !settings.isConfigured() {
		return nil
	}

	headers := strings.Builder{}
	headers.WriteString(fmt.Sprintf("From: %s\r\n", settings.fromEmail))
	headers.WriteString(fmt.Sprintf("To: %s\r\n", settings.toEmail))
	if settings.ccEmail != "" {
		headers.WriteString(fmt.Sprintf("Cc: %s\r\n", settings.ccEmail))
	}

	message := []byte(
		headers.String() +
			fmt.Sprintf("Subject: %s\r\n", settings.subject) +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n" +
			settings.body,
	)

	address := fmt.Sprintf("%s:%s", settings.smtpHost, settings.smtpPort)
	auth := smtp.PlainAuth("", settings.smtpUser, settings.smtpPass, settings.smtpHost)
	return smtp.SendMail(address, auth, settings.fromEmail, settings.receivers(), message)
}

func buildEmailWithAttachment(settings mailSettings, attachmentPath string) ([]byte, []string, error) {
	content, err := os.ReadFile(attachmentPath)
	if err != nil {
		return nil, nil, err
	}

	filename := filepath.Base(attachmentPath)
	boundary := "dbscan-boundary-20260429"

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("From: %s\r\n", settings.fromEmail))
	buffer.WriteString(fmt.Sprintf("To: %s\r\n", settings.toEmail))
	if settings.ccEmail != "" {
		buffer.WriteString(fmt.Sprintf("Cc: %s\r\n", settings.ccEmail))
	}
	buffer.WriteString(fmt.Sprintf("Subject: %s\r\n", settings.subject))
	buffer.WriteString("MIME-Version: 1.0\r\n")
	buffer.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%q\r\n", boundary))
	buffer.WriteString("\r\n")

	buffer.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buffer.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	buffer.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	buffer.WriteString("\r\n")
	buffer.WriteString(settings.body + "\r\n")
	buffer.WriteString("\r\n")

	buffer.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buffer.WriteString("Content-Type: application/json; name=\"" + filename + "\"\r\n")
	buffer.WriteString("Content-Transfer-Encoding: base64\r\n")
	buffer.WriteString("Content-Disposition: attachment; filename=\"" + filename + "\"\r\n")
	buffer.WriteString("\r\n")

	encoded := base64.StdEncoding.EncodeToString(content)
	for start := 0; start < len(encoded); start += 76 {
		end := start + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		buffer.WriteString(encoded[start:end] + "\r\n")
	}

	buffer.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	return buffer.Bytes(), settings.receivers(), nil
}

func readEnvWithDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func readMailSettings(defaultTo, defaultSubject, defaultBody string) mailSettings {
	to := firstNonEmptyEnv("MAIL_EMAIL_TO", "DBSCAN_EMAIL_TO")
	if to == "" {
		to = defaultTo
	}

	subject := firstNonEmptyEnv("MAIL_EMAIL_SUBJECT")
	if subject == "" {
		subject = defaultSubject
	}

	body := firstNonEmptyEnv("MAIL_EMAIL_BODY")
	if body == "" {
		body = defaultBody
	}

	return mailSettings{
		smtpHost:  firstNonEmptyEnv("MAIL_SMTP_HOST", "DBSCAN_SMTP_HOST"),
		smtpPort:  firstNonEmptyEnv("MAIL_SMTP_PORT", "DBSCAN_SMTP_PORT"),
		smtpUser:  firstNonEmptyEnv("MAIL_SMTP_USER", "DBSCAN_SMTP_USER"),
		smtpPass:  firstNonEmptyEnv("MAIL_SMTP_PASS", "DBSCAN_SMTP_PASS"),
		fromEmail: firstNonEmptyEnv("MAIL_EMAIL_FROM", "DBSCAN_EMAIL_FROM"),
		toEmail:   to,
		ccEmail:   firstNonEmptyEnv("MAIL_EMAIL_CC"),
		bccEmail:  firstNonEmptyEnv("MAIL_EMAIL_BCC"),
		subject:   subject,
		body:      body,
	}
}

func (m mailSettings) isConfigured() bool {
	return m.smtpHost != "" && m.smtpPort != "" && m.smtpUser != "" && m.smtpPass != "" && m.fromEmail != "" && m.toEmail != ""
}

func (m mailSettings) receivers() []string {
	receivers := []string{m.toEmail}
	if m.ccEmail != "" {
		receivers = append(receivers, m.ccEmail)
	}
	if m.bccEmail != "" {
		receivers = append(receivers, m.bccEmail)
	}
	return receivers
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}
