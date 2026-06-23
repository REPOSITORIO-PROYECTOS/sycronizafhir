package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/monitor"
	"sycronizafhir/internal/supabase"
)

var imageSyncCycleMu sync.Mutex

const (
	imageSyncStateKey       = "image_sync_last_run_utc"
	imageSyncStatusStateKey = "image_sync_last_status"
	imageUploadDirection    = "image_upload"
	imageCacheKeyPrefix     = "img:"
)

type ImageSyncStats struct {
	Uploaded       int                     `json:"uploaded"`
	Skipped        int                     `json:"skipped"`
	Failed         int                     `json:"failed"`
	StartedAt      time.Time               `json:"started_at"`
	FinishedAt     time.Time               `json:"finished_at"`
	Message        string                  `json:"message"`
	ErrorSummaries []ImageSyncErrorSummary `json:"error_summaries,omitempty"`
}

type PendingProductImage struct {
	ProdID       string `json:"prod_id"`
	ProdImagen   string `json:"prod_imagen"`
	FileStatus   string `json:"file_status"`
	ResolvedPath string `json:"resolved_path,omitempty"`
}

type PendingProductImagesSummary struct {
	Total        int64                 `json:"total"`
	Ready        int                   `json:"ready"`
	Missing      int                   `json:"missing"`
	Invalid      int                   `json:"invalid"`
	QueuedRetry  int64                 `json:"queued_retry"`
	LocalBase    string                `json:"local_base"`
	PreviewLimit int                   `json:"preview_limit"`
	Items        []PendingProductImage `json:"items"`
}

type imageCacheEntry struct {
	Fingerprint string `json:"fingerprint"`
	URL         string `json:"url"`
}

type ImageResolver struct {
	storage   *supabase.StorageClient
	queue     *db.QueueSQLite
	bucket    string
	localBase string
	enabled   bool
	runtime   *monitor.Runtime
}

func NewImageResolver(cfg config.Config, queue *db.QueueSQLite, runtime *monitor.Runtime) *ImageResolver {
	resolver := &ImageResolver{
		queue:     queue,
		bucket:    cfg.StorageBucketProductos,
		localBase: cfg.ImageLocalBasePath,
		enabled:   cfg.ImageSyncEnabled,
		runtime:   runtime,
	}
	if cfg.ImageSyncEnabled {
		resolver.storage = supabase.NewStorageClient(cfg.SupabaseURL, cfg.SupabaseServiceRole)
	}
	return resolver
}

func (r *ImageResolver) Enabled() bool {
	return r != nil && r.enabled && r.storage != nil
}

func (r *ImageResolver) ResolveProductRows(ctx context.Context, rows []map[string]interface{}) []map[string]interface{} {
	if !r.Enabled() || len(rows) == 0 {
		return rows
	}

	resolved := make([]map[string]interface{}, len(rows))
	for index, row := range rows {
		clone := cloneRowMap(row)
		_ = r.resolveProductRow(ctx, clone)
		resolved[index] = clone
	}
	return resolved
}

func (r *ImageResolver) resolveProductRow(ctx context.Context, row map[string]interface{}) error {
	rawImage, ok := row["prod_imagen"]
	if !ok || rawImage == nil {
		return nil
	}

	imagePath := strings.TrimSpace(fmt.Sprint(rawImage))
	if imagePath == "" || isRemoteImageURL(imagePath) {
		return nil
	}

	prodID := strings.TrimSpace(fmt.Sprint(row["prod_id"]))
	if prodID == "" {
		return fmt.Errorf("prod_id vacio para imagen %s", imagePath)
	}

	publicURL, err := r.uploadLocalImage(ctx, prodID, imagePath)
	if err != nil {
		return err
	}
	row["prod_imagen"] = publicURL
	return nil
}

