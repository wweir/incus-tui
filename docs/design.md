**Incus TUI 设计文档**

**版本**：1.0  
**日期**：2026 年 4 月  
**作者**：Grok（基于官方文档与社区分析）  
**目标**：分析现有 Incus Web UI 特性，评估为 Incus 开发一套 TUI（Text User Interface，终端用户界面）的可行性，并基于 Incus CLI 的完整能力规划模块、架构与实现方案，形成可落地文档。

### 1. Incus Web UI 分析

Incus **上游官方不提供原生 Web UI**（区别于 LXD）。Incus 设计为“可插拔”：daemon 可直接从 `/opt/incus/ui` 目录静态服务任意前端（纯 HTML + JS 应用，无状态）。

#### 当前主流 Web UI
- **incus-ui-canonical**（Zabbly 维护，https://github.com/zabbly/incus-ui-canonical）：  
  最常用方案，是 Canonical LXD-UI 的 fork（持续 rebase）。通过 `apt install incus-ui-canonical`（Zabbly 仓库）安装后，Incus daemon 自动提供该 UI。
- 访问方式：  
  `incus config set core.https_address :8443`（或 `127.0.0.1:8443`）  
  然后用 `incus webui` 命令或直接浏览器打开 `https://<ip>:8443`。  
  需客户端证书认证（生成 `incus-ui.crt` + `incus-ui.pfx` 并导入浏览器与 Incus trust store）。

#### Web UI 核心特性（继承自 LXD-UI）
- **实例管理**：列表、创建（容器/VM）、启动/停止/重启/删除、终端（console）、图形控制台（VM）、快照、迁移。
- **存储管理**：Storage Pools 与 Volumes 的 CRUD、浏览。
- **网络管理**：Networks、Network ACLs 配置。
- **配置管理**：Profiles、Projects、自定义字段。
- **集群与权限**：Cluster groups、用户组、权限分配。
- **运维监控**：Operations 列表、Warnings、Server Settings。
- **其他**：图像列表、导入/发布等。
- **优点**：图形化、易用、无需 CLI 知识，支持大规模私有云。
- **局限**：依赖浏览器证书认证；非官方维护（fork）；某些 Incus 新特性可能滞后。

**总结**：Web UI 是“CLI 功能的图形封装”，高度依赖 Incus REST API。TUI 的目标是提供**终端原生、键盘友好、无浏览器依赖** 的等价体验，尤其适合服务器/无头环境。

### 2. 为 Incus 开发 TUI 的必要性与可行性分析

**为什么需要 TUI？**
- CLI 强大但操作繁琐（`incus list` + `incus exec` + 管道等）。
- Web UI 需要浏览器 + 证书，不适合纯终端用户（SSH、无 GUI 服务器）。
- TUI 可实现“键盘驱动 + 实时交互 + 表格/树形视图”，结合 CLI 的全部能力，提供类似 `htop` / `lazygit` / `k9s` 的沉浸式体验。
- 适用于运维、开发、边缘部署场景。

**技术可行性**
- Incus 所有功能均可通过 **CLI** 或 **REST API** 驱动（CLI 本身就是 REST API 的 Go 客户端）。
- 现有 CLI 子命令已覆盖 100% 核心能力（见下文）。
- TUI 可选择：
  - **推荐**：直接调用 Incus Go 客户端库（incus 源码中的 `client` 包），性能最佳、无解析开销。
  - 备选：子进程调用 `incus` CLI（简单但需解析输出）。
  - 认证：支持 Unix socket（本地）与 HTTPS + 证书（远程），与 CLI 完全一致。

### 3. 基于 Incus CLI 能力的模块规划

Incus CLI 顶级子命令（官方 manpage 完整列表）：

| 类别 | 主要子命令 | 核心功能 |
|------|------------|----------|
| **实例管理** | `list`, `launch`, `create`, `start`, `stop`, `restart`, `pause`, `resume`, `delete`, `exec`, `console`, `copy`, `move`, `rename`, `rebuild`, `snapshot`, `top`, `wait` | 实例生命周期、终端、文件传输、快照 |
| **镜像管理** | `image`, `publish`, `import` | 镜像列表、导入、发布 |
| **存储管理** | `storage` | Pools、Volumes、Snapshots |
| **网络管理** | `network` | Networks、ACLs、Forwardings |
| **配置管理** | `profile`, `project`, `config` | Profiles、Projects、实例/服务器配置 |
| **集群管理** | `cluster` | Cluster 成员、Groups |
| **运维监控** | `operation`, `monitor`, `warning`, `info`, `debug` | 操作列表、实时监控、警告 |
| **远程与别名** | `remote`, `alias` | 多远程管理、自定义别名 |
| **其他** | `export`, `import`, `file`, `query`, `version`, `webui`, `manpage`, `admin` | 备份/导入、文件操作、API 查询 |

**TUI 模块划分**（建议采用领域驱动设计，每模块独立目录）：

