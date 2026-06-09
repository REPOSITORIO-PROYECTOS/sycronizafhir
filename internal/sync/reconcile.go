package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
)

const dataAuditStateKey = "data_audit_last_report"

type TableAuditResult struct {
	LocalTable      string `json:"local_table"`
	RemoteTable     string `json:"remote_table"`
	LocalCount      int64  `json:"local_count"`
	RemoteCount     int64  `json:"remote_count"`
	MissingInRemote int64  `json:"missing_in_remote"`
	Changed         int64  `json:"changed"`
	InSync          int64  `json:"in_sync"`
	Status          string `json:"status"`
	Error           string `json:"error,omitempty"`
	Selected        bool   `json:"selected"`
}

type DataAuditReport struct {
	AuditedAt       time.Time          `json:"audited_at"`
	Trigger         string             `json:"trigger"`
	Tables          []TableAuditResult `json:"tables"`
	Summary         string             `json:"summary"`
	AutoSyncApplied bool               `json:"auto_sync_applied"`
	SyncedRows      int                `json:"synced_rows"`
}

type ReconcileService struct {
	localPG      *db.LocalPG
	remotePG     *supabase.PGClient
	sourceSchema string
	exclude      []string
	runtime      *monitor.Runtime
}

func NewReconcileService(
	localPG *db.LocalPG,
	remotePG *supabase.PGClient,
	sourceSchema string,
	exclude []string,
	runtime *monitor.Runtime,
) *ReconcileService {
	return &ReconcileService{
		localPG:      localPG,
		remotePG:     remotePG,
		sourceSchema: sourceSchema,
		exclude:      exclude,
		runtime:      runtime,
	}
}

func (s *ReconcileService) ListAvailableTables(ctx context.Context) ([]db.SyncTable, error) {
	return s.localPG.ListSyncTables(ctx, s.sourceSchema, s.exclude)
}

func (s *ReconcileService) RunAudit(
	ctx context.Context,
	syncCfg config.SyncTablesConfig,
	trigger string,
	applySync bool,
) (DataAuditReport, error) {
	tables, err := s.ListAvailableTables(ctx)
	if err != nil {
		return DataAuditReport{}, err
	}

	report := DataAuditReport{
		AuditedAt: time.Now().UTC(),
		Trigger:   trigger,
		Tables:    make([]TableAuditResult, 0, len(tables)),
	}

	totalMissing := int64(0)
	totalChanged := int64(0)
	for _, table := range tables {
		selected := syncCfg.IsEnabled(table.Name)
		result := TableAuditResult{
			LocalTable:  table.Name,
			RemoteTable: syncCfg.ResolveRemoteTable(table.Name),
			Selected:    selected,
			Status:      "skipped",
		}

		if !selected {
			report.Tables = append(report.Tables, result)
			continue
		}

		auditResult, auditErr := s.auditTable(ctx, table, syncCfg.ResolveRemoteTable(table.Name))
		result = auditResult
		result.Selected = true
		if auditErr != nil {
			result.Status = "error"
			result.Error = auditErr.Error()
			report.Tables = append(report.Tables, result)
			continue
		}

		totalMissing += result.MissingInRemote
		totalChanged += result.Changed
		report.Tables = append(report.Tables, result)

		shouldApplySync := applySync && (trigger == "manual" || syncCfg.AutoSyncOnAudit)
		if shouldApplySync && (result.MissingInRemote > 0 || result.Changed > 0) {
			synced, syncErr := s.SyncTableDiff(ctx, table, syncCfg.ResolveRemoteTable(table.Name))
			if syncErr != nil {
				result.Status = "error"
				result.Error = syncErr.Error()
				report.Tables[len(report.Tables)-1] = result
				continue
			}
			report.SyncedRows += synced
			report.AutoSyncApplied = true
		}
	}

	report.Summary = fmt.Sprintf(
		"Auditoria: %d tablas, %d faltantes, %d cambiadas",
		len(report.Tables),
		totalMissing,
		totalChanged,
	)
	if report.AutoSyncApplied {
		report.Summary += fmt.Sprintf(", %d filas sincronizadas", report.SyncedRows)
	}

	if s.runtime != nil {
		s.runtime.AddLog(fmt.Sprintf("auditoria (%s): %s", trigger, report.Summary))
	}

	return report, nil
}

