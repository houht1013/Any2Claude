# Any2Claude

Local reverse proxy that enables Claude Desktop to use third-party models (GLM-4, Qwen, DeepSeek, Doubao, etc.) by mapping custom model IDs to real upstream API endpoints.

```
Claude Desktop                Any2Claude                    API Provider
 claude-fast-v3  ──────>  localhost:8089  ──────>  api.opclab.vip
                           model mapping              glm-4
                           format conversion           (OpenAI format)
```

## Features

- **Single-file exe** — ~8-10MB, double-click to run, zero runtime dependencies
- **System tray** — right-click to open Dashboard or quit
- **Web Dashboard** — visual management of Providers, model mappings, logs, and settings
- **Multi-provider** — route each model to a different API provider independently
- **API format translation** — auto-converts Anthropic API format to OpenAI-compatible format (and back)
- **SSE streaming** — full streaming support with real-time format conversion
- **Debug mode** — detailed request/response logging viewable in Dashboard

## Quick Start

### 1. Build

Install [Go](https://go.dev/dl/) (download .msi for Windows, .pkg for macOS), then:

**Windows:**
```cmd
cd Any2Claude
scripts\build.bat
```
Or manually: `go build -ldflags="-s -w -H windowsgui" -o Any2Claude.exe ./cmd/any2claude`

Cross-compile all platforms: `scripts\build.bat all`

**macOS / Linux:**
```bash
cd Any2Claude
chmod +x scripts/build.sh
scripts/build.sh
```
Or manually:
```bash
go build -ldflags="-s -w" -o Any2Claude ./cmd/any2claude
```

> macOS users can also install Go via Homebrew: `brew install go`

### 2. Run

**Windows:** Double-click `Any2Claude.exe`. A tray icon appears in the system tray.

- **Right-click** tray icon → Open Dashboard / Quit
- **Double-click** tray icon → Open Dashboard

**macOS / Linux:** Run from terminal:
```bash
./Any2Claude
```
Or run in background:
```bash
nohup ./Any2Claude &
```

Dashboard runs at `http://127.0.0.1:8090`

### 3. Configure Providers

Open Dashboard → **Providers** tab → **+ Add Provider**

| Field | Description |
|-------|------------|
| ID | Unique identifier, lowercase (e.g. `opclab`, `deepseek`) |
| Display Name | Human-readable name (e.g. "OPC Lab") |
| Base URL | API endpoint (e.g. `https://api.opclab.vip`) |
| API Format | `OpenAI Compatible` (most Chinese providers) or `Anthropic Native` |
| API Key | Your API key for this provider |

### 4. Configure Model Mappings

Open Dashboard → **Models** tab → **+ Add Model**

| Field | Description |
|-------|------------|
| Model ID | The name Claude Desktop sees. **Must start with `claude-`** |
| Real Model ID | The actual model name sent to the provider |
| Provider | Which API provider to route this model to |

**Example mappings:**

| Model ID (Claude Desktop) | → | Real Model ID | Provider |
|--------------------------|---|--------------|----------|
| `claude-fast-v3` | → | `glm-4` | Zhipu |
| `claude-think-max` | → | `qwen-max` | Alibaba |
| `claude-reason-v3` | → | `deepseek-chat` | DeepSeek |
| `claude-reason-r1` | → | `deepseek-reasoner` | DeepSeek |
| `claude-pro-256k` | → | `doubao-pro-256k` | Doubao |
| `claude-mim-v2.5` | → | `mimo-v2.5` | OPC Lab |

> **Important:** Do NOT include competitor brand names after `claude-` (e.g. `claude-gpt`, `claude-deepseek`, `claude-gemini`). Claude Desktop will reject model IDs containing competitor names. Use custom aliases like `claude-fast-v3`, `claude-think-pro`, etc.

### 5. Configure Claude Desktop

1. Open Claude Desktop → **Settings** → **Developer**
2. Click **Configure Third-Party Inference**
3. In the **Connection** section, find **Gateway**
4. Set Gateway URL to:

```
http://127.0.0.1:8089
```

5. Restart Claude Desktop

Your mapped models will appear in the model selector.

## How It Works

```
┌─────────────────┐     ┌──────────────────────┐     ┌─────────────────┐
│  Claude Desktop  │     │      Any2Claude       │     │  API Provider   │
│                  │     │                       │     │                 │
│  Sends request   │────>│  1. Intercept request │────>│  Receives       │
│  with model:     │     │  2. Map model name    │     │  OpenAI-format  │
│  "claude-fast-v3"│     │  3. Convert format    │     │  request with   │
│                  │     │     Anthropic→OpenAI   │     │  model: "glm-4" │
│  Receives        │<────│  4. Convert response  │<────│                 │
│  Anthropic-format│     │     OpenAI→Anthropic   │     │  Returns        │
│  streaming resp  │     │  5. Stream back       │     │  OpenAI-format  │
└─────────────────┘     └──────────────────────┘     │  response       │
                                                      └─────────────────┘
```

### API Format Translation

Most Chinese API providers (OPC Lab, DeepSeek, Zhipu, Alibaba, ByteDance, etc.) use **OpenAI-compatible** API format. Claude Desktop sends requests in **Anthropic** format. Any2Claude automatically handles the conversion:

**Request conversion (Anthropic → OpenAI):**
- Endpoint: `/v1/messages` → `/v1/chat/completions`
- System prompt: top-level `system` field → system message in `messages` array
- Content blocks: Anthropic content array → plain text string
- Parameters: `max_tokens`, `temperature`, `top_p` mapped directly

**Response conversion (OpenAI → Anthropic):**
- Non-streaming: OpenAI `choices[0].message.content` → Anthropic `content[{type:"text"}]` format
- Streaming: OpenAI `data: {"choices":[{"delta":{"content":"..."}}]}` → Anthropic SSE events (`message_start`, `content_block_delta`, `message_stop`)
- Token usage and finish reasons mapped between formats

If a provider uses native Anthropic format, set API Format to `Anthropic Native` — requests will pass through without conversion.

## Configuration

### Config file location

- **Portable mode:** Place `config.json` next to the exe
- **User data mode (default):**
  - Windows: `%APPDATA%\Any2Claude\config.json`
  - macOS: `~/Library/Application Support/Any2Claude/config.json`
  - Linux: `~/.config/any2claude/config.json`

Changes made through the Dashboard take effect immediately (no restart needed, except for port changes).

### config.json structure

```json
{
  "listen": {
    "host": "127.0.0.1",
    "port": 8089
  },
  "providers": {
    "opclab": {
      "name": "OPC Lab",
      "base_url": "https://api.opclab.vip",
      "api_key": "sk-...",
      "api_format": "openai",
      "enabled": true
    }
  },
  "models": [
    {
      "display_name": "claude-fast-v3",
      "real_model": "glm-4",
      "provider": "opclab",
      "enabled": true
    }
  ],
  "log_level": "info",
  "debug_mode": false,
  "auto_start": true,
  "open_dashboard_on_start": false
}
```

| Field | Description |
|-------|------------|
| `listen.host` | Bind address (default `127.0.0.1`) |
| `listen.port` | Proxy port (default `8089`), Dashboard runs on port+1 (`8090`) |
| `providers` | Map of API providers with base_url, api_key, api_format |
| `providers.*.api_format` | `"openai"` (default) or `"anthropic"` |
| `models` | Array of model mappings (display_name → real_model + provider) |
| `debug_mode` | When `true`, logs full request/response details to Dashboard |
| `open_dashboard_on_start` | Auto-open Dashboard in browser on startup |

## Debug Mode

Enable Debug Mode in Dashboard → **Settings** to see detailed logs including:

- Incoming request method, path, and body
- Model mapping decisions
- Upstream URL construction and format conversion
- Outgoing request body (after conversion)
- Upstream response status, headers, and body
- Streaming event details

All debug logs appear in Dashboard → **Logs** tab with auto-scroll.

## Project Structure

```
Any2Claude/
├── cmd/any2claude/            # Go source (package main)
│   ├── main.go                #   Entry point, CLI flags, HTTP servers
│   ├── proxy.go               #   Reverse proxy + Anthropic↔OpenAI translation
│   ├── api.go                 #   Dashboard REST API + log buffer
│   ├── config.go              #   Thread-safe config management
│   ├── icon.go                #   Tray icon loader
│   ├── tray_windows.go        #   Windows system tray (pure Win32 syscall)
│   ├── tray_other.go          #   Non-Windows stub (macOS/Linux)
│   └── embed/                 #   Assets compiled into binary
│       ├── dashboard.html     #     Web Dashboard UI
│       ├── config.json        #     Default config template
│       └── icon/
│           └── any2claude-tray-logo.ico
├── assets/                    # Source artwork (not compiled in)
│   ├── any2claude-tray-logo.svg
│   └── any2claude-tray-logo-*.png
├── scripts/
│   ├── build.bat              # Windows build (supports cross-compile)
│   └── build.sh               # macOS/Linux build
├── deploy/
│   ├── install.sh             # Linux server install (systemd)
│   └── any2claude.service     # systemd unit file
├── go.mod                     # Go module (zero external dependencies)
├── Dockerfile
├── docker-compose.yml
├── .gitignore
├── README.md
└── README_CN.md
```

## Linux Server Deployment

Any2Claude can run as a service on a Linux server, allowing multiple users' Claude Desktop clients to connect remotely.

### Option A: systemd (recommended)

```bash
# Clone and install
git clone https://github.com/houht1013/Any2Claude.git
cd Any2Claude
sudo chmod +x deploy/install.sh
sudo ./deploy/install.sh
```

This will:
- Build the binary with Go
- Install to `/opt/any2claude/`
- Create a `any2claude` system user
- Install and start a systemd service

**Post-install:**
```bash
# Edit config (set your API keys!)
sudo nano /opt/any2claude/config.json
sudo systemctl restart any2claude

# Manage service
systemctl status any2claude       # Check status
journalctl -u any2claude -f       # Follow logs
sudo systemctl restart any2claude # Restart
sudo systemctl stop any2claude    # Stop

# Uninstall
sudo ./deploy/install.sh --uninstall
```

### Option B: Docker

```bash
git clone https://github.com/houht1013/Any2Claude.git
cd Any2Claude

# Edit config.json with your API keys first
nano config.json

# Build and run
docker compose up -d

# View logs
docker compose logs -f

# Stop
docker compose down
```

### Option C: Manual

```bash
# Build
go build -ldflags="-s -w" -o Any2Claude ./cmd/any2claude

# Run (listen on all interfaces)
./Any2Claude -host 0.0.0.0 -port 8089

# Or run in background
nohup ./Any2Claude -host 0.0.0.0 -port 8089 > /var/log/any2claude.log 2>&1 &
```

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-host` | from config | Listen address (`0.0.0.0` for all interfaces) |
| `-port` | from config | Proxy port (dashboard = port+1) |
| `-config` | auto-detect | Path to config.json |

### Firewall

```bash
# Allow proxy and dashboard ports
sudo ufw allow 8089/tcp    # Proxy
sudo ufw allow 8090/tcp    # Dashboard (optional, restrict in production)
```

### Claude Desktop → Remote Server

In Claude Desktop: **Settings → Developer → Configure Third-Party Inference → Gateway**

```
http://<your-server-ip>:8089
```

> **Security tip:** In production, use a reverse proxy (nginx/Caddy) with HTTPS and authentication in front of Any2Claude.

## Technical Notes

- **Zero external dependencies** — pure Go stdlib + Windows syscall
- **No CGO required** — system tray implemented via direct Win32 API calls
- **All assets embedded** — `dashboard.html`, `config.json`, and `.ico` compiled into the single exe via `//go:embed`
- **Thread-safe config** — `sync.RWMutex` protected, live-reload via Dashboard API
- **Streaming proxy** — 4096-byte chunked reads with immediate flush for low-latency SSE passthrough
- **Cross-platform config paths** — Windows APPDATA, macOS Library, Linux .config

## Ports

| Port | Service |
|------|---------|
| 8089 | Proxy (Claude Desktop connects here) |
| 8090 | Dashboard (Web UI) |

Both ports are configurable. Dashboard always runs on proxy port + 1.

## FAQ

**Q: Can I still use original Claude models?**
A: Yes. Models that don't match any mapping are passed through to the default provider. If you need original Claude models, configure an Anthropic-native provider.

**Q: Do I need to restart the proxy after changing config?**
A: No, config changes via Dashboard take effect immediately. Only port changes require a restart.

**Q: Where is the config file?**
A: If `config.json` exists next to the exe, it uses that (portable mode). Otherwise: `%APPDATA%\Any2Claude\config.json` (Windows).

**Q: The proxy is running but Claude Desktop shows errors?**
A: Enable Debug Mode in Settings, then check the Logs tab for detailed error info. Common issues: wrong API key, incorrect base_url, or upstream provider blocking requests.
