# incus-tui Architecture

## System Boundary

`incus-tui` 是一个本地终端应用，职责是将 Incus 的核心管理能力通过键盘驱动交互呈现。

- 输入边界：用户键盘输入、终端尺寸变化。
- 输出边界：终端渲染内容、对 `incus` CLI 的命令调用。
- 外部依赖：本机 `incus` 二进制与已配置的认证上下文。

## Layered Responsibilities

1. **Entry (`cmd/incus-tui`)**
   - 解析参数
   - 校验配置
   - 注入运行时依赖
   - 启动 Bubble Tea 程序

2. **UI Orchestration (`internal/app`)**
   - 全局退出热键
   - 聚合模块更新与渲染

3. **Domain Module (`internal/modules/instances`)**
   - 实例列表状态管理
   - 键位映射与动作分发
   - 异步命令执行（刷新、启动、停止、删除）

4. **Client Adapter (`internal/client`)**
   - 定义 `InstanceService` 接口
   - 以 `incus` CLI 为后端实现调用
   - 统一错误包装与输出脱敏

5. **Config (`internal/config`)**
   - 运行参数结构定义
   - 参数校验规则

## Key Data Flow

1. 用户按键触发 Bubble Tea `Update`。
2. `instances.Model` 根据按键产生 `tea.Cmd`。
3. `tea.Cmd` 在后台调用 `InstanceService`。
4. 结果回传为消息（成功/失败）。
5. 模型更新状态后重新渲染表格与状态栏。

## Design Decisions

- 当前阶段采用 `incus` CLI 适配器实现，降低集成复杂度。
- 通过接口隔离后续替换为 Incus Go client 的成本。
- 所有外部命令调用必须受 `context timeout` 约束。
- MVP 聚焦实例管理，其他模块按设计文档分阶段扩展。

## Related Documents

- `Incus TUI Design.md`
- `docs/implementation-plan.md`
- `README.md`
