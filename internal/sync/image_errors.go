package sync

import (
	"fmt"
	"strings"

	"sycronizafhir/internal/monitor"
)

const (
	imageSyncErrorSampleLimit = 5
	imageSyncDetailLogLimit   = 3
)

type imageSyncErrorCategory string

const (
	imageSyncErrRLSAuth      imageSyncErrorCategory = "rls_auth"
	imageSyncErrFileMissing  imageSyncErrorCategory = "file_missing"
	imageSyncErrPathInvalid  imageSyncErrorCategory = "path_invalid"
	imageSyncErrNetwork      imageSyncErrorCategory = "network"
	imageSyncErrUpsertRemote imageSyncErrorCategory = "upsert_remote"
	imageSyncErrOther        imageSyncErrorCategory = "other"
)

type ImageSyncErrorSummary struct {
	Category  string   `json:"category"`
	Message   string   `json:"message"`
	Count     int      `json:"count"`
	SampleIDs []string `json:"sample_ids,omitempty"`
}

type imageSyncFailureBucket struct {
	category    imageSyncErrorCategory
	message     string
	count       int
	sampleIDs   []string
	detailLogged bool
}

type imageSyncFailureCollector struct {
	buckets  map[string]*imageSyncFailureBucket
	uploaded int
	skipped  int
}

func newImageSyncFailureCollector() *imageSyncFailureCollector {
	return &imageSyncFailureCollector{
		buckets: make(map[string]*imageSyncFailureBucket),
	}
}

func (c *imageSyncFailureCollector) RecordUploadSuccess() {
	c.uploaded++
}

func (c *imageSyncFailureCollector) RecordSkipped() {
	c.skipped++
}

func (c *imageSyncFailureCollector) Merge(other *imageSyncFailureCollector) {
	if other == nil || len(other.buckets) == 0 {
		return
	}

	for key, source := range other.buckets {
		target, ok := c.buckets[key]
		if !ok {
			c.buckets[key] = &imageSyncFailureBucket{
				category:  source.category,
				message:   source.message,
				count:     source.count,
				sampleIDs: append([]string(nil), source.sampleIDs...),
			}
			continue
		}

		target.count += source.count
		for _, prodID := range source.sampleIDs {
			if len(target.sampleIDs) >= imageSyncErrorSampleLimit {
				break
			}
			target.sampleIDs = append(target.sampleIDs, prodID)
		}
	}
}

func (c *imageSyncFailureCollector) RecordFailure(prodID string, err error) {
	if err == nil {
		return
	}

	category := classifyImageSyncError(err)
	message := normalizeImageSyncError(err)
	key := string(category) + "\x00" + message

	bucket, ok := c.buckets[key]
	if !ok {
		bucket = &imageSyncFailureBucket{
			category: category,
			message:  message,
		}
		c.buckets[key] = bucket
	}

	bucket.count++
	prodID = strings.TrimSpace(prodID)
	if prodID != "" && len(bucket.sampleIDs) < imageSyncErrorSampleLimit {
		bucket.sampleIDs = append(bucket.sampleIDs, prodID)
	}
}

func (c *imageSyncFailureCollector) TotalFailed() int {
	total := 0
	for _, bucket := range c.buckets {
		total += bucket.count
	}
	return total
}

func (c *imageSyncFailureCollector) ToSummaries() []ImageSyncErrorSummary {
	if len(c.buckets) == 0 {
		return nil
	}

	summaries := make([]ImageSyncErrorSummary, 0, len(c.buckets))
	for _, bucket := range c.buckets {
		summaries = append(summaries, ImageSyncErrorSummary{
			Category:  string(bucket.category),
			Message:   bucket.message,
			Count:     bucket.count,
			SampleIDs: append([]string(nil), bucket.sampleIDs...),
		})
	}
	return summaries
}

func (c *imageSyncFailureCollector) ShortSummary() string {
	summaries := c.ToSummaries()
	if len(summaries) == 0 {
		return ""
	}

	parts := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		parts = append(parts, fmt.Sprintf("%s (%d): %s", summary.Category, summary.Count, summary.Message))
	}
	return strings.Join(parts, "; ")
}

