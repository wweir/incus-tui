# incus-tui Architecture

## System Boundary

`incus-tui` 是一个本地终端应用，职责是将 Incus 的核心管理能力通过键盘和鼠标驱动交互呈现。

- 输入边界：用户键盘输入、鼠标点击/滚轮、终端尺寸变化。
- 输出边界：终端渲染内容、对 Incus daemon 的 API 调用。
- 外部依赖：本机 `incusd`（默认 Unix socket）或远程 HTTPS API 端点。

## Layered Responsibilities

1. **Entry (`cmd/incus-tui`)**
   - 解析参数
   - 校验配置
   - 注入运行时依赖
   - 在启动前探测本地 `incusd` Unix socket 可访问性，必要时执行一次提权重启
   - 启动 Bubble Tea 程序

2. **UI Orchestration (`internal/app`)**
   - 全局退出热键
   - 侧边栏与主显示区域焦点管理
   - 侧边栏模块导航，光标移动与模块激活分离
   - 非实例模块统一表格渲染、覆盖式详情/表单浮层、结构化 CRUD 表单与刷新调度
   - 统一维护表格展示值与真实资源标识映射，避免截断 ID 影响写操作与详情查询
   - 统一管理状态栏/帮助区/详情面板的语义化展示

3. **Domain Module (`internal/modules/instances`)**
   - 实例列表状态管理
   - 键位映射与动作分发
   - 异步命令执行（刷新、详情、创建、更新、启动、停止、删除）
   - 实例独立表单/确认渲染
   - 复用共享详情视图组件处理紧凑布局与滚动
   - 复用共享覆盖层渲染器输出全屏详情/表单浮层与小型确认对话框

4. **Client Adapter (`internal/client`)**
   - 定义统一 `InstanceService` 边界
   - 基于 Incus 官方 Go client (`github.com/lxc/incus/v6/client`) 实现
   - 覆盖 Instances/Images/Storage/Networks/Profiles/Projects/Cluster/Operations/Warnings 列表与详情能力
   - 为非实例模块提供按资源类型解析的结构化 create/update/delete 适配

5. **Shared TUI Components (`internal/tui`)**
   - `tableinput`: 鼠标滚轮/点击到表格游标变更的适配
   - `detailview`: 共享详情页渲染、viewport 滚动与窗口尺寸同步
   - `overlay`: 共享全屏浮层与居中对话框渲染

6. **Config (`internal/config`)**
   - 运行参数结构定义
   - 参数校验规则

## Key Data Flow

1. 用户按键触发 Bubble Tea `Update`。
2. `app.Model` 处理全局导航、焦点切换和刷新行为；正常浏览态下 `Tab` 在侧边栏与主显示区域之间切换焦点。
3. 鼠标事件由入口开启后进入 `app.Model`，侧边栏点击在 app 层切换模块并聚焦侧边栏，表格点击/滚轮聚焦主显示区域并通过 `internal/tui/tableinput` 转换为选中行变化。
4. Instances 模块走 `internal/modules/instances`（含 create/update/delete 流）；其它模块走 app 内统一表格数据管线。
5. 两类详情页都通过 `internal/tui/detailview` 生成紧凑文本，并交给 viewport 提供键盘/鼠标滚动。
6. 详情页与表单页不再占用主内容区，而是通过 `internal/tui/overlay` 以覆盖式浮层渲染；确认交互保持小型对话框。
7. 非实例模块的写操作不再使用单一 `value` 字段，而是由菜单项定义表单字段，再转换成结构化 `ResourceValues` 传给 client adapter。
8. Project/Remote 支持运行时动态切换，切换成功后重建服务连接并刷新当前模块。
9. 通过 Incus 事件订阅（monitor）监听 lifecycle/operation 事件，触发当前模块实时刷新。
10. `tea.Cmd` 在后台调用 `InstanceService`。
11. 结果回传为消息，更新缓存与状态栏后重新渲染。

## Design Decisions

- 连接层采用官方 Go client，避免 CLI 子进程解析。
- 零参数默认连接本地 `incusd`。
- 对零参数本地连接增加启动前访问探测；若命中 Unix socket 权限拒绝，则只尝试一次 `sudo` 提权重启，避免循环。
- 先完成所有设计模块的列表浏览与导航，再逐步补齐每个模块的写操作工作流。
- 侧边栏导航采用明确焦点模型：移动侧边栏光标不触发加载，只有确认激活模块时才刷新或应用缓存。
- app 层临时交互态使用单一 mode 表达，避免表单、确认框、Remote/Project 输入出现重叠状态。
- 非实例模块表格允许截断展示长标识，但写操作与详情查询始终使用缓存中的完整资源键。
- 非实例模块表单采用“按资源定义字段”的方式，避免把不同资源强行压扁成 `name + value` 二元组。
- 详情页不再按“每字段一个边框块”渲染，而是统一走共享紧凑布局；长内容必须可滚动，不能依赖终端自然截断。
- 深阅读/深编辑动作不再与列表共用一个内容区；详情和表单统一使用覆盖式浮层，确认保持小型居中对话框。
- 对 update 流程引入 `keep` 语义，允许用户只修改指定字段而不误清空未知当前值。
- 所有远程调用都由 `context timeout` 约束。

## Related Documents

- `docs/design.md`
- `docs/implementation-plan.md`
- `docs/README.md`
- `README.md`