1. **Core（核心）**  
   - 连接管理（本地 socket / 远程 HTTPS + cert）  
   - 认证、Remote 切换、Project 切换  
   - 全局配置、帮助、退出

2. **Instances（实例）**  
   - 表格列表（支持过滤、排序、列自定义，如 CLI `-c`）  
   - 操作栏：Start/Stop/Restart/Delete/Exec/Console/Snapshot  
   - 详情面板（状态、配置、日志）  
   - 快速启动向导（类似 `incus launch`）

3. **Images（镜像）**  
   - 远程/本地镜像列表  
   - 搜索、导入、发布、删除

4. **Storage（存储）**  
   - Pools 列表 + Volumes 树形视图  
   - 创建/编辑/删除/快照

5. **Networks（网络）**  
   - Networks + ACLs 管理

6. **Profiles & Projects（配置）**  
   - Profiles 列表/编辑  
   - Projects 切换与管理

7. **Cluster（集群）**  
   - 成员列表、Groups、操作（仅在集群模式显示）

8. **Operations & Monitoring（运维）**  
   - 实时 Operations 列表  
   - Warnings  
   - `top` 风格的资源监控（CPU/Memory/Network）

9. **Utils（工具）**  
   - 别名管理  
   - 直接输入 CLI 命令模式（fallback）  
   - 导出/导入向导

每个模块均支持**键盘快捷键**（类似 vim/k9s：j/k 上下、Enter 详情、/ 搜索、? 帮助）。

### 4. 架构设计

**整体架构（推荐）**：

```
TUI App (Go + Bubble Tea)
├── Client Layer          ← 复用 Incus 官方 client 包（或自建轻量 wrapper）
├── State / Store         ← 全局状态（当前 Remote、Project、选中实例等）
├── UI Components        ← Bubble Tea Model + Bubbles（Table、Modal、TextInput、Spinner 等）
├── Modules (Domain)     ← 每个模块一个 Model + View + Commands
├── API / CLI Fallback   ← 优先 API，降级调用 incus CLI
└── Renderer             ← Lip Gloss 样式（支持主题、边框、颜色）
```

**关键设计决策**：
- **语言**：Go（与 Incus 同源，可直接 vendor/incus 源码中的 client 包，避免重复认证逻辑）。
- **TUI 框架**：**Bubble Tea**（Charmbracelet）+ **Bubbles** + **Lip Gloss**  
  （现代、响应式、支持异步、易扩展；替代方案：tview / gocui）。
- **数据流**：
  - 异步查询（`tea.Cmd`）避免阻塞 UI。
  - 实时更新：使用 `incus monitor` 或 WebSocket-like polling。
- **认证与安全**：完全复用 Incus CLI 逻辑（Unix socket 优先，HTTPS + TLS 证书）。
- **可扩展性**：插件式模块（每个模块实现相同 Interface）。
- **无头支持**：纯 TUI，可通过 `incus-tui` 二进制直接运行。
- **性能**：列表分页（CLI 支持 limit），大集群场景下延迟加载。

**与 Web UI 的映射**：
- Web UI 的“实例列表 + 操作栏” → TUI 表格 + 底部/侧边快捷键栏。
- Web UI 的“模态窗口” → TUI Modal + Form。
- Web UI 的“终端” → TUI 内置 pseudo-terminal（支持 `incus exec` 交互）。

### 5. 实现方案与路线图

**Phase 1：MVP（核心实例管理）**（2-4 周）
- Core + Instances 模块
- 列表、启动/停止/删除、Exec/Console
- 本地 socket 连接

**Phase 2：完整功能覆盖**（4-6 周）
- 补齐 Images/Storage/Networks/Profiles 等模块
- 集群、监控支持

**Phase 3：打磨与发布**（2 周）
- 主题、配置、帮助系统
- 打包（deb/rpm/静态二进制）
- 文档 & 示例

**技术栈推荐**：
```bash
go install github.com/charmbracelet/bubbletea@latest
go install github.com/charmbracelet/bubbles@latest
go install github.com/charmbracelet/lipgloss@latest
```

**代码结构建议**：
```
incus-tui/
├── cmd/incus-tui/
├── internal/
│   ├── client/          # Incus API wrapper
│   ├── modules/         # instances, storage, ...
│   ├── ui/              # components, styles
│   ├── state/           # store
│   └── config/
└── go.mod
```

**潜在挑战与解决方案**：
- 复杂表单 → 用 Bubbles Form 或自定义多字段输入。
- 终端大小自适应 → Bubble Tea 内置 Resize 事件。
- 权限控制 → 复用 Incus 证书/ACL 机制。
- 测试 → 集成 Incus 测试容器 + Go test。

**开源建议**：放置于 GitHub（lxc/incus 组织下或独立 repo），遵循 Incus 贡献流程。欢迎社区贡献模块。

此文档可作为开发蓝图直接落地。如需原型代码、详细 API 调用示例或特定模块的伪代码，请进一步说明，我可以立即提供。
