# incus-tui Architecture

## System Boundary

`incus-tui` 是一个本地终端应用，职责是将 Incus 的核心管理能力通过键盘驱动交互呈现。

- 输入边界：用户键盘输入、终端尺寸变化。
- 输出边界：终端渲染内容、对 Incus daemon 的 API 调用。
- 外部依赖：本机 `incusd`（默认 Unix socket）或远程 HTTPS API 端点。

## Layered Responsibilities

1. **Entry (`cmd/incus-tui`)**
   - 解析参数
   - 校验配置
   - 注入运行时依赖
   - 启动 Bubble Tea 程序

2. **UI Orchestration (`internal/app`)**
   - 全局退出热键
   - 侧边栏模块导航
   - 聚合模块更新与渲染

3. **Domain Module (`internal/modules/instances`)**
   - 实例列表状态管理
   - 键位映射与动作分发
   - 异步命令执行（刷新、启动、停止、删除）

4. **Client Adapter (`internal/client`)**
   - 定义 `InstanceService` 接口
   - 基于 Incus 官方 Go client (`github.com/lxc/incus/v6/client`) 实现调用
   - 默认连接本地 Unix socket，可选连接远程 HTTPS 端点

5. **Config (`internal/config`)**
   - 运行参数结构定义
   - 参数校验规则

## Key Data Flow

1. 用户按键触发 Bubble Tea `Update`。
2. `app.Model` 处理全局导航并路由到当前模块。
3. `instances.Model` 根据按键产生 `tea.Cmd`。
4. `tea.Cmd` 在后台调用 `InstanceService`。
5. 结果回传为消息（成功/失败）。
6. 模型更新状态后重新渲染侧边栏与主内容区。

## Design Decisions

- 连接层从 CLI 子进程迁移为官方 Go client，减少输出解析与进程开销。
- 零参数默认连接本地 `incusd`，与常见 Incus 本地使用习惯一致。
- 侧边栏先提供完整模块入口，未实现模块显示占位页，避免后续重构导航结构。
- 所有远程 API 调用由 `context timeout` 约束。

## Related Documents

- `Incus TUI Design.md`
- `docs/implementation-plan.md`
- `README.md`