func (s *ReconcileService) auditTable(
	ctx context.Context,
	table db.SyncTable,
	remoteTable string,
) (TableAuditResult, error) {
	result := TableAuditResult{
		LocalTable:  table.Name,
		RemoteTable: remoteTable,
		Status:      "ok",
	}

	exists, err := s.remotePG.TableExists(ctx, "public", remoteTable)
	if err != nil {
		return result, err
	}
	if !exists {
		result.Status = "error"
		result.Error = fmt.Sprintf("tabla remota public.%s no existe", remoteTable)
		return result, nil
	}

	result.LocalCount, err = s.localPG.CountTableRows(ctx, s.sourceSchema, table.Name)
	if err != nil {
		return result, err
	}
	result.RemoteCount, err = s.remotePG.CountTableRows(ctx, "public", remoteTable)
	if err != nil {
		return result, err
	}

	localPKRows, err := s.localPG.LoadPrimaryKeyRows(ctx, s.sourceSchema, table.Name, table.PrimaryKeys)
	if err != nil {
		return result, err
	}
	remotePKRows, err := s.remotePG.LoadPrimaryKeyRows(ctx, "public", remoteTable, table.PrimaryKeys)
	if err != nil {
		return result, err
	}

	localKeys := make(map[string]map[string]interface{}, len(localPKRows))
	for _, row := range localPKRows {
		key, keyErr := PKKey(row, table.PrimaryKeys)
		if keyErr != nil {
			continue
		}
		localKeys[key] = row
	}

	remoteKeys := make(map[string]map[string]interface{}, len(remotePKRows))
	for _, row := range remotePKRows {
		key, keyErr := PKKey(row, table.PrimaryKeys)
		if keyErr != nil {
			continue
		}
		remoteKeys[key] = row
	}

	missingPKRows := make([]map[string]interface{}, 0)
	commonPKRows := make([]map[string]interface{}, 0)
	for key, row := range localKeys {
		if _, exists := remoteKeys[key]; exists {
			commonPKRows = append(commonPKRows, row)
			continue
		}
		missingPKRows = append(missingPKRows, row)
	}
	result.MissingInRemote = int64(len(missingPKRows))

	remoteColumns, err := s.remotePG.ReadTableColumns(ctx, "public", remoteTable)
	if err != nil {
		return result, err
	}

	changedCount := int64(0)
	for start := 0; start < len(commonPKRows); start += 200 {
		end := start + 200
		if end > len(commonPKRows) {
			end = len(commonPKRows)
		}
		batch := commonPKRows[start:end]

		localRows, loadErr := s.localPG.LoadRowsByPrimaryKeys(ctx, s.sourceSchema, table.Name, table.PrimaryKeys, batch)
		if loadErr != nil {
			return result, loadErr
		}
		remoteRows, loadErr := s.remotePG.LoadRowsByPrimaryKeys(ctx, "public", remoteTable, table.PrimaryKeys, batch)
		if loadErr != nil {
			return result, loadErr
		}

		localByKey := mapRowsByPK(localRows, table.PrimaryKeys)
		remoteByKey := mapRowsByPK(remoteRows, table.PrimaryKeys)

		for key, localRow := range localByKey {
			remoteRow, ok := remoteByKey[key]
			if !ok {
				continue
			}
			hashColumns := CommonColumns(localRow, remoteColumns)
			localHash, hashErr := RowHash(localRow, hashColumns)
			if hashErr != nil {
				return result, hashErr
			}
			remoteHash, hashErr := RowHash(remoteRow, hashColumns)
			if hashErr != nil {
				return result, hashErr
			}
			if localHash != remoteHash {
				changedCount++
			}
		}
	}

	result.Changed = changedCount
	result.InSync = int64(len(commonPKRows)) - changedCount
	if result.MissingInRemote > 0 || result.Changed > 0 {
		result.Status = "diff"
	}

	return result, nil
}

func (s *ReconcileService) SyncSelectedTables(
	ctx context.Context,
	syncCfg config.SyncTablesConfig,
	tableNames []string,
) (int, error) {
	tables, err := s.ListAvailableTables(ctx)
	if err != nil {
		return 0, err
	}

	allowed := map[string]db.SyncTable{}
	for _, table := range tables {
		allowed[table.Name] = table
	}

	totalSynced := 0
	for _, tableName := range tableNames {
		table, ok := allowed[tableName]
		if !ok {
			return totalSynced, fmt.Errorf("tabla no disponible para sync: %s", tableName)
		}
		if !syncCfg.IsEnabled(tableName) {
			return totalSynced, fmt.Errorf("tabla no habilitada: %s", tableName)
		}

		synced, syncErr := s.SyncTableDiff(ctx, table, syncCfg.ResolveRemoteTable(table.Name))
		if syncErr != nil {
			return totalSynced, syncErr
		}
		totalSynced += synced
	}

	return totalSynced, nil
}