func (c *imageSyncFailureCollector) LogCycleSummary(runtime *monitor.Runtime, uploaded, skipped, failed int) {
	if runtime == nil {
		return
	}

	detailLogs := 0
	for _, bucket := range c.buckets {
		if detailLogs >= imageSyncDetailLogLimit {
			break
		}
		if len(bucket.sampleIDs) == 0 {
			continue
		}
		runtime.AddLog(fmt.Sprintf(
			"image_sync: fallo [%s] prod_id=%s: %s",
			bucket.category,
			bucket.sampleIDs[0],
			bucket.message,
		))
		detailLogs++
		bucket.detailLogged = true
	}

	if failed > 0 {
		runtime.AddLog(fmt.Sprintf(
			"image_sync: resumen ciclo — subidas=%d omitidas=%d fallidas=%d; %s",
			uploaded,
			skipped,
			failed,
			c.ShortSummary(),
		))
		for _, summary := range c.ToSummaries() {
			if len(summary.SampleIDs) == 0 {
				continue
			}
			extra := summary.Count - len(summary.SampleIDs)
			if extra > 0 {
				runtime.AddLog(fmt.Sprintf(
					"image_sync: muestra %s (%d): %s (+%d más)",
					summary.Category,
					summary.Count,
					strings.Join(summary.SampleIDs, ", "),
					extra,
				))
			} else {
				runtime.AddLog(fmt.Sprintf(
					"image_sync: muestra %s (%d): %s",
					summary.Category,
					summary.Count,
					strings.Join(summary.SampleIDs, ", "),
				))
			}
		}
		return
	}

	if uploaded > 0 {
		runtime.AddLog(fmt.Sprintf(
			"image_sync: ciclo OK — subidas=%d omitidas=%d fallidas=0",
			uploaded,
			skipped,
		))
	}
}

func classifyImageSyncError(err error) imageSyncErrorCategory {
	if err == nil {
		return imageSyncErrOther
	}

	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "row-level security"),
		strings.Contains(lower, `"statuscode":"403"`),
		strings.Contains(lower, "unauthorized"),
		strings.Contains(lower, "service_role"):
		return imageSyncErrRLSAuth
	case strings.Contains(lower, "archivo no encontrado"),
		strings.Contains(lower, "no such file"),
		strings.Contains(lower, "not found"):
		return imageSyncErrFileMissing
	case strings.Contains(lower, "ruta de imagen"),
		strings.Contains(lower, "ruta no local"),
		strings.Contains(lower, "es directorio"),
		strings.Contains(lower, "prod_id vacio"):
		return imageSyncErrPathInvalid
	case strings.Contains(lower, "execute storage upload"),
		strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "timeout"),
		strings.Contains(lower, "temporary failure"),
		strings.Contains(lower, "network"):
		return imageSyncErrNetwork
	case strings.Contains(lower, "upsert"):
		return imageSyncErrUpsertRemote
	default:
		return imageSyncErrOther
	}
}

func normalizeImageSyncError(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "row-level security") || strings.Contains(lower, `"statuscode":"403"`):
		return "subida Storage rechazada por política RLS (revisar SUPABASE_SERVICE_ROLE_KEY)"
	case strings.Contains(lower, "unauthorized"):
		return "subida Storage no autorizada (clave API o permisos)"
	case strings.Contains(lower, "storage upload failed"):
		return "subida Storage falló"
	case strings.Contains(lower, "archivo no encontrado") || strings.Contains(lower, "no such file"):
		return "archivo de imagen no encontrado en disco"
	case strings.Contains(lower, "ruta de imagen vacia"):
		return "ruta de imagen vacía"
	case strings.Contains(lower, "ruta no local"):
		return "ruta de imagen no es local"
	case strings.Contains(lower, "es directorio"):
		return "ruta de imagen apunta a un directorio"
	case strings.Contains(lower, "prod_id vacio"):
		return "prod_id vacío para la imagen"
	case strings.Contains(lower, "execute storage upload"):
		return "error de red al subir a Storage"
	case strings.Contains(lower, "timeout"):
		return "tiempo de espera agotado al subir imagen"
	default:
		if len(msg) > 160 {
			return msg[:157] + "..."
		}
		return msg
	}
}

func formatImageSyncStatsMessage(uploaded, skipped, failed int, summaries []ImageSyncErrorSummary) string {
	base := fmt.Sprintf("subidas=%d omitidas=%d fallidas=%d", uploaded, skipped, failed)
	if failed == 0 || len(summaries) == 0 {
		return base
	}

	parts := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		parts = append(parts, fmt.Sprintf("%s×%d", summary.Category, summary.Count))
	}
	return base + "; errores: " + strings.Join(parts, ", ")
}
