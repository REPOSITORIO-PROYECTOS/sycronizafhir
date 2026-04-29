package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type componentState struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
}

type snapshot struct {
	StartedAt  time.Time                 `json:"started_at"`
	Now        time.Time                 `json:"now"`
	Components map[string]componentState `json:"components"`
	Meta       map[string]string         `json:"meta"`
	LastScan   *ScanResult               `json:"last_scan,omitempty"`
	Logs       []string                  `json:"logs"`
}

type ScanIssue struct {
	Level     string `json:"level"`
	Component string `json:"component"`
	Message   string `json:"message"`
}

type ScanResult struct {
	ScannedAt    time.Time         `json:"scanned_at"`
	Status       string            `json:"status"`
	Summary      string            `json:"summary"`
	Issues       []ScanIssue       `json:"issues"`
	Metrics      map[string]string `json:"metrics,omitempty"`
	ComparedWith *time.Time        `json:"compared_with,omitempty"`
	Changes      []string          `json:"changes,omitempty"`
}

type ScannerFunc func(ctx context.Context) (ScanResult, error)

type Runtime struct {
	mu         sync.RWMutex
	startedAt  time.Time
	components map[string]componentState
	meta       map[string]string
	lastScan   *ScanResult
	scanner    ScannerFunc
	logs       []string
	maxLogs    int
}

func NewRuntime() *Runtime {
	return &Runtime{
		startedAt:  time.Now().UTC(),
		components: map[string]componentState{},
		meta:       map[string]string{},
		logs:       []string{},
		maxLogs:    300,
	}
}

func (r *Runtime) SetComponentStatus(name, status, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.components[name] = componentState{
		Status:    status,
		Message:   message,
		UpdatedAt: time.Now().UTC(),
	}
}

func (r *Runtime) AddLog(message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	line := fmt.Sprintf("%s | %s", time.Now().Format(time.RFC3339), strings.TrimSpace(message))
	r.logs = append(r.logs, line)
	if len(r.logs) > r.maxLogs {
		r.logs = r.logs[len(r.logs)-r.maxLogs:]
	}
}

func (r *Runtime) SetMeta(key, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.meta[key] = value
}

func (r *Runtime) SetScanner(scanner ScannerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scanner = scanner
}

func (r *Runtime) GetComponentStatus(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, exists := r.components[name]
	if !exists {
		return "", false
	}
	return state.Status, true
}

func (r *Runtime) GetComponentState(name string) (string, string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, exists := r.components[name]
	if !exists {
		return "", "", false
	}
	return state.Status, state.Message, true
}

func (r *Runtime) LogWriter() http.Handler {
	return http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		r.AddLog(req.URL.Query().Get("m"))
	})
}

func (r *Runtime) Writer() io.Writer {
	return &runtimeWriter{runtime: r}
}

func (r *Runtime) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/", "/index.html":
		r.serveHTML(w)
	case "/status":
		r.serveJSON(w)
	case "/scan":
		r.runScan(w, req)
	case "/scan/compare":
		r.runCompare(w, req)
	case "/scan/export":
		r.exportScan(w, req)
	default:
		http.NotFound(w, req)
	}
}

