# incus-tui

`incus-tui` 是一个基于终端的 Incus 管理界面（TUI）。

## 当前实现范围

- 左侧功能侧边栏（Instances/Images/Storage/Networks/Profiles/Projects/Cluster/Operations/Warnings）
- 零参数默认连接本机默认 `incusd`（Unix socket）
- 实例模块（Instances）
  - 列表展示（名称、状态、类型、IPv4）
  - 启动、停止、删除（含确认）
- 其余模块（Images/Storage/Networks/Profiles/Projects/Cluster/Operations/Warnings）
  - 列表浏览
  - 刷新
- 支持 `--remote`（remote 名称或 URL 端点）/`--project`/`--timeout`

## 快捷键

- `h/l` 或 `←/→` 或 `tab`: 侧边栏模块切换
- `r`: 刷新当前模块
- `q`: 退出
- 非 Instances 模块：
  - `c`: 打开 create 表单
  - `u`: 打开 update 表单（默认填充选中行名称）
  - `d`: 打开 delete 表单（默认填充选中行名称）
  - `enter`: 提交表单后进入确认
  - `y/n`: 确认或取消写操作
- Instances 模块额外支持：
  - `j/k` 或 `↑/↓`: 上下选择
  - `c`: 创建实例（表单 + 确认）
  - `u`: 更新实例配置（表单 + 确认）
  - `s`: 启动选中实例
  - `x`: 停止选中实例
  - `d`: 删除选中实例（需确认）

## 连接说明

- 不加参数：连接默认本地 `incusd`。
- `--remote` 支持 Incus remote 名称（例如 `local`）或显式 URL（例如 `https://127.0.0.1:8443`）。
- `--project` 留空时使用服务端默认项目。

## 文档导航

- 架构文档：`ARCHITECTURE.md`
- 设计蓝图：`docs/design.md`
- 实施计划：`docs/implementation-plan.md`
- 文档索引：`docs/README.md`

## 开发

```bash
make fmt
make vet
make test
make build
```
