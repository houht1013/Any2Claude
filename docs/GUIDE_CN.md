# Any2Claude 使用指南

> 从安装到使用的完整流程。

---

## 快速开始

### 1. 启动

**Windows：** 双击 `Any2Claude.exe`，托盘出现图标，右键可打开 Dashboard。

**macOS / Linux：** 终端运行 `./Any2Claude`，浏览器打开 `http://127.0.0.1:8090`。

### 2. Dashboard

打开 Dashboard 后看到管理面板：

![Dashboard 主界面](images/dashboard.png)
<!-- 截图：Dashboard 完整页面，Guide 标签页，显示统计卡片 + How It Works 流程图 + Quick Start 步骤 -->

面板包含 5 个标签页：**使用指南** / **模型** / **供应商** / **日志** / **设置**

### 3. 配置流程

1. **供应商标签** → 点击 `+ Add Provider` → 填入 API 地址和 Key → 保存
2. **模型标签** → 点击 `+ Add Model` → 填入 Model ID（`claude-` 开头）和实际模型名 → 保存
3. **Claude Desktop** → Settings → Developer → Configure Third-Party Inference → Gateway 填入 `http://127.0.0.1:8089` → 重启

完成后，你配置的模型将出现在 Claude Desktop 的模型选择器中。

### 4. 调试

遇到问题时，在 **设置标签** 开启 Debug Mode，然后查看 **日志标签** 的详细请求/响应信息。

---

详细文档见 [README_CN.md](../README_CN.md) | [README.md](../README.md)