func (s *ReconcileService) SyncTableDiff(
	ctx context.Context,
	table db.SyncTable,
	remoteTable string,
) (int, error) {
	audit, err := s.auditTable(ctx, table, remoteTable)
	if err != nil {
		return 0, err
	}
	if audit.MissingInRemote == 0 && audit.Changed == 0 {
		return 0, nil
	}

	localPKRows, err := s.localPG.LoadPrimaryKeyRows(ctx, s.sourceSchema, table.Name, table.PrimaryKeys)
	if err != nil {
		return 0, err
	}
	remotePKRows, err := s.remotePG.LoadPrimaryKeyRows(ctx, "public", remoteTable, table.PrimaryKeys)
	if err != nil {
		return 0, err
	}

	localKeys := mapRowsByPK(localPKRows, table.PrimaryKeys)
	remoteKeys := mapRowsByPK(remotePKRows, table.PrimaryKeys)

	pkRowsToSync := make([]map[string]interface{}, 0)
	for key, row := range localKeys {
		if _, exists := remoteKeys[key]; !exists {
			pkRowsToSync = append(pkRowsToSync, row)
		}
	}

	remoteColumns, err := s.remotePG.ReadTableColumns(ctx, "public", remoteTable)
	if err != nil {
		return 0, err
	}

	commonPKRows := make([]map[string]interface{}, 0)
	for key, row := range localKeys {
		if _, exists := remoteKeys[key]; exists {
			commonPKRows = append(commonPKRows, row)
		}
	}

	for start := 0; start < len(commonPKRows); start += 200 {
		end := start + 200
		if end > len(commonPKRows) {
			end = len(commonPKRows)
		}
		batch := commonPKRows[start:end]

		localRows, loadErr := s.localPG.LoadRowsByPrimaryKeys(ctx, s.sourceSchema, table.Name, table.PrimaryKeys, batch)
		if loadErr != nil {
			return 0, loadErr
		}
		remoteRows, loadErr := s.remotePG.LoadRowsByPrimaryKeys(ctx, "public", remoteTable, table.PrimaryKeys, batch)
		if loadErr != nil {
			return 0, loadErr
		}

		localByKey := mapRowsByPK(localRows, table.PrimaryKeys)
		remoteByKey := mapRowsByPK(remoteRows, table.PrimaryKeys)
		for key, localRow := range localByKey {
			remoteRow, ok := remoteByKey[key]
			if !ok {
				continue
			}
			hashColumns := CommonColumns(localRow, remoteColumns)
			localHash, hashErr := RowHash(localRow, hashColumns)
			if hashErr != nil {
				return 0, hashErr
			}
			remoteHash, hashErr := RowHash(remoteRow, hashColumns)
			if hashErr != nil {
				return 0, hashErr
			}
			if localHash != remoteHash {
				pkOnly := make(map[string]interface{}, len(table.PrimaryKeys))
				for _, column := range table.PrimaryKeys {
					pkOnly[column] = localRow[column]
				}
				pkRowsToSync = append(pkRowsToSync, pkOnly)
			}
		}
	}

	if len(pkRowsToSync) == 0 {
		return 0, nil
	}

	synced := 0
	for start := 0; start < len(pkRowsToSync); start += 200 {
		end := start + 200
		if end > len(pkRowsToSync) {
			end = len(pkRowsToSync)
		}
		batch := pkRowsToSync[start:end]
		rows, loadErr := s.localPG.LoadRowsByPrimaryKeys(ctx, s.sourceSchema, table.Name, table.PrimaryKeys, batch)
		if loadErr != nil {
			return synced, loadErr
		}
		if len(rows) == 0 {
			continue
		}
		if upsertErr := s.remotePG.UpsertRows(ctx, "public", remoteTable, rows, table.PrimaryKeys); upsertErr != nil {
			return synced, upsertErr
		}
		synced += len(rows)
		if s.runtime != nil {
			s.runtime.AddLog(fmt.Sprintf("sync diff: subidas %d filas a %s", len(rows), remoteTable))
		}
	}

	return synced, nil
}

func mapRowsByPK(rows []map[string]interface{}, pkColumns []string) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{}, len(rows))
	for _, row := range rows {
		key, err := PKKey(row, pkColumns)
		if err != nil {
			continue
		}
		result[key] = row
	}
	return result
}

func SaveAuditReport(ctx context.Context, queue *db.QueueSQLite, report DataAuditReport) error {
	if queue == nil {
		return nil
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	return queue.SetStateValue(ctx, dataAuditStateKey, string(raw))
}

func LoadAuditReport(ctx context.Context, queue *db.QueueSQLite) (DataAuditReport, bool, error) {
	if queue == nil {
		return DataAuditReport{}, false, nil
	}
	raw, exists, err := queue.GetStateValue(ctx, dataAuditStateKey)
	if err != nil || !exists {
		return DataAuditReport{}, false, err
	}
	var report DataAuditReport
	if err = json.Unmarshal([]byte(raw), &report); err != nil {
		return DataAuditReport{}, false, err
	}
	return report, true, nil
}
