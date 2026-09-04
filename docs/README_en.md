# doratiger-counter

English | [简体中文](../README.md)

`doratiger-counter` is a lightweight traffic counter for Hexo and other static sites. It runs as a single Go binary and stores page views (PV) and unique visitors (UV) in SQLite.

## Features

- Site and per-page PV
- Site and per-page UV
- Single-file SQLite storage
- Exact hostname checks for Origin/Referer
- TOML configuration with environment overrides
- Health endpoint, graceful shutdown, and Docker deployment
- Automated Linux amd64/arm64 and container releases

## Quick start

Download an archive from [Releases](https://github.com/DoraTiger/doratiger-counter/releases), verify it with `checksums.txt`, then run:

```bash
./doratiger-counter init-config
./doratiger-counter serve
```

The service listens on `:8080` and stores SQLite data in `data/counter.db` by default.

To build from source, install Go 1.25 or later:

```bash
git clone https://github.com/DoraTiger/doratiger-counter.git
cd doratiger-counter
make check
make build
./build/doratiger-counter serve
```

Docker deployment:

```bash
docker run -d \
  --name doratiger-counter \
  -p 8080:8080 \
  -e COUNTER_ALLOWED_ORIGINS=blog.example.com \
  -e COUNTER_ENABLE_CORS=true \
  -v doratiger-counter-data:/app/data \
  ghcr.io/doratiger/doratiger-counter:latest
```

Use an HTTPS reverse proxy in production.

## Configuration

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

TOML timeout values are Go `time.Duration` values expressed as integer nanoseconds; the example uses 10 seconds. Environment overrides are `COUNTER_ADDR`, `COUNTER_READ_TIMEOUT`, `COUNTER_WRITE_TIMEOUT`, `COUNTER_DB_DSN`, `COUNTER_SITE_KEY`, `COUNTER_ALLOWED_ORIGINS`, and `COUNTER_ENABLE_CORS`. Timeout environment values accept strings such as `10s` or `500ms`; separate multiple origins with commas.

Allowed origins are matched by normalized hostname. Root domains and subdomains must be listed separately.

## API

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

`page` is required. `uid` is optional; requests without it increment PV only. A missing page returns 400 and a rejected source returns 403.

`GET /health` returns `{"status":"ok"}` and does not require an Origin header.

## DoraTiger theme integration

```yaml
statistics:
  enable: true
  type: counter
  counter:
    api: https://counter.example.com/count
    uv: true
```

The theme creates a visitor UUID, keeps it in a one-year cookie, and sends it with the current page path. Site operators should disclose this behavior in their privacy notice.

## Data model and limitations

- PV and UV are flushed to SQLite every 30 seconds and once more during graceful shutdown. An abnormal exit may lose the most recent interval.
- Only SHA-256 digests of visitor UUIDs are persisted. These digests remain linkable pseudonymous identifiers and must not be treated as anonymous data.
- SQLite uniqueness constraints deduplicate UV, so values continue across restarts and upgrades.
- Visitor digests are loaded at startup; memory and database use grow with pages and visitors. This release targets personal sites and a single service instance.
- Origin/Referer filtering is not authentication and cannot stop forged HTTP clients.
- This release does not support multiple instances, MySQL, or PostgreSQL.

## Compatibility and upgrades

- SQLite uses monotonic schema versions. Migrations may add or transform data but must never silently delete existing statistics.
- Back up `counter.db` together with its `-wal` and `-shm` files before upgrading.
- The four `GET /count` response fields are the stable v1 contract. Removing or renaming fields, or changing their semantics, requires a new API version.
- CI migrates a legacy schema fixture and verifies continued counting and restart persistence.

## Development

```bash
make check
make build
make release
```

Tags matching `v*` publish Linux archives, checksums, and a multi-platform GHCR image. Release changes are recorded in [CHANGELOG.md](CHANGELOG.md). See [CONTRIBUTING.md](../.github/CONTRIBUTING.md) and [SECURITY.md](../.github/SECURITY.md) before contributing or reporting a vulnerability.

## License

[MIT License](../LICENSE)
