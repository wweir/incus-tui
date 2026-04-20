# incus-tui

`incus-tui` 是一个基于终端的 Incus 管理界面（TUI）。

## 当前实现范围（MVP）

- 实例列表展示（名称、状态、类型、IPv4）
- 实例操作快捷键：启动、停止、删除
- 刷新与状态提示
- 支持 `--remote`、`--project`、`--timeout`

## 快捷键

- `j/k` 或 `↑/↓`: 上下选择
- `r`: 刷新
- `s`: 启动选中实例
- `x`: 停止选中实例
- `d`: 删除选中实例（需确认）
- `q`: 退出

## 开发

```bash
make fmt
make vet
make test
make build
```
