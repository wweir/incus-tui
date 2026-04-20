# Incus TUI Implementation Plan

## Phase 1 (Started: 2026-04-20)

### Scope
- 建立 Go 项目基础结构
- 接入 Bubble Tea 并实现实例列表界面
- 实现实例操作：启动、停止、删除
- 提供构建与质量检查入口

### Status
- [x] 项目结构初始化
- [x] Core/Instances MVP 实现
- [x] 文档（ARCHITECTURE/README）补齐
- [ ] Phase 1 验收与问题清单

## Next Steps

1. 扩展 Core 状态（remote/project 动态切换）
2. 增加 Images/Storage/Network 模块骨架
3. 引入更细粒度测试（client mock + module update）
