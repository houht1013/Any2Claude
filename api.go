package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
//  Log buffer (ring buffer for dashboard)
// ---------------------------------------------------------------------------

type LogEntry struct {
	Seq   int    `json:"seq"`
	TS    string `json:"ts"`
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

type LogBuffer struct {
	mu       sync.Mutex
	entries  []LogEntry
	seq      int
	capacity int
}

func NewLogBuffer(capacity int) *LogBuffer {
	return &LogBuffer{
		entries:  make([]LogEntry, 0, capacity),
		capacity: capacity,
	}
}

func (lb *LogBuffer) Add(level, msg string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	entry := LogEntry{
		Seq:   lb.seq,
		TS:    time.Now().Format("15:04:05"),
		Level: level,
		Msg:   msg,
	}
	lb.entries = append(lb.entries, entry)
	if len(lb.entries) > lb.capacity {
		lb.entries = lb.entries[len(lb.entries)-lb.capacity:]
	}
	lb.seq++
}

func (lb *LogBuffer) GetAfter(afterSeq int) []LogEntry {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	var result []LogEntry
	for _, e := range lb.entries {
		if e.Seq > afterSeq {
			result = append(result, e)
		}
	}
	return result
}

var logBuf = NewLogBuffer(500)

// ---------------------------------------------------------------------------
//  Dashboard API Handler
// ---------------------------------------------------------------------------

type DashboardServer struct {
	configMgr     *ConfigManager
	dashboardHTML string
}

func NewDashboardServer(cm *ConfigManager, html string) *DashboardServer {
	return &DashboardServer{
		configMgr:     cm,
		dashboardHTML: html,
	}
}

func (ds *DashboardServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")

	if r.Method == http.MethodOptions {
		w.WriteHeader(200)
		return
	}

	path := r.URL.Path

	switch {
	case path == "/" || path == "/index.html":
		ds.serveDashboard(w, r)
	case path == "/api/config" && r.Method == http.MethodGet:
		ds.getConfig(w, r)
	case path == "/api/config" && r.Method == http.MethodPost:
		ds.updateConfig(w, r)
	case path == "/api/stats":
		ds.getStats(w, r)
	case path == "/api/logs":
		ds.getLogs(w, r)
	case path == "/api/providers" && r.Method == http.MethodPost:
		ds.addProvider(w, r)
	case strings.HasPrefix(path, "/api/providers/") && r.Method == http.MethodPut:
		ds.updateProvider(w, r)
	case strings.HasPrefix(path, "/api/providers/") && r.Method == http.MethodDelete:
		ds.deleteProvider(w, r)
	case path == "/api/models" && r.Method == http.MethodPost:
		ds.addModel(w, r)
	case strings.HasPrefix(path, "/api/models/") && r.Method == http.MethodPut:
		ds.updateModel(w, r)
	case strings.HasPrefix(path, "/api/models/") && r.Method == http.MethodDelete:
		ds.deleteModel(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (ds *DashboardServer) serveDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(ds.dashboardHTML))
}

func (ds *DashboardServer) jsonResp(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}

func (ds *DashboardServer) getConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(ds.configMgr.GetJSON())
}

func (ds *DashboardServer) updateConfig(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := ds.configMgr.Update(raw); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	logBuf.Add("INFO", "Config updated via dashboard")
	ds.jsonResp(w, map[string]bool{"ok": true})
}

func (ds *DashboardServer) getStats(w http.ResponseWriter, r *http.Request) {
	ds.jsonResp(w, GetStats(ds.configMgr))
}

func (ds *DashboardServer) getLogs(w http.ResponseWriter, r *http.Request) {
	afterStr := r.URL.Query().Get("after")
	after := -1
	if afterStr != "" {
		after, _ = strconv.Atoi(afterStr)
	}
	entries := logBuf.GetAfter(after)
	if entries == nil {
		entries = []LogEntry{}
	}
	ds.jsonResp(w, entries)
}

func (ds *DashboardServer) addProvider(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if body.ID == "" {
		http.Error(w, `{"error":"id required"}`, 400)
		return
	}
	ds.configMgr.AddProvider(body.ID, &Provider{
		Name:    body.Name,
		BaseURL: body.BaseURL,
		APIKey:  body.APIKey,
		Enabled: body.Enabled,
	})
	logBuf.Add("INFO", fmt.Sprintf("Provider added: %s", body.ID))
	ds.jsonResp(w, map[string]bool{"ok": true})
}

func (ds *DashboardServer) updateProvider(w http.ResponseWriter, r *http.Request) {
	pid := strings.TrimPrefix(r.URL.Path, "/api/providers/")
	var body Provider
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ds.configMgr.UpdateProvider(pid, &body)
	ds.jsonResp(w, map[string]bool{"ok": true})
}

func (ds *DashboardServer) deleteProvider(w http.ResponseWriter, r *http.Request) {
	pid := strings.TrimPrefix(r.URL.Path, "/api/providers/")
	ds.configMgr.DeleteProvider(pid)
	logBuf.Add("INFO", fmt.Sprintf("Provider deleted: %s", pid))
	ds.jsonResp(w, map[string]bool{"ok": true})
}

func (ds *DashboardServer) addModel(w http.ResponseWriter, r *http.Request) {
	var m ModelMapping
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ds.configMgr.AddModel(&m)
	logBuf.Add("INFO", fmt.Sprintf("Model added: %s -> %s", m.DisplayName, m.RealModel))
	ds.jsonResp(w, map[string]bool{"ok": true})
}

func (ds *DashboardServer) updateModel(w http.ResponseWriter, r *http.Request) {
	idxStr := strings.TrimPrefix(r.URL.Path, "/api/models/")
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		http.Error(w, "invalid index", 400)
		return
	}
	var m ModelMapping
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ds.configMgr.UpdateModel(idx, &m)
	ds.jsonResp(w, map[string]bool{"ok": true})
}

func (ds *DashboardServer) deleteModel(w http.ResponseWriter, r *http.Request) {
	idxStr := strings.TrimPrefix(r.URL.Path, "/api/models/")
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		http.Error(w, "invalid index", 400)
		return
	}
	ds.configMgr.DeleteModel(idx)
	logBuf.Add("INFO", fmt.Sprintf("Model deleted at index %d", idx))
	ds.jsonResp(w, map[string]bool{"ok": true})
}
