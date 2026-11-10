package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	syncworker "sycronizafhir/internal/sync"
)

func main() {
	dryRun := flag.Bool("dry-run", true, "solo listar y registrar, no escribe en pedidos (default true)")
	apply := flag.Bool("apply", false, "escribir en pedidos.bultos (anula -dry-run)")
	maxBultos := flag.Int("max", syncworker.DefaultBultosFacturaMax, "tope: copiar solo valores entre 1 y este número")
	windowDays := flag.Int("days", syncworker.DefaultBultosFacturaWindowDays, "ventana en días sobre fvc_fecha")
	flag.Parse()

	runDry := *dryRun && !*apply
	if *maxBultos <= 0 || *windowDays <= 0 {
		fmt.Fprintf(os.Stderr, "-max y -days deben ser mayores a 0\n")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	resolution, err := db.ResolveLocalPostgresSource(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local source: %v\n", err)
		os.Exit(1)
	}

	localPG, err := db.NewLocalPG(ctx, resolution.Selected.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local connect: %v\n", err)
		os.Exit(1)
	}
	defer localPG.Close()

	schema := cfg.SourceSchema
	if schema == "" {
		schema = "public"
	}

	mode := "DRY-RUN (no se escribe nada)"
	if !runDry {
		mode = "APPLY (se escribe pedidos.bultos si está vacío)"
	}
	fmt.Printf("bultos factura → pedidos  %s\n", mode)
	fmt.Printf("ventana=%d días  tope=1..%d  schema=%s\n", *windowDays, *maxBultos, schema)

	result, err := syncworker.RunBultosFacturaCycle(ctx, localPG, schema, *windowDays, *maxBultos, runDry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ciclo: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("candidatos (pedido vacío con factura de bultos): %d\n", result.Candidates)
	fmt.Printf("se completarían (1..%d): %d\n", *maxBultos, result.Copied)
	fmt.Printf("fuera de tope (>%d): %d\n", *maxBultos, result.SkippedCap)
	if !runDry {
		fmt.Printf("aplicados: %d\n", result.Applied)
	}

	overCap := 0
	for _, e := range result.Entries {
		if e.Action == syncworker.BultosFacturaSkipOverCap {
			if overCap == 0 {
				fmt.Println("pedidos fuera de tope (sin copiar):")
			}
			overCap++
			fmt.Printf("  ped_id=%s factura=%s bultos=%d fecha=%s\n", e.PedID, e.Factura, e.ValorNuevo, e.FacturaFecha)
		}
	}

	if path, err := syncworker.BultosFacturaLogPath(); err == nil {
		fmt.Printf("log: %s\n", path)
	}
}
