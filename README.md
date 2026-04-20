# incus-tui

`incus-tui` 是一个基于终端的 Incus 管理界面（TUI）。

## 当前实现范围（MVP）

- 左侧功能侧边栏（Instances/Images/Storage/Networks/Profiles/Projects/Cluster/Operations）
- Instances 模块：实例列表展示（名称、状态、类型、IPv4）
- 实例操作快捷键：启动、停止、删除（含确认）
- 刷新与状态提示
- 默认零参数连接本机默认 `incusd`（Unix socket）
- 支持 `--remote`（URL 端点）/`--project`/`--timeout`

## 快捷键

- `h/l` 或 `←/→` 或 `tab`: 在侧边栏模块间切换
- `j/k` 或 `↑/↓`: 上下选择（Instances 模块）
- `r`: 刷新
- `s`: 启动选中实例
- `x`: 停止选中实例
- `d`: 删除选中实例（需确认）
- `q`: 退出

## 连接说明

- 不加参数：连接默认本地 `incusd`（等价于 Incus 常见本地使用方式）。
- `--remote` 当前仅支持显式 URL（示例：`https://127.0.0.1:8443`）。
- `--project` 留空时使用服务端默认项目。

## 开发

```bash
make fmt
make vet
make test
make build
```