func (r *ImageResolver) uploadLocalImage(ctx context.Context, prodID, imagePath string) (string, error) {
	localPath, err := resolveLocalImagePath(r.localBase, imagePath)
	if err != nil {
		return "", err
	}

	info, statErr := os.Stat(localPath)
	if statErr != nil {
		return "", fmt.Errorf("archivo no encontrado %s: %w", localPath, statErr)
	}
	if info.IsDir() {
		return "", fmt.Errorf("ruta de imagen es directorio: %s", localPath)
	}

	content, readErr := os.ReadFile(localPath)
	if readErr != nil {
		return "", fmt.Errorf("leer imagen %s: %w", localPath, readErr)
	}

	fingerprint := fileFingerprint(content, info.ModTime(), info.Size())
	cacheKey := imageCacheKeyPrefix + strings.ToLower(localPath)
	if cachedURL, ok := r.loadCachedURL(ctx, cacheKey, fingerprint); ok {
		return cachedURL, nil
	}

	objectPath := buildStorageObjectPath(prodID, localPath)
	contentType := supabase.ContentTypeFromExtension(localPath)
	if uploadErr := r.storage.UploadObject(ctx, r.bucket, objectPath, contentType, content); uploadErr != nil {
		return "", uploadErr
	}

	publicURL := r.storage.PublicURL(r.bucket, objectPath)
	if saveErr := r.saveCachedURL(ctx, cacheKey, fingerprint, publicURL); saveErr != nil && r.runtime != nil {
		r.runtime.AddLog(fmt.Sprintf("image_sync: cache warning %s: %v", cacheKey, saveErr))
	}
	return publicURL, nil
}

func (r *ImageResolver) loadCachedURL(ctx context.Context, cacheKey, fingerprint string) (string, bool) {
	if r.queue == nil {
		return "", false
	}
	raw, exists, err := r.queue.GetStateValue(ctx, cacheKey)
	if err != nil || !exists {
		return "", false
	}
	var entry imageCacheEntry
	if err = json.Unmarshal([]byte(raw), &entry); err != nil {
		return "", false
	}
	if entry.Fingerprint != fingerprint || strings.TrimSpace(entry.URL) == "" {
		return "", false
	}
	return entry.URL, true
}

func (r *ImageResolver) saveCachedURL(ctx context.Context, cacheKey, fingerprint, url string) error {
	if r.queue == nil {
		return nil
	}
	payload, err := json.Marshal(imageCacheEntry{
		Fingerprint: fingerprint,
		URL:         url,
	})
	if err != nil {
		return err
	}
	return r.queue.SetStateValue(ctx, cacheKey, string(payload))
}

type ImageSyncWorker struct {
	localPG      *db.LocalPG
	remotePG     *supabase.PGClient
	queue        *db.QueueSQLite
	resolver     *ImageResolver
	sourceSchema string
	remoteTable  string
	pollInterval time.Duration
	runtime      *monitor.Runtime

	mu          sync.Mutex
	lastStats   ImageSyncStats
	runningOnce bool
}

type queuedImagePayload struct {
	ProdID     string `json:"prod_id"`
	ProdImagen string `json:"prod_imagen"`
}

func NewImageSyncWorker(
	localPG *db.LocalPG,
	remotePG *supabase.PGClient,
	queue *db.QueueSQLite,
	resolver *ImageResolver,
	cfg config.Config,
	runtime *monitor.Runtime,
) *ImageSyncWorker {
	syncCfg, _ := config.LoadSyncTablesConfig()
	return &ImageSyncWorker{
		localPG:      localPG,
		remotePG:     remotePG,
		queue:        queue,
		resolver:     resolver,
		sourceSchema: cfg.SourceSchema,
		remoteTable:  syncCfg.ResolveRemoteTable("productos"),
		pollInterval: cfg.ImageSyncInterval,
		runtime:      runtime,
	}
}

func (w *ImageSyncWorker) Run(ctx context.Context) {
	if w.resolver == nil || !w.resolver.Enabled() {
		if w.runtime != nil {
			w.runtime.SetComponentStatus("image_sync", "stopped", "image sync deshabilitado")
		}
		return
	}

	if err := w.loadPersistedStatus(ctx); err != nil {
		log.Printf("load image sync status failed: %v", err)
	}
	w.runtime.SetComponentStatus("image_sync", "running", "worker iniciado")

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	if err := w.runCycle(ctx, false); err != nil {
		log.Printf("image sync initial cycle failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.runCycle(ctx, false); err != nil {
				log.Printf("image sync cycle failed: %v", err)
				w.runtime.SetComponentStatus("image_sync", "error", err.Error())
			} else {
				w.runtime.SetComponentStatus("image_sync", "running", "ciclo OK")
			}
		}
	}
}

