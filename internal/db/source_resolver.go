package db

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sycronizafhir/internal/config"
)

type SourceCandidate struct {
	Kind   string `json:"kind"`
	DSN    string `json:"dsn"`
	Reason string `json:"reason"`
}

type SourceResolution struct {
	Selected   SourceCandidate   `json:"selected"`
	Candidates []SourceCandidate `json:"candidates"`
}

func ResolveLocalPostgresSource(ctx context.Context, cfg config.Config) (SourceResolution, error) {
	if cfg.DBSourceMode != "auto-fallback" {
		dsn := strings.TrimSpace(cfg.LocalPostgresURL)
		if dsn == "" {
			return SourceResolution{}, errors.New("LOCAL_POSTGRES_URL no configurado")
		}
		if err := pingDSN(ctx, dsn); err != nil {
			return SourceResolution{}, fmt.Errorf("conexion local configurada fallo: %w", err)
		}
		selected := SourceCandidate{Kind: "local", DSN: dsn, Reason: "modo manual"}
		return SourceResolution{Selected: selected, Candidates: []SourceCandidate{selected}}, nil
	}

	candidates := make([]SourceCandidate, 0, 2)
	priority := cfg.DBSourcePriority
	for _, sourceType := range priority {
		switch sourceType {
		case "docker":
			dsn, err := discoverDockerPostgresDSN(ctx, cfg.LocalPostgresURL)
			if err != nil {
				candidates = append(candidates, SourceCandidate{Kind: "docker", Reason: err.Error()})
				continue
			}
			if err = pingDSN(ctx, dsn); err != nil {
				candidates = append(candidates, SourceCandidate{Kind: "docker", DSN: dsn, Reason: err.Error()})
				continue
			}
			selected := SourceCandidate{Kind: "docker", DSN: dsn, Reason: "conexion saludable"}
			candidates = append(candidates, selected)
			return SourceResolution{Selected: selected, Candidates: candidates}, nil
		case "local":
			dsn := strings.TrimSpace(cfg.LocalPostgresURL)
			if dsn == "" {
				candidates = append(candidates, SourceCandidate{Kind: "local", Reason: "LOCAL_POSTGRES_URL vacio"})
				continue
			}
			if err := pingDSN(ctx, dsn); err != nil {
				candidates = append(candidates, SourceCandidate{Kind: "local", DSN: dsn, Reason: err.Error()})
				continue
			}
			selected := SourceCandidate{Kind: "local", DSN: dsn, Reason: "fallback local saludable"}
			candidates = append(candidates, selected)
			return SourceResolution{Selected: selected, Candidates: candidates}, nil
		}
	}

	return SourceResolution{Candidates: candidates}, errors.New("no hay fuente PostgreSQL saludable (docker/local)")
}

func discoverDockerPostgresDSN(ctx context.Context, fallbackDSN string) (string, error) {
	base := strings.TrimSpace(fallbackDSN)
	if base == "" {
		base = "postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable"
	}
	parsedBase, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("dsn base invalido: %w", err)
	}
	if parsedBase.Hostname() == "" {
		parsedBase.Host = "127.0.0.1:5432"
	}
	if parsedBase.Path == "" || parsedBase.Path == "/" {
		parsedBase.Path = "/postgres"
	}
	if parsedBase.Query().Get("sslmode") == "" {
		queryValues := parsedBase.Query()
		queryValues.Set("sslmode", "disable")
		parsedBase.RawQuery = queryValues.Encode()
	}

	desiredHostPort := strings.TrimSpace(parsedBase.Port())
	cmd := exec.CommandContext(
		ctx,
		"docker",
		"ps",
		"--filter", "status=running",
		"--filter", "publish=5432",
		"--format", "{{.Names}}\t{{.Ports}}",
	)
	rawOutput, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker no disponible o sin permisos: %w", err)
	}

	output := strings.TrimSpace(string(rawOutput))
	if output == "" {
		return "", errors.New("sin contenedores postgres publicados en host:puerto")
	}

	lines := strings.Split(output, "\n")
	type candidate struct {
		dsn   string
		name  string
		ports string
	}
	candidates := make([]candidate, 0)
	for _, line := range lines {
		dsn, name, ports, hostPort, parseErr := buildDSNFromDockerPortLine(parsedBase, line)
		if parseErr != nil {
			continue
		}
		if desiredHostPort != "" && hostPort != desiredHostPort {
			continue
		}
		candidates = append(candidates, candidate{dsn: dsn, name: name, ports: ports})
	}

	if len(candidates) == 1 {
		return candidates[0].dsn, nil
	}
	if len(candidates) > 1 {
		labels := make([]string, 0, len(candidates))
		for _, item := range candidates {
			labels = append(labels, fmt.Sprintf("%s (%s)", item.name, item.ports))
		}
		return "", fmt.Errorf("varios contenedores postgres publicados: %s; especifique el puerto en LOCAL_POSTGRES_URL", strings.Join(labels, ", "))
	}

	return "", errors.New("no se pudo inferir puerto publicado de docker")
}

func buildDSNFromDockerPortLine(base *url.URL, line string) (dsn, name, ports, hostPort string, _ error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", "", "", "", errors.New("linea vacia")
	}
	fields := strings.SplitN(trimmed, "\t", 2)
	if len(fields) != 2 {
		return "", "", "", "", errors.New("formato invalido")
	}
	name = strings.TrimSpace(fields[0])
	ports = strings.TrimSpace(fields[1])
	if name == "" || ports == "" {
		return "", "", "", "", errors.New("formato incompleto")
	}

	parts := strings.Split(ports, ",")
	for _, part := range parts {
		segment := strings.TrimSpace(part)
		if !strings.Contains(segment, "->5432/tcp") || !strings.Contains(segment, ":") {
			continue
		}
		arrowIndex := strings.Index(segment, "->")
		left := segment
		if arrowIndex > 0 {
			left = strings.TrimSpace(segment[:arrowIndex])
		}
		lastColon := strings.LastIndex(left, ":")
		if lastColon < 0 || lastColon == len(left)-1 {
			continue
		}
		host := strings.TrimSpace(left[:lastColon])
		if strings.HasPrefix(host, "0.0.0.0") || strings.HasPrefix(host, "::") || host == "" {
			host = "127.0.0.1"
		}
		hostPort = strings.TrimSpace(left[lastColon+1:])
		if hostPort == "" {
			continue
		}

		candidate := *base
		candidate.Host = fmt.Sprintf("%s:%s", host, hostPort)
		return candidate.String(), name, ports, hostPort, nil
	}
	return "", name, ports, "", errors.New("linea sin mapping util")
}

func pingDSN(ctx context.Context, dsn string) error {
	pingCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	pool, err := pgxpool.New(pingCtx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	return pool.Ping(pingCtx)
}
