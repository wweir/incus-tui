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

1. 完善各模块写操作参数校验与字段级错误提示
2. 增加 Project/Remote 动态切换
3. 引入事件订阅（monitor）与实时刷新
4. 增加 module update 行为测试与 client mock 测试