func (r *Runtime) serveJSON(w http.ResponseWriter) {
	r.mu.RLock()
	data := snapshot{
		StartedAt:  r.startedAt,
		Now:        time.Now().UTC(),
		Components: cloneComponents(r.components),
		Meta:       cloneMeta(r.meta),
		LastScan:   cloneScan(r.lastScan),
		Logs:       append([]string{}, r.logs...),
	}
	r.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func (r *Runtime) serveHTML(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width, initial-scale=1"/><title>sycronizafhir control center</title><script src="https://cdn.tailwindcss.com"></script></head><body class="bg-slate-950 text-slate-100"><div class="max-w-7xl mx-auto p-4 md:p-6"><div class="mb-4 flex items-center justify-between gap-3"><div><h1 class="text-2xl font-semibold">sycronizafhir Control Center</h1><p class="text-slate-400 text-sm">Monitor en vivo, escaneo y comparacion de estado</p></div><div class="flex gap-2"><button id="scanBtn" class="px-3 py-2 text-sm rounded bg-indigo-600 hover:bg-indigo-500">Escanear ahora</button><button id="compareBtn" class="px-3 py-2 text-sm rounded bg-cyan-700 hover:bg-cyan-600">Comparar con anterior</button><a href="/scan/export" class="px-3 py-2 text-sm rounded bg-slate-700 hover:bg-slate-600">Exportar reporte JSON</a></div></div><div class="rounded-xl border border-slate-800 bg-slate-900 p-4 mb-4"><h2 class="font-semibold mb-2">Que ejecutan los botones</h2><ul class="text-sm text-slate-300 list-disc pl-5 space-y-1"><li><strong>Escanear ahora:</strong> valida DB local, DB remota, descubrimiento de tablas y estado inbound.</li><li><strong>Comparar con anterior:</strong> corre un escaneo nuevo y marca cambios contra el ultimo resultado.</li><li><strong>Exportar reporte JSON:</strong> descarga el ultimo escaneo para compartir o auditar.</li></ul></div><div id="root" class="space-y-4"><div class="text-slate-400">Cargando...</div></div></div><script>const badge=(s)=>{if(s==='running')return 'bg-emerald-500/20 text-emerald-300';if(s==='error')return 'bg-rose-500/20 text-rose-300';if(s==='stopping')return 'bg-amber-500/20 text-amber-300';return 'bg-slate-700 text-slate-200'};const issue=(s)=>s==='error'?'bg-rose-500/20 text-rose-300 border-rose-900':'bg-amber-500/20 text-amber-300 border-amber-900';const esc=(v)=>String(v??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;');async function act(btnId,url,loading,label){const btn=document.getElementById(btnId);btn.disabled=true;btn.textContent=loading;try{await fetch(url,{method:'POST'});}finally{btn.disabled=false;btn.textContent=label;}}document.getElementById('scanBtn').addEventListener('click',()=>act('scanBtn','/scan','Escaneando...','Escanear ahora'));document.getElementById('compareBtn').addEventListener('click',()=>act('compareBtn','/scan/compare','Comparando...','Comparar con anterior'));async function load(){const r=await fetch('/status');const d=await r.json();const rows=Object.entries(d.components||{}).sort((a,b)=>a[0].localeCompare(b[0])).map(([k,v])=>'<tr class=\"border-b border-slate-800\"><td class=\"py-2 pr-2 font-medium\">'+esc(k)+'</td><td class=\"py-2 pr-2\"><span class=\"px-2 py-1 rounded text-xs '+badge(v.status)+'\">'+esc(v.status)+'</span></td><td class=\"py-2 pr-2 text-slate-300\">'+esc(v.message)+'</td><td class=\"py-2 text-slate-400 text-xs\">'+esc(v.updated_at)+'</td></tr>').join('');const metaRows=Object.entries(d.meta||{}).sort((a,b)=>a[0].localeCompare(b[0])).map(([k,v])=>'<tr class=\"border-b border-slate-800\"><td class=\"py-2 pr-2 text-slate-400\">'+esc(k)+'</td><td class=\"py-2 text-slate-200\">'+esc(v)+'</td></tr>').join('');const logs=(d.logs||[]).slice(-120).join('\n');const issues=((d.last_scan&&d.last_scan.issues)||[]).map(it=>'<li class=\"border rounded px-3 py-2 text-sm '+issue(it.level)+'\"><strong>'+esc(it.component)+'</strong>: '+esc(it.message)+'</li>').join('');const changes=((d.last_scan&&d.last_scan.changes)||[]).map(ch=>'<li class=\"text-sm text-cyan-300\">'+esc(ch)+'</li>').join('');const scan=d.last_scan?('<div class=\"rounded-xl border border-slate-800 bg-slate-900 p-4\"><div class=\"flex items-center justify-between\"><h2 class=\"font-semibold\">Ultimo escaneo</h2><span class=\"text-xs text-slate-400\">'+esc(d.last_scan.scanned_at)+'</span></div><p class=\"text-sm mt-2 text-slate-300\">'+esc(d.last_scan.summary)+'</p>'+(issues?'<ul class=\"mt-3 space-y-2\">'+issues+'</ul>':'<p class=\"mt-3 text-sm text-emerald-300\">Sin problemas detectados.</p>')+(changes?'<div class=\"mt-3\"><h3 class=\"text-sm font-semibold text-cyan-300\">Cambios detectados</h3><ul class=\"mt-1 list-disc pl-5 space-y-1\">'+changes+'</ul></div>':'')+'</div>'):'<div class=\"rounded-xl border border-slate-800 bg-slate-900 p-4 text-slate-400 text-sm\">Todavia no hay escaneo. Usa el boton "Escanear ahora".</div>';document.getElementById('root').innerHTML=scan+'<div class=\"grid grid-cols-1 lg:grid-cols-2 gap-4\"><section class=\"rounded-xl border border-slate-800 bg-slate-900 p-4\"><h2 class=\"font-semibold mb-3\">Conexiones y Config</h2><table class=\"w-full text-sm\">'+metaRows+'</table></section><section class=\"rounded-xl border border-slate-800 bg-slate-900 p-4\"><h2 class=\"font-semibold mb-3\">Componentes</h2><table class=\"w-full text-sm\">'+rows+'</table></section></div><section class=\"rounded-xl border border-slate-800 bg-slate-900 p-4\"><div class=\"flex items-center justify-between mb-2\"><h2 class=\"font-semibold\">Logs recientes</h2><span class=\"text-xs text-slate-400\">Auto-refresh 2s</span></div><pre class=\"text-xs bg-slate-950 border border-slate-800 rounded p-3 overflow-auto max-h-[50vh] whitespace-pre-wrap\">'+esc(logs)+'</pre></section>';};setInterval(load,2000);load();</script></body></html>`))
}

func (r *Runtime) runScan(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.mu.RLock()
	scanner := r.scanner
	r.mu.RUnlock()
	if scanner == nil {
		http.Error(w, "scanner not configured", http.StatusNotImplemented)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 15*time.Second)
	defer cancel()

	result, err := scanner(ctx)
	if err != nil {
		result = ScanResult{
			ScannedAt: time.Now().UTC(),
			Status:    "error",
			Summary:   "Escaneo con errores",
			Issues: []ScanIssue{
				{Level: "error", Component: "scanner", Message: err.Error()},
			},
		}
	}

	r.mu.Lock()
	r.lastScan = &result
	r.mu.Unlock()
	r.AddLog("scan executed: " + result.Status)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (r *Runtime) runCompare(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.mu.RLock()
	previous := cloneScan(r.lastScan)
	r.mu.RUnlock()

	r.runScan(w, req)

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastScan == nil {
		return
	}
	if previous == nil {
		r.lastScan.Changes = []string{"No habia escaneo previo para comparar."}
		return
	}

	r.lastScan.ComparedWith = &previous.ScannedAt
	r.lastScan.Changes = compareScans(previous, r.lastScan)
}

func (r *Runtime) exportScan(w http.ResponseWriter, _ *http.Request) {
	r.mu.RLock()
	lastScan := cloneScan(r.lastScan)
	r.mu.RUnlock()
	if lastScan == nil {
		http.Error(w, "no scan available", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"sync-bridge-scan-%s.json\"", lastScan.ScannedAt.Format("20060102-150405")))
	_ = json.NewEncoder(w).Encode(lastScan)
}

func cloneComponents(source map[string]componentState) map[string]componentState {
	target := make(map[string]componentState, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func cloneMeta(source map[string]string) map[string]string {
	target := make(map[string]string, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func cloneScan(source *ScanResult) *ScanResult {
	if source == nil {
		return nil
	}
	cloned := *source
	cloned.Issues = append([]ScanIssue{}, source.Issues...)
	if source.Metrics != nil {
		cloned.Metrics = make(map[string]string, len(source.Metrics))
		for key, value := range source.Metrics {
			cloned.Metrics[key] = value
		}
	}
	return &cloned
}

func compareScans(previous, current *ScanResult) []string {
	changes := make([]string, 0)
	if previous.Status != current.Status {
		changes = append(changes, fmt.Sprintf("Estado general: %s -> %s", previous.Status, current.Status))
	}
	if len(previous.Issues) != len(current.Issues) {
		changes = append(changes, fmt.Sprintf("Cantidad de problemas: %d -> %d", len(previous.Issues), len(current.Issues)))
	}

	for key, currentValue := range current.Metrics {
		previousValue := previous.Metrics[key]
		if previousValue != currentValue {
			changes = append(changes, fmt.Sprintf("Metrica %s: %s -> %s", key, previousValue, currentValue))
		}
	}

	if len(changes) == 0 {
		changes = append(changes, "Sin cambios respecto al escaneo anterior.")
	}
	return changes
}

type runtimeWriter struct {
	runtime *Runtime
}

func (w *runtimeWriter) Write(p []byte) (int, error) {
	w.runtime.AddLog(string(p))
	return len(p), nil
}
