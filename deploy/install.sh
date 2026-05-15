#!/bin/bash
set -e

# ============================================================
#  Any2Claude — Linux Server Install Script
# ============================================================
#
#  Usage:
#    sudo ./install.sh              # Install + enable service
#    sudo ./install.sh --uninstall  # Remove everything
#
#  What it does:
#    1. Builds the binary (requires Go)
#    2. Creates /opt/any2claude/ with binary + config
#    3. Creates any2claude system user
#    4. Installs systemd service
#    5. Starts and enables the service
#
# ============================================================

INSTALL_DIR="/opt/any2claude"
SERVICE_NAME="any2claude"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
USER_NAME="any2claude"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# --- Uninstall ---
if [ "$1" = "--uninstall" ]; then
    info "Uninstalling Any2Claude..."
    systemctl stop ${SERVICE_NAME} 2>/dev/null || true
    systemctl disable ${SERVICE_NAME} 2>/dev/null || true
    rm -f ${SERVICE_FILE}
    systemctl daemon-reload
    # Keep /opt/any2claude (user may want config)
    info "Service removed. Config preserved at ${INSTALL_DIR}/config.json"
    info "To fully remove: rm -rf ${INSTALL_DIR} && userdel ${USER_NAME}"
    exit 0
fi

# --- Pre-checks ---
if [ "$(id -u)" -ne 0 ]; then
    error "Please run as root: sudo ./install.sh"
fi

# Check Go
if ! command -v go &> /dev/null; then
    error "Go not found. Install it first:\n  Ubuntu/Debian: sudo apt install golang-go\n  CentOS/Fedora: sudo dnf install golang\n  Or download: https://go.dev/dl/"
fi
info "Go found: $(go version | awk '{print $3}')"

# --- Build ---
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
info "Building Any2Claude..."
cd "${SCRIPT_DIR}/.."
go build -ldflags="-s -w" -o Any2Claude ./cmd/any2claude
info "Build success: $(ls -lh Any2Claude | awk '{print $5}')"

# --- Install ---
info "Installing to ${INSTALL_DIR}..."
mkdir -p ${INSTALL_DIR}

# Copy binary
cp -f Any2Claude ${INSTALL_DIR}/Any2Claude
chmod 755 ${INSTALL_DIR}/Any2Claude

# Copy default config if not present (don't overwrite existing)
if [ ! -f ${INSTALL_DIR}/config.json ]; then
    cp cmd/any2claude/embed/config.json ${INSTALL_DIR}/config.json
    info "Default config written to ${INSTALL_DIR}/config.json"
    info "  >>> Edit ${INSTALL_DIR}/config.json to set your API keys! <<<"
else
    warn "Config exists at ${INSTALL_DIR}/config.json — not overwritten"
fi

# --- Create user ---
if ! id "${USER_NAME}" &>/dev/null; then
    useradd --system --no-create-home --shell /usr/sbin/nologin ${USER_NAME}
    info "Created system user: ${USER_NAME}"
fi
chown -R ${USER_NAME}:${USER_NAME} ${INSTALL_DIR}

# --- Install systemd service ---
info "Installing systemd service..."
cp deploy/any2claude.service ${SERVICE_FILE}
systemctl daemon-reload
systemctl enable ${SERVICE_NAME}
systemctl start ${SERVICE_NAME}

# --- Done ---
echo ""
echo -e "${GREEN}============================================================${NC}"
echo -e "${GREEN}  Any2Claude installed successfully!${NC}"
echo -e "${GREEN}============================================================${NC}"
echo ""
echo "  Binary:    ${INSTALL_DIR}/Any2Claude"
echo "  Config:    ${INSTALL_DIR}/config.json"
echo "  Service:   ${SERVICE_NAME}"
echo ""
echo "  Proxy:     http://0.0.0.0:8089"
echo "  Dashboard: http://0.0.0.0:8090"
echo ""
echo "  Commands:"
echo "    systemctl status  ${SERVICE_NAME}   # Check status"
echo "    systemctl restart ${SERVICE_NAME}   # Restart"
echo "    systemctl stop    ${SERVICE_NAME}   # Stop"
echo "    journalctl -u     ${SERVICE_NAME}   # View logs"
echo ""
echo "  Edit config:"
echo "    sudo nano ${INSTALL_DIR}/config.json"
echo "    sudo systemctl restart ${SERVICE_NAME}"
echo ""
echo "  Configure Claude Desktop:"
echo "    Settings → Developer → Configure Third-Party Inference"
echo "    Gateway URL: http://<your-server-ip>:8089"
echo ""
echo "  Uninstall:"
echo "    sudo ./install.sh --uninstall"
echo ""
