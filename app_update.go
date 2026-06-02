package main

import (
	"context"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"sycronizafhir/internal/updater"
)

func (a *App) CheckForUpdate() updater.Status {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	return updater.Check(ctx)
}

func (a *App) ApplyUpdate() updater.ApplyResult {
	result := updater.Apply(true)
	if !result.Success {
		return result
	}

	if a.ctx != nil {
		a.runtime.AddLog("actualizacion: cerrando aplicacion para aplicar release")
		go func() {
			time.Sleep(400 * time.Millisecond)
			wailsruntime.Quit(a.ctx)
		}()
	}

	return result
}
