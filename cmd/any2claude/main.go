package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

//go:embed embed/dashboard.html
var dashboardFS embed.FS

//go:embed embed/config.json
var defaultConfigFS embed.FS

const (
	appName = "Any2Claude"
	version = "1.0.0"
)

// ---------------------------------------------------------------------------
//  Path helpers
// ---------------------------------------------------------------------------

func getExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func getConfigPath() string {
	// 1. exe 同目录（便携模式）
	exeDir := getExeDir()
	local := filepath.Join(exeDir, "config.json")
	if _, err := os.Stat(local); err == nil {
		return local
	}

	// 2. 用户数据目录
	var dataDir string
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData, _ = os.UserHomeDir()
		}
		dataDir = filepath.Join(appData, "Any2Claude")
	case "darwin":
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, "Library", "Application Support", "Any2Claude")
	default:
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".config", "any2claude")
	}

	os.MkdirAll(dataDir, 0755)
	userCfg := filepath.Join(dataDir, "config.json")

	// 首次运行：从 embed 写入默认配置
	if _, err := os.Stat(userCfg); os.IsNotExist(err) {
		defaultCfg, err := defaultConfigFS.ReadFile("embed/config.json")
		if err == nil {
			os.WriteFile(userCfg, defaultCfg, 0644)
		}
	}

	return userCfg
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

// ---------------------------------------------------------------------------
//  Main
// ---------------------------------------------------------------------------

func main() {
	// CLI flags
	flagHost := flag.String("host", "", "Listen address (overrides config, e.g. 0.0.0.0)")
	flagPort := flag.Int("port", 0, "Proxy port (overrides config, dashboard = port+1)")
	flagConfig := flag.String("config", "", "Path to config.json")
	flag.Parse()

	// Load config
	var configPath string
	if *flagConfig != "" {
		configPath = *flagConfig
	} else {
		configPath = getConfigPath()
	}
	cfgMgr := NewConfigManager(configPath)
	cfg := cfgMgr.Get()

	// CLI overrides
	listenHost := cfg.Listen.Host
	proxyPort := cfg.Listen.Port
	if *flagHost != "" {
		listenHost = *flagHost
	}
	if *flagPort > 0 {
		proxyPort = *flagPort
	}
	dashPort := proxyPort + 1

	statStartTime = time.Now()

	log.Printf("[main] %s v%s", appName, version)
	log.Printf("[main] Config: %s", configPath)
	log.Printf("[main] Proxy:  http://%s:%d", listenHost, proxyPort)
	log.Printf("[main] Dashboard: http://%s:%d", listenHost, dashPort)

	// Log model mappings
	modelMap := cfgMgr.BuildModelMap()
	log.Printf("[main] Active models: %d", len(modelMap))
	for dn, rm := range modelMap {
		log.Printf("[main]   %s -> %s", dn, rm.RealModel)
		logBuf.Add("INFO", fmt.Sprintf("%s -> %s", dn, rm.RealModel))
	}

	// Load dashboard HTML
	dashHTML, err := dashboardFS.ReadFile("embed/dashboard.html")
	if err != nil {
		log.Printf("[main] WARNING: dashboard.html not embedded: %v", err)
		dashHTML = []byte("<html><body><h1>Dashboard not available</h1></body></html>")
	}

	// Start proxy server
	proxyServer := NewProxyServer(cfgMgr)
	go func() {
		addr := fmt.Sprintf("%s:%d", listenHost, proxyPort)
		log.Printf("[proxy] Listening on %s", addr)
		logBuf.Add("INFO", fmt.Sprintf("Proxy listening on %s", addr))
		if err := http.ListenAndServe(addr, proxyServer); err != nil {
			log.Fatalf("[proxy] Failed to start: %v", err)
		}
	}()

	// Start dashboard server
	dashServer := NewDashboardServer(cfgMgr, string(dashHTML))
	go func() {
		addr := fmt.Sprintf("%s:%d", listenHost, dashPort)
		log.Printf("[dashboard] Listening on %s", addr)
		logBuf.Add("INFO", fmt.Sprintf("Dashboard listening on %s", addr))
		if err := http.ListenAndServe(addr, dashServer); err != nil {
			log.Fatalf("[dashboard] Failed to start: %v", err)
		}
	}()

	// Open dashboard on start if configured
	if cfg.OpenDashOnStart {
		time.Sleep(300 * time.Millisecond)
		openBrowser(fmt.Sprintf("http://127.0.0.1:%d", dashPort))
	}

	// Start system tray (Windows only, pure syscall)
	if runtime.GOOS == "windows" {
		go runSystray(proxyPort, dashPort)
	}

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("[main] Shutting down...")
}