func (w *ImageSyncWorker) RunOnce(ctx context.Context, force bool) (ImageSyncStats, error) {
	if w.resolver == nil || !w.resolver.Enabled() {
		return ImageSyncStats{}, fmt.Errorf("image sync deshabilitado")
	}

	w.mu.Lock()
	if w.runningOnce {
		w.mu.Unlock()
		return w.lastStats, fmt.Errorf("image sync ya en curso")
	}
	w.runningOnce = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.runningOnce = false
		w.mu.Unlock()
	}()

	if err := w.runCycle(ctx, force); err != nil {
		return w.GetStatus(), err
	}
	return w.GetStatus(), nil
}

func (w *ImageSyncWorker) GetStatus() ImageSyncStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastStats
}

func (w *ImageSyncWorker) runCycle(ctx context.Context, force bool) error {
	imageSyncCycleMu.Lock()
	defer imageSyncCycleMu.Unlock()

	retryFailures := w.retryQueuedUploads(ctx)

	stats := ImageSyncStats{
		StartedAt: time.Now().UTC(),
	}
	failures := newImageSyncFailureCollector()
	defer func() {
		stats.FinishedAt = time.Now().UTC()
		stats.ErrorSummaries = failures.ToSummaries()
		stats.Message = formatImageSyncStatsMessage(stats.Uploaded, stats.Skipped, stats.Failed, stats.ErrorSummaries)
		failures.LogCycleSummary(w.runtime, stats.Uploaded, stats.Skipped, stats.Failed)
		w.mu.Lock()
		w.lastStats = stats
		w.mu.Unlock()
		_ = w.persistStatus(ctx, stats)
	}()

	if retryFailures != nil && retryFailures.TotalFailed() > 0 {
		log.Printf("retry queued image uploads completed with errors: %s", retryFailures.ShortSummary())
		if w.runtime != nil {
			w.runtime.AddLog(fmt.Sprintf("image_sync: reintentos pendientes — %s", retryFailures.ShortSummary()))
		}
	}

	since := time.Time{}
	if !force {
		if loaded, err := w.loadCheckpoint(ctx); err == nil {
			since = loaded
		}
	}

	const batchSize = 100
	offset := 0

	for {
		candidates, err := w.localPG.LoadProductImageCandidates(ctx, w.sourceSchema, since, batchSize, offset)
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			break
		}

		for _, candidate := range candidates {
			row := map[string]interface{}{
				"prod_id":     candidate.ProdID,
				"prod_imagen": candidate.ProdImagen,
			}
			if candidate.FechaModificacion.IsZero() {
				row["fecha_modificacion"] = time.Now().UTC()
			} else {
				row["fecha_modificacion"] = candidate.FechaModificacion
			}

			if resolveErr := w.resolver.resolveProductRow(ctx, row); resolveErr != nil {
				if isSkippableImageSyncError(resolveErr) {
					stats.Skipped++
					continue
				}
				stats.Failed++
				failures.RecordFailure(candidate.ProdID, resolveErr)
				payload, marshalErr := json.Marshal(queuedImagePayload{
					ProdID:     candidate.ProdID,
					ProdImagen: candidate.ProdImagen,
				})
				if marshalErr == nil {
					_ = w.queue.Enqueue(ctx, imageUploadDirection, string(payload))
				}
				continue
			}

			publicURL, ok := row["prod_imagen"].(string)
			if !ok || publicURL == "" || !isRemoteImageURL(publicURL) {
				stats.Skipped++
				continue
			}

			if upsertErr := w.remotePG.UpsertRows(ctx, "public", w.remoteTable, []map[string]interface{}{row}, []string{"prod_id"}); upsertErr != nil {
				stats.Failed++
				failures.RecordFailure(candidate.ProdID, upsertErr)
				continue
			}

			stats.Uploaded++
		}

		offset += len(candidates)
		if len(candidates) < batchSize {
			break
		}
	}

	if stats.Failed == 0 {
		now := time.Now().UTC()
		if err := w.persistCheckpoint(ctx, now); err != nil {
			log.Printf("persist image sync checkpoint failed: %v", err)
		}
	}

	if stats.Failed > 0 {
		return fmt.Errorf("image sync: %d fallo(s) — %s", stats.Failed, failures.ShortSummary())
	}
	return nil
}

