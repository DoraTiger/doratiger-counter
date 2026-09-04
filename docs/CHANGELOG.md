# 更新日志（Changelog）

本项目遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## [0.1.0] - 待发布

### 新增

- SQLite 单实例 PV/UV 统计服务。
- 站点与页面四字段 v1 API 契约。
- 基于 SHA-256 访问者摘要的持久化 UV 去重。
- 版本化 SQLite schema 和旧版 PV 数据迁移测试。
- Origin/Referer 精确主机名限制和按来源 CORS。
- Linux amd64/arm64 发行包、SHA256 清单和 GHCR 镜像自动发布。

### 修正

- 正常退出时等待最后一次 SQLite 刷新，重复停止不再 panic。
- 数据库结构损坏时拒绝启动，避免以空值覆盖历史统计。
- HTTP 服务正确应用读写超时和优雅退出。
