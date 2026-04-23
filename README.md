# incus-tui

`incus-tui` 是一个基于终端的 Incus 管理界面（TUI）。

## 当前实现范围

- 左侧功能侧边栏（Instances/Images/Storage/Networks/Profiles/Projects/Cluster/Operations/Warnings）
- 零参数默认连接本机默认 `incusd`（Unix socket）
- 当零参数连接本地 `incusd` 且命中 Unix socket 权限拒绝时，启动阶段会自动尝试使用 `sudo` 重新拉起自身
- 实例模块（Instances）
  - 列表展示（名称、状态、类型、IPv4）
  - 详情查看（覆盖式浮层 + 可滚动，覆盖状态、配置、设备、扩展配置）
  - 启动、停止、删除（含确认）
  - 创建、更新配置（覆盖式浮层表单 + 确认）
- 其余模块（Images/Storage/Networks/Profiles/Projects/Cluster/Operations/Warnings）
  - 列表浏览
  - 详情查看（覆盖式浮层 + 可滚动）
  - 刷新
  - 结构化写操作表单（覆盖式浮层 + 字段级校验与确认）
    - Images: create / update / delete
      - create: remote image pull（server/protocol/alias/local alias/public/auto update）
      - update: public / auto update / profiles
    - Storage: create / update / delete
      - create: name / driver / description
      - update: description
    - Networks: create / update / delete
      - create: name / type / description
      - update: description
    - Profiles: create / update / delete
      - create/update: description
    - Projects: create / update / delete
      - create/update: description
    - Cluster: update / delete
      - update: description / failure domain / groups / roles
    - Operations: delete
    - Warnings: update / delete
      - update: status
- 支持 `--remote`（remote 名称或 URL 端点）/`--project`/`--timeout`
- 订阅 Incus monitor 事件并自动实时刷新当前模块
- 列表保留主界面布局；详情和表单改为覆盖式浮层；确认使用居中对话框

## 快捷键

- `tab` / `shift+tab`: 在侧边栏和主显示区域之间切换焦点
- 侧边栏焦点：
  - `j/k` 或 `↑/↓`: 移动侧边栏光标，不立即切换模块
  - `enter`: 打开光标所在模块
  - `l` 或 `→`: 回到主显示区域焦点
- 主显示区域焦点：
  - `h` 或 `←`: 回到侧边栏焦点
  - `j/k` 或 `↑/↓`: 上下选择当前表格
  - `enter`: 查看当前资源详情
- 详情页：
  - 以覆盖式浮层形式打开，占据主终端窗口
  - `j/k` 或 `↑/↓`: 滚动详情
  - `pgup/pgdn`: 整页滚动
  - `g/G` 或 `home/end`: 跳到顶部/底部
  - `esc`: 返回列表
- 鼠标点击侧边栏：切换模块，并将焦点放到侧边栏
- 鼠标滚轮：滚动当前表格选择，并将焦点放到主显示区域
- 鼠标点击表格行：选择当前行，并将焦点放到主显示区域
- `r`: 刷新当前模块
- `O`: 动态切换 remote（运行中生效）
- `P`: 动态切换 project（运行中生效）
- `q`: 退出
- 非 Instances 模块：
  - `c`: 打开 create 表单
  - `u`: 打开 update 表单（默认填充选中行名称）
  - `d`: 打开 delete 表单（默认填充选中行名称）
  - `enter`: 提交表单后进入确认
  - `y/n`: 确认或取消写操作
  - update 表单中的 `keep` 表示保持当前值不变；留空仅在字段说明明确写明时表示清空
- Instances 模块额外支持：
  - `enter`: 查看实例详情
  - `c`: 创建实例（表单 + 确认）
  - `u`: 更新实例配置（表单 + 确认）
  - `s`: 启动选中实例
  - `x`: 停止选中实例
  - `d`: 删除选中实例（需确认）

## 连接说明

- 不加参数：连接默认本地 `incusd`。
- `--remote` 支持 Incus remote 名称（例如 `local`）或显式 URL（例如 `https://127.0.0.1:8443`）。
- `--project` 留空时使用服务端默认项目。
- 仅当 `--remote` 留空并且探测到本地 Unix socket 权限拒绝时，程序才会自动提权；显式远端不会触发该逻辑。

## 文档导航

- 架构文档：`ARCHITECTURE.md`
- 设计蓝图：`docs/design.md`
- 实施计划：`docs/implementation-plan.md`
- 文档索引：`docs/README.md`

## 开发

```bash
make check
make fmt
make vet
make test
make build
make package
make release
```
