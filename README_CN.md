# Any2Claude

让 Claude Desktop 无缝使用第三方大模型（GLM-4、Qwen、DeepSeek、豆包等）的本地反向代理工具。

> **📖 [图文使用指南（中文）](docs/GUIDE_CN.md)** | **[User Guide with Screenshots (English)](docs/GUIDE.md)**

```
Claude Desktop                Any2Claude                    API 供应商
 claude-fast-v3  ──────>  localhost:8089  ──────>  api.opclab.vip
                           模型名映射                  glm-4
                           格式自动转换                 (OpenAI 格式)
```

## 特性

- **单文件 exe** — 约 8-10MB，双击即用，零运行时依赖
- **系统托盘** — 右键打开 Dashboard 或退出
- **Web 管理面板** — 可视化管理供应商、模型映射、日志和设置，支持中英双语 / 黑白主题
- **多供应商路由** — 每个模型可独立路由到不同的 API 供应商
- **API 格式自动转换** — Anthropic 格式 ↔ OpenAI 格式双向自动转换
- **SSE 流式传输** — 完整流式支持，实时格式转换，打字机效果流畅
- **Debug 模式** — 详细的请求/响应日志，可在 Dashboard 中查看

## 快速开始

### 1. 编译

安装 [Go](https://go.dev/dl/)（Windows 下载 .msi，macOS 下载 .pkg），然后：

**Windows：**
```cmd
cd Any2Claude
scripts\build.bat
```
或手动：`go build -ldflags="-s -w -H windowsgui" -o Any2Claude.exe ./cmd/any2claude`

全平台交叉编译：`scripts\build.bat all`

**macOS / Linux：**
```bash
cd Any2Claude
chmod +x scripts/build.sh
scripts/build.sh
```
或手动编译：
```bash
go build -ldflags="-s -w" -o Any2Claude ./cmd/any2claude
```

> macOS 也可以用 Homebrew 安装 Go：`brew install go`

> 中国用户如遇网络问题，先设置 Go 代理：
> ```cmd
> go env -w GOPROXY=https://goproxy.cn,direct
> ```

### 2. 运行

**Windows：** 双击 `Any2Claude.exe`，系统托盘出现图标。

- **右键** 托盘图标 → 打开 Dashboard / 退出
- **双击** 托盘图标 → 打开 Dashboard

**macOS / Linux：** 在终端运行：
```bash
./Any2Claude
```
或后台运行：
```bash
nohup ./Any2Claude &
```

Dashboard 地址：`http://127.0.0.1:8090`

### 3. 配置供应商

打开 Dashboard → **供应商** 标签 → **+ 添加供应商**

| 字段 | 说明 |
|------|------|
| ID | 唯一标识，小写字母（如 `opclab`、`deepseek`） |
| 显示名称 | 易读名称（如 "OPC Lab"） |
| Base URL | API 地址（如 `https://api.opclab.vip`） |
| API 格式 | `OpenAI Compatible`（大多数国产供应商）或 `Anthropic Native` |
| API Key | 该供应商的 API 密钥 |

### 4. 配置模型映射

打开 Dashboard → **模型** 标签 → **+ 添加模型**

| 字段 | 说明 |
|------|------|
| Model ID | Claude Desktop 中显示的名称，**必须以 `claude-` 开头** |
| Real Model ID | 实际发送给供应商的模型名 |
| 供应商 | 路由到哪个 API 供应商 |

**映射示例：**

| Model ID（Claude Desktop 中显示） | → | 实际模型名 | 供应商 |
|----------------------------------|---|-----------|--------|
| `claude-fast-v3` | → | `glm-4` | 智谱 |
| `claude-think-max` | → | `qwen-max` | 阿里 |
| `claude-reason-v3` | → | `deepseek-chat` | DeepSeek |
| `claude-reason-r1` | → | `deepseek-reasoner` | DeepSeek |
| `claude-pro-256k` | → | `doubao-pro-256k` | 豆包 |
| `claude-mim-v2.5` | → | `mimo-v2.5` | OPC Lab |

> **重要：** `claude-` 后面**不要使用竞品名称**（如 `claude-gpt`、`claude-deepseek`、`claude-gemini`），Claude Desktop 会拒绝包含竞品名的模型 ID。请使用自定义别名，如 `claude-fast-v3`、`claude-think-pro` 等。

### 5. 配置 Claude Desktop

1. 打开 Claude Desktop → **Settings（设置）** → **Developer（开发者）**
2. 点击 **Configure Third-Party Inference**
3. 在 **Connection** 区域找到 **Gateway**
4. 设置 Gateway URL 为：

```
http://127.0.0.1:8089
```

5. 重启 Claude Desktop

你配置的模型将出现在模型选择器中。

## 工作原理

```
┌─────────────────┐     ┌──────────────────────┐     ┌─────────────────┐
│  Claude Desktop  │     │      Any2Claude       │     │   API 供应商     │
│                  │     │                       │     │                 │
│  发送请求         │────>│  1. 拦截请求           │────>│  接收            │
│  model:          │     │  2. 映射模型名         │     │  OpenAI 格式     │
│  "claude-fast-v3"│     │  3. 转换格式           │     │  请求            │
│                  │     │     Anthropic→OpenAI   │     │  model: "glm-4" │
│  接收             │<────│  4. 转换响应           │<────│                 │
│  Anthropic 格式   │     │     OpenAI→Anthropic   │     │  返回            │
│  流式响应         │     │  5. 流式回传           │     │  OpenAI 格式     │
└─────────────────┘     └──────────────────────┘     │  响应            │
                                                      └─────────────────┘
```

### API 格式转换

大多数国产 API 供应商（OPC Lab、DeepSeek、智谱、阿里、字节等）使用 **OpenAI 兼容格式**。Claude Desktop 发送的是 **Anthropic 格式**。Any2Claude 自动处理双向转换：

**请求转换（Anthropic → OpenAI）：**
- 端点重写：`/v1/messages` → `/v1/chat/completions`
- System 提示：顶层 `system` 字段 → messages 数组中的 system 消息
- Content blocks：Anthropic 内容数组 → 纯文本字符串
- 参数映射：`max_tokens`、`temperature`、`top_p` 直接对应

**响应转换（OpenAI → Anthropic）：**
- 非流式：OpenAI `choices[0].message.content` → Anthropic `content[{type:"text"}]`
- 流式：OpenAI `data: {"choices":[{"delta":{"content":"..."}}]}` → Anthropic SSE 事件序列（`message_start`、`content_block_delta`、`message_stop`）
- Token 用量和结束原因自动映射

如果供应商原生支持 Anthropic 格式，将 API 格式设为 `Anthropic Native` 即可直通。

## 配置文件

### 存储位置

- **便携模式：** 将 `config.json` 放在 exe 同目录
- **用户数据模式（默认）：**
  - Windows：`%APPDATA%\Any2Claude\config.json`
  - macOS：`~/Library/Application Support/Any2Claude/config.json`
  - Linux：`~/.config/any2claude/config.json`

通过 Dashboard 修改的配置立即生效（端口修改除外，需重启）。

### config.json 结构

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

| 字段 | 说明 |
|------|------|
| `listen.host` | 绑定地址（默认 `127.0.0.1`） |
| `listen.port` | 代理端口（默认 `8089`），Dashboard 端口为 port+1（`8090`） |
| `providers` | API 供应商配置，包含 base_url、api_key、api_format |
| `providers.*.api_format` | `"openai"`（默认）或 `"anthropic"` |
| `models` | 模型映射数组（display_name → real_model + provider） |
| `debug_mode` | `true` 时记录完整请求/响应详情到 Dashboard 日志 |
| `open_dashboard_on_start` | 启动时自动在浏览器中打开 Dashboard |

## Debug 模式

在 Dashboard → **设置** 中开启 Debug 模式，可在日志中查看：

- 收到的请求方法、路径、请求体
- 模型映射决策
- 上游 URL 构建和格式转换详情
- 转换后的请求体
- 上游响应状态码、头信息、响应体
- 流式事件详情

所有日志在 Dashboard → **日志** 标签中实时显示。

## 项目结构

```
Any2Claude/
├── cmd/any2claude/            # Go 源码（package main）
│   ├── main.go                #   入口 + CLI 参数 + HTTP 服务
│   ├── proxy.go               #   反向代理 + Anthropic↔OpenAI 格式转换
│   ├── api.go                 #   Dashboard REST API + 日志缓冲
│   ├── config.go              #   线程安全的配置管理
│   ├── icon.go                #   托盘图标加载
│   ├── tray_windows.go        #   Windows 系统托盘（纯 Win32 syscall）
│   ├── tray_other.go          #   非 Windows 平台 stub
│   └── embed/                 #   编译时嵌入二进制的资源
│       ├── dashboard.html     #     Web 管理面板
│       ├── config.json        #     默认配置模板
│       └── icon/
│           └── any2claude-tray-logo.ico
├── assets/                    # 源始素材（不编译进二进制）
│   ├── any2claude-tray-logo.svg
│   └── any2claude-tray-logo-*.png
├── scripts/
│   ├── build.bat              # Windows 编译（支持交叉编译）
│   └── build.sh               # macOS/Linux 编译
├── deploy/
│   ├── install.sh             # Linux 服务器安装（systemd）
│   └── any2claude.service     # systemd 服务文件
├── go.mod                     # Go 模块定义（零外部依赖）
├── Dockerfile
├── docker-compose.yml
├── .gitignore
├── README.md
└── README_CN.md
├── README.md          # English documentation
└── README_CN.md       # 中文文档
```

## Linux 服务器部署

Any2Claude 可以部署在 Linux 服务器上作为服务运行，让多个用户的 Claude Desktop 远程连接。

### 方案 A：systemd（推荐）

```bash
# 克隆并安装
git clone https://github.com/houht1013/Any2Claude.git
cd Any2Claude
sudo chmod +x deploy/install.sh
sudo ./deploy/install.sh
```

安装脚本会自动：
- 用 Go 编译二进制
- 安装到 `/opt/any2claude/`
- 创建 `any2claude` 系统用户
- 安装并启动 systemd 服务

**安装后操作：**
```bash
# 编辑配置（填入你的 API Key！）
sudo nano /opt/any2claude/config.json
sudo systemctl restart any2claude

# 管理服务
systemctl status any2claude        # 查看状态
journalctl -u any2claude -f        # 查看实时日志
sudo systemctl restart any2claude  # 重启
sudo systemctl stop any2claude     # 停止

# 卸载
sudo ./deploy/install.sh --uninstall
```

### 方案 B：Docker

```bash
git clone https://github.com/houht1013/Any2Claude.git
cd Any2Claude

# 先编辑 config.json，填入你的 API Key
nano config.json

# 构建并运行
docker compose up -d

# 查看日志
docker compose logs -f

# 停止
docker compose down
```

### 方案 C：手动运行

```bash
# 编译
go build -ldflags="-s -w" -o Any2Claude ./cmd/any2claude

# 运行（监听所有网卡）
./Any2Claude -host 0.0.0.0 -port 8089

# 后台运行
nohup ./Any2Claude -host 0.0.0.0 -port 8089 > /var/log/any2claude.log 2>&1 &
```

### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-host` | 来自 config | 监听地址（`0.0.0.0` 监听所有网卡） |
| `-port` | 来自 config | 代理端口（Dashboard = port+1） |
| `-config` | 自动检测 | config.json 路径 |

### 防火墙

```bash
# 放通代理和 Dashboard 端口
sudo ufw allow 8089/tcp    # 代理
sudo ufw allow 8090/tcp    # Dashboard（生产环境建议限制访问）
```

### Claude Desktop 连接远程服务器

在 Claude Desktop 中：**Settings → Developer → Configure Third-Party Inference → Gateway**

```
http://<你的服务器 IP>:8089
```

> **安全提示：** 生产环境建议在 Any2Claude 前面加 nginx/Caddy 反向代理，配置 HTTPS 和认证。

## 端口说明

| 端口 | 服务 |
|------|------|
| 8089 | 代理（Claude Desktop 连接此端口） |
| 8090 | Dashboard（Web 管理面板） |

两个端口均可配置，Dashboard 始终为代理端口 + 1。

## 常见问题

**Q：还能用原版 Claude 模型吗？**
A：可以。没有匹配映射的模型会直通到默认供应商。如需使用原版 Claude，配置一个 Anthropic Native 格式的供应商即可。

**Q：改完配置需要重启代理吗？**
A：不需要，通过 Dashboard 修改的配置立即生效。仅端口修改需要重启。

**Q：config.json 在哪里？**
A：如果 exe 同目录存在 `config.json` 则使用它（便携模式），否则在 `%APPDATA%\Any2Claude\config.json`（Windows）。

**Q：代理运行了但 Claude Desktop 报错？**
A：在设置中开启 Debug 模式，然后查看日志标签获取详细错误信息。常见问题：API Key 错误、base_url 不正确、上游供应商拦截请求。

**Q：中国大陆编译时下载 Go 模块超时？**
A：设置 Go 代理：`go env -w GOPROXY=https://goproxy.cn,direct`，本项目零外部依赖，通常无需下载任何模块。