func (w *ImageSyncWorker) retryQueuedUploads(ctx context.Context) *imageSyncFailureCollector {
	failures := newImageSyncFailureCollector()
	jobs, err := w.queue.PeekByDirection(ctx, imageUploadDirection, 50)
	if err != nil {
		log.Printf("retry queued image uploads failed: %v", err)
		if w.runtime != nil {
			w.runtime.AddLog(fmt.Sprintf("image_sync: cola de reintentos: %v", err))
		}
		return failures
	}

	retryOK := 0
	for _, job := range jobs {
		var payload queuedImagePayload
		if err = json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
			_ = w.queue.Delete(ctx, job.ID)
			continue
		}

		row := map[string]interface{}{
			"prod_id":     payload.ProdID,
			"prod_imagen": payload.ProdImagen,
			"fecha_modificacion": time.Now().UTC(),
		}
		if resolveErr := w.resolver.resolveProductRow(ctx, row); resolveErr != nil {
			if isSkippableImageSyncError(resolveErr) {
				_ = w.queue.Delete(ctx, job.ID)
				continue
			}
			failures.RecordFailure(payload.ProdID, resolveErr)
			continue
		}

		publicURL, ok := row["prod_imagen"].(string)
		if !ok || !isRemoteImageURL(publicURL) {
			_ = w.queue.Delete(ctx, job.ID)
			continue
		}

		if upsertErr := w.remotePG.UpsertRows(ctx, "public", w.remoteTable, []map[string]interface{}{row}, []string{"prod_id"}); upsertErr != nil {
			failures.RecordFailure(payload.ProdID, upsertErr)
			continue
		}

		_ = w.queue.Delete(ctx, job.ID)
		retryOK++
	}

	if retryOK > 0 && w.runtime != nil {
		w.runtime.AddLog(fmt.Sprintf("image_sync: reintentos OK=%d", retryOK))
	}
	return failures
}

func (w *ImageSyncWorker) loadCheckpoint(ctx context.Context) (time.Time, error) {
	rawValue, exists, err := w.queue.GetStateValue(ctx, imageSyncStateKey)
	if err != nil {
		return time.Time{}, err
	}
	if !exists {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, rawValue)
}

func (w *ImageSyncWorker) persistCheckpoint(ctx context.Context, value time.Time) error {
	return w.queue.SetStateValue(ctx, imageSyncStateKey, value.Format(time.RFC3339Nano))
}

func (w *ImageSyncWorker) persistStatus(ctx context.Context, stats ImageSyncStats) error {
	raw, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	return w.queue.SetStateValue(ctx, imageSyncStatusStateKey, string(raw))
}

func (w *ImageSyncWorker) loadPersistedStatus(ctx context.Context) error {
	raw, exists, err := w.queue.GetStateValue(ctx, imageSyncStatusStateKey)
	if err != nil || !exists {
		return err
	}
	var stats ImageSyncStats
	if err = json.Unmarshal([]byte(raw), &stats); err != nil {
		return err
	}
	w.mu.Lock()
	w.lastStats = stats
	w.mu.Unlock()
	return nil
}

func PreviewPendingProductImages(
	ctx context.Context,
	localPG *db.LocalPG,
	queue *db.QueueSQLite,
	schemaName, localBase string,
	previewLimit int,
) (PendingProductImagesSummary, error) {
	summary := PendingProductImagesSummary{
		LocalBase:    strings.TrimSpace(localBase),
		PreviewLimit: previewLimit,
		Items:        []PendingProductImage{},
	}
	if localPG == nil {
		return summary, fmt.Errorf("conexion local no disponible")
	}
	if previewLimit <= 0 {
		previewLimit = 50
	}
	summary.PreviewLimit = previewLimit

	total, err := localPG.CountProductImageCandidates(ctx, schemaName, time.Time{})
	if err != nil {
		return summary, err
	}
	summary.Total = total

	if queue != nil {
		queued, countErr := queue.CountByDirection(ctx, imageUploadDirection)
		if countErr == nil {
			summary.QueuedRetry = queued
		}
	}

	candidates, err := localPG.LoadProductImageCandidates(ctx, schemaName, time.Time{}, previewLimit, 0)
	if err != nil {
		return summary, err
	}

	for _, candidate := range candidates {
		item := PendingProductImage{
			ProdID:     candidate.ProdID,
			ProdImagen: candidate.ProdImagen,
		}
		status, resolvedPath := classifyPendingImageFile(localBase, candidate.ProdImagen)
		item.FileStatus = status
		item.ResolvedPath = resolvedPath
		switch status {
		case "ready":
			summary.Ready++
		case "missing":
			summary.Missing++
		default:
			summary.Invalid++
		}
		summary.Items = append(summary.Items, item)
	}

	return summary, nil
}

