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
   - 非实例模块统一表格渲染与刷新调度

3. **Domain Module (`internal/modules/instances`)**
   - 实例列表状态管理
   - 键位映射与动作分发
   - 异步命令执行（刷新、启动、停止、删除）

4. **Client Adapter (`internal/client`)**
   - 定义统一 `InstanceService` 边界
   - 基于 Incus 官方 Go client (`github.com/lxc/incus/v6/client`) 实现
   - 覆盖 Instances/Images/Storage/Networks/Profiles/Projects/Cluster/Operations/Warnings 列表能力

5. **Config (`internal/config`)**
   - 运行参数结构定义
   - 参数校验规则

## Key Data Flow

1. 用户按键触发 Bubble Tea `Update`。
2. `app.Model` 处理全局导航和刷新行为。
3. Instances 模块走 `internal/modules/instances`（含 create/update/delete 表单与确认流）；其它模块走 app 内统一表格数据管线，并共享 create/update/delete 表单与确认流。
4. `tea.Cmd` 在后台调用 `InstanceService`。
5. 结果回传为消息，更新缓存与状态栏后重新渲染。

## Design Decisions

- 连接层采用官方 Go client，避免 CLI 子进程解析。
- 零参数默认连接本地 `incusd`。
- 先完成所有设计模块的列表浏览与导航，再逐步补齐每个模块的写操作工作流。
- 所有远程调用都由 `context timeout` 约束。

## Related Documents

- `docs/design.md`
- `docs/implementation-plan.md`
- `docs/README.md`
- `README.md`
