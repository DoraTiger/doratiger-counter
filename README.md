# doratiger-counter

[English](docs/README_en.md) | 简体中文

`doratiger-counter` 是一个面向 Hexo 等静态站点的轻量级访问统计服务。它以单个 Go 二进制运行，使用 SQLite 保存页面浏览量（PV）和独立访客数（UV）。

## 功能

- 站点 PV 与页面 PV
- 站点 UV 与页面 UV
- SQLite 单文件存储
- Origin/Referer 来源限制
- TOML 配置与环境变量覆盖
- 健康检查、优雅退出和 Docker 部署
- Linux amd64/arm64 自动构建与容器镜像发布

## 快速开始

### 二进制运行

从 [Releases](https://github.com/DoraTiger/doratiger-counter/releases) 下载对应架构的压缩包，校验 `checksums.txt` 后解压：

```bash
./doratiger-counter init-config
./doratiger-counter serve
```

默认监听 `:8080`，SQLite 数据写入 `data/counter.db`。

### 从源码构建

需要 Go 1.25 或更高版本：

```bash
git clone https://github.com/DoraTiger/doratiger-counter.git
cd doratiger-counter
make check
make build
./build/doratiger-counter serve
```

### Docker

```bash
docker run -d \
  --name doratiger-counter \
  -p 8080:8080 \
  -e COUNTER_ALLOWED_ORIGINS=blog.example.com \
  -e COUNTER_ENABLE_CORS=true \
  -v doratiger-counter-data:/app/data \
  ghcr.io/doratiger/doratiger-counter:latest
```

生产环境建议只通过反向代理暴露服务，并为 API 配置 HTTPS。

## 配置

默认配置文件为 `config.toml`：

```toml
[server]
addr = ':8080'
read_timeout = 10000000000
write_timeout = 10000000000

[database]
dsn = 'data/counter.db'

[counter]
site_key = 'dtc_site'
allowed_origins = ['blog.example.com']
enable_cors = true
```

`read_timeout` 和 `write_timeout` 使用 Go `time.Duration` 的纳秒整数表示；上例均为 10 秒。

| 配置 | 环境变量 | 默认值 |
| --- | --- | --- |
| `server.addr` | `COUNTER_ADDR` | `:8080` |
| `server.read_timeout` | `COUNTER_READ_TIMEOUT` | `10s` |
| `server.write_timeout` | `COUNTER_WRITE_TIMEOUT` | `10s` |
| `database.dsn` | `COUNTER_DB_DSN` | `data/counter.db` |
| `counter.site_key` | `COUNTER_SITE_KEY` | `dtc_site` |
| `counter.allowed_origins` | `COUNTER_ALLOWED_ORIGINS` | 空列表 |
| `counter.enable_cors` | `COUNTER_ENABLE_CORS` | `false` |

超时环境变量接受 `10s`、`500ms` 等 Go duration 字符串。多个允许来源使用逗号分隔。允许列表按规范化后的主机名精确匹配；如需同时允许根域名和子域名，必须分别列出。

## API

### 计数

```http
GET /count?page=/posts/example/&uid=visitor-id
Origin: https://blog.example.com
```

```json
{
  "site_pv": 42,
  "page_pv": 3,
  "site_uv": 12,
  "page_uv": 2
}
```

- `page` 必填，是页面路径或调用方定义的页面键。
- `uid` 可选；为空时只增加 PV。
- 配置允许来源后，服务优先校验 `Origin`，缺失时才使用 `Referer`。
- 缺少 `page` 返回 400，来源不允许返回 403。

### 健康检查

```http
GET /health
```

返回 `{"status":"ok"}`。健康检查不要求 Origin。

## DoraTiger 主题接入

在博客的 `_config.hexo-theme-doratiger.yml` 中启用：

```yaml
statistics:
  enable: true
  type: counter
  counter:
    api: https://counter.example.com/count
    uv: true
```

主题在浏览器中生成访问者 UUID，通过 Cookie 保留一年，并把页面路径和 UUID 发送给计数 API。部署方应在隐私说明中披露这一用途。

## 数据与限制

- PV 和 UV 在内存中累计，每 30 秒写入 SQLite，正常退出时会执行最后一次同步写入。
- 异常退出可能丢失最近一个刷新周期内的 PV 和 UV。
- 服务只持久化访问者 UUID 的 SHA-256 摘要，不保存原始 UUID；摘要仍属于可关联的假名标识，不应当作匿名数据。
- UV 通过 SQLite 唯一约束去重，服务重启和版本升级后继续累计。
- 启动时会把访客摘要载入内存，内存与数据库会随页面和访客数量增长；当前版本面向个人站点和单实例部署。
- Origin/Referer 限制只能减少普通跨站调用，不是身份认证，也不能阻止伪造 HTTP 请求。
- 当前版本不支持多实例共享计数，也不支持 MySQL 或 PostgreSQL。

## 兼容性与升级

- SQLite 使用单调递增的 schema 版本；迁移只允许增加或转换数据，不得静默删除已有统计。
- 启动新版本前仍建议备份 `counter.db` 及其 `-wal`、`-shm` 文件。
- `GET /count` 的四个响应字段构成 v1 稳定契约。删除字段、重命名字段或改变计数语义必须使用新的 API 版本。
- CI 使用旧版 schema 夹具验证迁移、继续计数及重启后的数据连续性。

## 开发与发布

```bash
make check      # 格式、vet、竞态测试
make build      # 当前平台二进制
make release    # Linux amd64/arm64 发行包及校验和
```

推送 `v*` 标签后，GitHub Actions 会创建发行包并发布多架构 GHCR 镜像。版本变化记录在 [CHANGELOG](docs/CHANGELOG.md)。参与开发前请阅读 [贡献指南](.github/CONTRIBUTING.md)，安全问题请参阅 [安全策略](.github/SECURITY.md)。

## 许可证

[MIT License](LICENSE)
