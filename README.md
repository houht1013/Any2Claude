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

Install [Go](https://go.dev/dl/) (download .msi for Windows), then:

```cmd
cd D:\x2claude-proxy
go build -ldflags="-s -w -H windowsgui" -o Any2Claude.exe .
```

Or simply double-click `build.bat`.

### 2. Run

Double-click `Any2Claude.exe`. A tray icon appears in the system tray.

- **Right-click** tray icon → Open Dashboard / Quit
- **Double-click** tray icon → Open Dashboard
- Dashboard runs at `http://127.0.0.1:8090`

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
x2claude-proxy/
├── main.go            # Entry point, config loading, HTTP servers, tray launcher
├── proxy.go           # Reverse proxy core + Anthropic↔OpenAI format translation
├── api.go             # Dashboard REST API + log buffer
├── config.go          # Thread-safe config management
├── icon.go            # Tray icon (embeds assets/any2claude-tray-logo.ico)
├── tray_windows.go    # Windows system tray (pure Win32 syscall, no CGO)
├── tray_other.go      # Non-Windows stub
├── dashboard.html     # Web Dashboard (embedded into exe)
├── config.json        # Default config (embedded into exe)
├── assets/
│   ├── any2claude-tray-logo.ico   # Tray icon
│   ├── any2claude-tray-logo.svg   # Vector logo
│   └── any2claude-tray-logo-*.png # PNG variants
├── go.mod             # Go module (zero external dependencies)
├── build.bat          # Windows one-click build script
└── README.md
```

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
