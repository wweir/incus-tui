# Incus TUI Implementation Plan

## Document Links

- Architecture: `../ARCHITECTURE.md`
- Design: `./design.md`
- Docs Index: `./README.md`

## Phase 1 (Started: 2026-04-20)

### Scope
- 建立 Go 项目基础结构
- 接入 Bubble Tea 并实现实例列表界面
- 实现实例操作：启动、停止、删除
- 提供构建与质量检查入口
- 建立功能侧边栏导航
- 接入 Incus 官方 Go client
- 实现 Images/Storage/Networks/Profiles/Projects/Cluster/Operations/Warnings 列表浏览

### Status
- [x] 项目结构初始化
- [x] Core/Instances MVP 实现
- [x] 文档（ARCHITECTURE/README）补齐
- [x] 侧边栏导航实现
- [x] 默认本地连接 + 官方 Go client 接入
- [x] 其它模块列表浏览能力
- [ ] Phase 1 验收与问题清单

## Next Steps

1. [x] 完善各模块写操作参数校验与字段级错误提示（结构化表单首版）
2. [x] 增加 Project/Remote 动态切换（首版）
3. [x] 引入事件订阅（monitor）与实时刷新（首版）
4. [x] 增加非 Instances 写操作/详情路由测试与 Instances detail 测试
5. [x] 将非实例模块写操作从 `name + value` 升级为按资源类型定义字段
6. [x] 增加 Images create/update 流程，并为 Cluster/Warnings 等菜单补齐资源特定字段
7. [x] 统一列表、表单、详情、状态栏的面板式展示
8. [x] 为默认本地 `incusd` socket 增加启动前权限探测与一次性提权重启逻辑
