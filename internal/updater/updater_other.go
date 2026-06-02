//go:build !windows

package updater

import "context"

func Check(ctx context.Context) Status {
	_ = ctx
	return Status{
		CurrentVersion: ProductVersion(),
		CanApply:       false,
		Message:        "Actualizacion in-app solo disponible en Windows.",
	}
}

func Apply(reopenMonitor bool) ApplyResult {
	_ = reopenMonitor
	return ApplyResult{
		Success: false,
		Message: "Actualizacion in-app solo disponible en Windows.",
	}
}
