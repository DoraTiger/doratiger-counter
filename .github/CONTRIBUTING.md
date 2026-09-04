# 贡献指南

感谢你对 `doratiger-counter` 的关注。

## 开始之前

- 当前稳定范围是 Go 1.25、SQLite 和单实例部署。
- 行为变更应先通过 Issue 说明使用场景和兼容性影响。
- 不要提交真实站点日志、SQLite 数据库、访问者 UUID、凭据或私有基础设施地址。

## 本地开发

```bash
git clone https://github.com/DoraTiger/doratiger-counter.git
cd doratiger-counter
make check
make build
```

修复缺陷或增加行为时，请先加入能够复现问题的测试。提交 Pull Request 前必须保证：

```bash
make check
git diff --check
```

## Pull Request

- 每个 Pull Request 聚焦一个问题。
- 说明变更动机、用户可见行为和验证方式。
- 配置或 API 变化必须同步更新中英文 README。
- 新依赖应说明必要性，并保持单二进制、无 CGO 的构建目标。

所有贡献均按项目的 [MIT License](../LICENSE) 发布。
