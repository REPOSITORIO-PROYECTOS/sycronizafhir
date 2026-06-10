package sync

import (
	"fmt"
	"strings"
	"testing"
)

func TestClassifyImageSyncError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err  error
		want imageSyncErrorCategory
	}{
		{
			err:  fmt.Errorf(`storage upload failed bucket=productos object=x.jpg status=400 body={"statusCode":"403","error":"Unauthorized","message":"new row violates row-level security policy"}`),
			want: imageSyncErrRLSAuth,
		},
		{err: fmt.Errorf("archivo no encontrado: C:\\img\\a.jpg"), want: imageSyncErrFileMissing},
		{err: fmt.Errorf("ruta no local: https://x"), want: imageSyncErrPathInvalid},
		{err: fmt.Errorf("execute storage upload request: connection refused"), want: imageSyncErrNetwork},
		{err: fmt.Errorf("upsert remoto falló"), want: imageSyncErrUpsertRemote},
	}

	for _, tc := range cases {
		if got := classifyImageSyncError(tc.err); got != tc.want {
			t.Fatalf("classifyImageSyncError(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestNormalizeImageSyncErrorStripsRLSBody(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf(
		`storage upload failed bucket=productos object=00600171.jpg status=400 body={"statusCode":"403","error":"Unauthorized","message":"new row violates row-level security policy"}`,
	)
	got := normalizeImageSyncError(err)
	if strings.Contains(got, "00600171") || strings.Contains(got, "statusCode") {
		t.Fatalf("expected short normalized message, got %q", got)
	}
	if !strings.Contains(got, "RLS") {
		t.Fatalf("expected RLS hint, got %q", got)
	}
}

func TestImageSyncFailureCollectorAggregatesByCategory(t *testing.T) {
	t.Parallel()

	collector := newImageSyncFailureCollector()
	rlsErr := fmt.Errorf(`storage upload failed bucket=productos object=a.jpg status=400 body={"statusCode":"403","message":"new row violates row-level security policy"}`)
	for i := 0; i < 10; i++ {
		collector.RecordFailure(fmt.Sprintf("prod-%02d", i), rlsErr)
	}
	collector.RecordFailure("missing-1", fmt.Errorf("archivo no encontrado: C:\\img\\x.jpg"))

	if got := collector.TotalFailed(); got != 11 {
		t.Fatalf("TotalFailed() = %d, want 11", got)
	}

	summaries := collector.ToSummaries()
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	var rlsSummary *ImageSyncErrorSummary
	for i := range summaries {
		if summaries[i].Category == string(imageSyncErrRLSAuth) {
			rlsSummary = &summaries[i]
			break
		}
	}
	if rlsSummary == nil {
		t.Fatal("expected rls_auth summary")
	}
	if rlsSummary.Count != 10 {
		t.Fatalf("rls count = %d, want 10", rlsSummary.Count)
	}
	if len(rlsSummary.SampleIDs) != imageSyncErrorSampleLimit {
		t.Fatalf("sample ids = %d, want %d", len(rlsSummary.SampleIDs), imageSyncErrorSampleLimit)
	}
}

func TestImageSyncFailureCollectorMerge(t *testing.T) {
	t.Parallel()

	left := newImageSyncFailureCollector()
	right := newImageSyncFailureCollector()
	err := fmt.Errorf("archivo no encontrado: a.jpg")
	left.RecordFailure("a", err)
	right.RecordFailure("b", err)
	right.RecordFailure("c", err)

	left.Merge(right)
	summaries := left.ToSummaries()
	if len(summaries) != 1 || summaries[0].Count != 3 {
		t.Fatalf("unexpected merge result: %+v", summaries)
	}
}

func TestFormatImageSyncStatsMessage(t *testing.T) {
	t.Parallel()

	msg := formatImageSyncStatsMessage(2, 1, 5, []ImageSyncErrorSummary{
		{Category: string(imageSyncErrRLSAuth), Count: 5},
	})
	if !strings.Contains(msg, "fallidas=5") || !strings.Contains(msg, "rls_auth×5") {
		t.Fatalf("unexpected stats message: %q", msg)
	}
}