func classifyPendingImageFile(localBase, imagePath string) (status, resolvedPath string) {
	resolvedPath, err := resolveLocalImagePath(localBase, imagePath)
	if err != nil {
		if strings.Contains(err.Error(), "no encontrado") {
			return "missing", ""
		}
		return "invalid", ""
	}
	return "ready", resolvedPath
}

func LoadImageSyncStatus(ctx context.Context, queue *db.QueueSQLite) (ImageSyncStats, bool, error) {
	if queue == nil {
		return ImageSyncStats{}, false, nil
	}
	raw, exists, err := queue.GetStateValue(ctx, imageSyncStatusStateKey)
	if err != nil || !exists {
		return ImageSyncStats{}, exists, err
	}
	var stats ImageSyncStats
	if err = json.Unmarshal([]byte(raw), &stats); err != nil {
		return ImageSyncStats{}, false, err
	}
	return stats, true, nil
}

func isRemoteImageURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func isLocalImagePath(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || isRemoteImageURL(trimmed) {
		return false
	}
	return !strings.Contains(trimmed, "://")
}

func resolveLocalImagePath(basePath, imagePath string) (string, error) {
	trimmed := strings.TrimSpace(imagePath)
	if trimmed == "" {
		return "", fmt.Errorf("ruta de imagen vacia")
	}
	if !isLocalImagePath(trimmed) {
		return "", fmt.Errorf("ruta no local: %s", trimmed)
	}

	candidate := normalizeLocalImagePath(trimmed)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	if filepath.IsAbs(candidate) {
		return "", fmt.Errorf("archivo no encontrado: %s", candidate)
	}

	base := strings.TrimSpace(basePath)
	if base == "" {
		return "", fmt.Errorf("archivo no encontrado: %s", candidate)
	}

	normalizedBase := normalizeLocalImagePath(base)
	relative := strings.TrimLeft(strings.ReplaceAll(trimmed, `\`, "/"), "/")
	if normalizedBase != "" {
		baseSlash := strings.TrimLeft(strings.ReplaceAll(normalizedBase, `\`, "/"), "/")
		if strings.HasPrefix(strings.ToLower(relative), strings.ToLower(baseSlash)+"/") {
			relative = strings.TrimPrefix(relative, baseSlash)
			relative = strings.TrimPrefix(relative, "/")
		}
	}

	joined := filepath.Clean(filepath.Join(normalizedBase, filepath.FromSlash(relative)))
	if _, err := os.Stat(joined); err != nil {
		return "", fmt.Errorf("archivo no encontrado: %s", joined)
	}
	return joined, nil
}

func normalizeLocalImagePath(value string) string {
	slashNormalized := strings.ReplaceAll(strings.TrimSpace(value), `\`, "/")
	return filepath.Clean(filepath.FromSlash(slashNormalized))
}

func buildStorageObjectPath(prodID, filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		ext = ".jpg"
	}
	safeProdID := strings.TrimSpace(prodID)
	return safeProdID + ext
}

func fileFingerprint(content []byte, modTime time.Time, size int64) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]) + ":" + modTime.UTC().Format(time.RFC3339Nano) + ":" + fmt.Sprintf("%d", size)
}

func cloneRowMap(row map[string]interface{}) map[string]interface{} {
	clone := make(map[string]interface{}, len(row))
	for key, value := range row {
		clone[key] = value
	}
	return clone
}
