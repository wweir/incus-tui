# Incus TUI Implementation Plan

## Phase 1 (Started: 2026-04-20)

### Scope
- 建立 Go 项目基础结构
- 接入 Bubble Tea 并实现实例列表界面
- 实现实例操作：启动、停止、删除
- 提供构建与质量检查入口
- 建立功能侧边栏导航骨架
- 将实例管理能力切换到 Incus 官方 Go client

### Status
- [x] 项目结构初始化
- [x] Core/Instances MVP 实现
- [x] 文档（ARCHITECTURE/README）补齐
- [x] 侧边栏导航骨架实现
- [x] 默认本地连接 + 官方 Go client 接入
- [ ] Phase 1 验收与问题清单

## Next Steps

1. 补齐非 Instances 模块（Images/Storage/Networks）最小可用列表页
2. 支持 Incus remote 名称解析（兼容 `incus remote list`）
3. 引入更细粒度测试（module update + client mock）
