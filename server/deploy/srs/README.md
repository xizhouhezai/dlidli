# SRS 直播服务器（M4-LIV-01 / WP-C）

DliDli 直播媒体服务器：接收 **RTMP 推流**，转出 **HLS** 与 **HTTP-FLV**，并通过
**HTTP 回调**把开播/下播事件交给后端（`:8000`）鉴权与状态联动。

```
主播 (OBS/FFmpeg) --RTMP:1935--> SRS --+-- HLS  -.m3u8/.ts  --> 8180
                                       +-- HTTP-FLV .flv    --> 8180
                                       +-- HTTP API         --> 1985
                                       \-- on_publish/on_unpublish --> 后端 :8000
```

## 1. 文件

| 文件 | 作用 |
| --- | --- |
| `srs.conf` | SRS 5 配置：端口、app、HLS、HTTP-FLV、回调、token |
| `docker-compose.srs.yaml` | 独立 compose（project `dlidli-live`），与 `../docker-compose.yaml` 互不干扰 |

## 2. 端口

| 端口 | 用途 | 暴露范围 |
| --- | :-: | --- |
| 1935 | RTMP 收流（推流） | 需对主播可达 |
| 8180 | HLS（`.m3u8`/`.ts`）与 HTTP-FLV（`.flv`） | 需对观众可达（生产走 CDN 回源） |
| 1985 | SRS HTTP API（查流/踢流） | **仅内网运维**，勿对公网开放 |

> 8180 而非常见的 8080：实测本机 8080 已有出站连接占用、且极易与外部代理/其它
> 服务冲突，故显式改到 8180（`netstat` 实测 8180 空闲，见交付证据）。

## 3. 启动与停止

在**仓库根目录**执行（compose 用 `--project-name` 由文件内 `name:` 固定为 `dlidli-live`）：

```powershell
# 启动
docker compose -f server/deploy/srs/docker-compose.srs.yaml up -d

# 状态
docker compose -f server/deploy/srs/docker-compose.srs.yaml ps

# 日志（SRS 日志走 stdout，daemon off，docker logs 直接可见）
docker compose -f server/deploy/srs/docker-compose.srs.yaml logs -f --tail 100

# 停止（保留 HLS 数据卷）
docker compose -f server/deploy/srs/docker-compose.srs.yaml down

# 停止并清空切片数据
docker compose -f server/deploy/srs/docker-compose.srs.yaml down -v
```

不用 compose 时（`docker run` 等价方式，注意加 `--add-host` 否则回调打不到宿主机）：

```powershell
docker run -d --name dlidli-srs --restart unless-stopped `
  -p 1935:1935 -p 8180:8180 -p 1985:1985 `
  -v "${PWD}\server\deploy\srs\srs.conf:/usr/local/srs/conf/srs.conf:ro" `
  -e SRS_CALLBACK_TOKEN=dev-live-cb-token `
  --add-host "host.docker.internal:host-gateway" `
  ossrs/srs:5
```

> 前置：Docker Desktop 需已启动（本机直连 Docker Hub 不通，拉镜像需先起代理，
> 见 `../README.md` §2）。首次 `up` 会拉取 `ossrs/srs:5`。

## 4. 地址口径（与后端契约一致）

| 项 | 值 |
| --- | --- |
| app | `live`（后端 `live.streamApp`） |
| 流名 | `room_<roomId>`（公开，**不含密钥**） |
| 推流 | `rtmp://<host>:1935/live/room_<roomId>?key=<streamKey>` |
| HLS | `http://<host>:8180/live/room_<roomId>.m3u8` |
| HTTP-FLV | `http://<host>:8180/live/room_<roomId>.flv` |

密钥只走推流 URL 的查询串，由 SRS 放进 `on_publish` 回调体的 `param` 字段；
若把密钥编进流名，密钥会随播放地址外泄并可被抢占推流位。

## 5. 验证命令

推流（需先在业务侧开播拿到 `streamKey`）：

```powershell
ffmpeg -re -f lavfi -i testsrc2=size=640x360:rate=25 -f lavfi -i sine=frequency=1000 `
  -c:v libx264 -preset veryfast -tune zerolatency -g 50 -b:v 800k -c:a aac -ar 44100 `
  -f flv "rtmp://127.0.0.1:1935/live/room_1?key=<streamKey>"
```

拉流预检 / 抽帧：

```powershell
# HLS：是否可解析出流信息
ffprobe -v error -show_streams -show_format "http://127.0.0.1:8180/live/room_1.m3u8"

# 播放列表内容（应随推流持续更新，含 .ts 片段）
curl.exe -s "http://127.0.0.1:8180/live/room_1.m3u8"

# HTTP-FLV
ffprobe -v error -show_streams "http://127.0.0.1:8180/live/room_1.flv"

# 抽一帧确认有画面
ffmpeg -y -i "http://127.0.0.1:8180/live/room_1.m3u8" -frames:v 1 frame.png
```

SRS API（active streams）：

```powershell
curl.exe -s "http://127.0.0.1:1985/api/v1/streams"          # 当前所有流
curl.exe -s "http://127.0.0.1:1985/api/v1/streams/1935"     # vhost 省略时按默认 vhost 查
curl.exe -s "http://127.0.0.1:1985/api/v1/clients"          # 在线客户端
```

回调闭环自查（后端日志应出现 on_publish / on_unpublish 两条请求）：

```powershell
# 开播时：应看到 POST /api/v1/live/stream/auth?token=...
# 停推后：应看到 POST /api/v1/live/stream/stop?token=...
docker logs dlidli-srs --tail 200 | Select-String 'on_publish|on_unpublish|http_hooks'
```

## 6. token 对齐（最容易出错的一步）

`SRS_CALLBACK_TOKEN` 必须与后端 `live.callbackToken` **完全一致**，否则回调被后端
拒绝（非 2xx），SRS 会直接**拒绝推流**，表现为 OBS 报连接被断开。

- 后端开发配置：`server/configs/dev.yaml` 的 `live.callbackToken`（WP-A 已落地，当前值
  `dev-live-cb-token`，可用 `DLIDLI_LIVE_CALLBACKTOKEN` 覆盖）
- 本目录默认值：`srs.conf` 中 `env SRS_CALLBACK_TOKEN=dev-live-cb-token;`（与上者一致）
- compose 覆盖：`docker-compose.srs.yaml` 的 `SRS_CALLBACK_TOKEN=${SRS_CALLBACK_TOKEN:-dev-live-cb-token}`

> ⚠️ **端口口径需与后端对齐**：`server/configs/dev.yaml` 当前 `live.playDomain` 为
> `127.0.0.1:8080`，而 SRS 的 HTTP 服务配置在本目录是 **8180**（`srs.conf` +
> compose 均为 8180）。后端下发的 `play_urls` 会按 `playDomain` 拼装，因此二者必须
> 一致，否则房间详情返回的拉流地址指向错误端口。请把 `dev.yaml` 的 `playDomain`
> 改为 `127.0.0.1:8180`（或把 SRS 配置改回 8080——但 8080 本机已占用，不推荐）。
> 该文件不在本工作包写入范围，已作为跨包问题上报。

对齐方式（三选一，务必两边同源）：

```powershell
# A) 临时：用同名环境变量起 compose（SRS 侧容器环境变量优先于 srs.conf 默认值）
$env:SRS_CALLBACK_TOKEN = '<与后端一致的值>'
docker compose -f server/deploy/srs/docker-compose.srs.yaml up -d

# B) docker run：改 -e SRS_CALLBACK_TOKEN=<值>（见 §3）

# C) 持久化：把 srs.conf 的默认值改成真实密钥（或解除文件末尾 `# env_file ./.env;`
#    注释、挂载 .env 注入）。生产环境请勿沿用 dev-live-cb-token。
```

后端侧同源注入示例：`$env:DLIDLI_LIVE_CALLBACKTOKEN='<值>'`（环境变量名以 WP-A
实现为准）。

> 若 WP-A 尚未加入 `live.callbackToken`，SRS 侧与文档暂以 `dev-live-cb-token`
> 为准；该键落地后请按上表对齐，两边任何一侧改动都必须同步另一侧。

## 7. 常见排查

| 现象 | 排查方向 |
| --- | --- |
| 推流被拒（OBS 断开） | token 不一致 → 后端返回非 2xx；看后端日志 + `docker logs dlidli-srs` |
| 回调打不到后端 | `host.docker.internal` 未解析 → 检查 compose 的 `host.docker.internal:host-gateway`；后端须监听 `0.0.0.0:8000` 而非仅 `127.0.0.1` |
| 回调 401/404 | token 不匹配 / 路径前缀不对（`/api/v1/live/stream/...`） |
| 拉流 404 | 流名必须是 `room_<roomId>`、app 必须是 `live`；确认推流已建立（§5 API） |
| HLS 起播慢 | `hls_wait_keyframe on` 需等关键帧；推流端 GOP 建议 ≤ 2s（`-g 50` @25fps） |
| 8180 打不开 | 端口未映射或宿主防火墙；`netstat -ano \| findstr 8180` |
| 首帧黑屏 | `gop_cache on` 已有缓存，长 GOP 仍会等；检查推流端编码参数 |

## 8. 已知边界与未运行验证

> **本目录的配置在编写会话中未做容器运行验证**：该会话 Docker 守护不可达
> （`npipe:////./pipe/dockerDesktopLinuxEngine` 权限拒绝，TCP 2375 关闭），
> 属已知沙箱边界。

因此本交付为**「配置 + 命令级」**：

- **已核验**：`srs.conf` 指令结构（对照 SRS 5 官方配置示例与文档）、端口 1935/8180/1985
  在本机空闲（`netstat` 实测无监听）、配置与后端回调契约的字段/路径/token 一致性
  （自查表见 `.dev-logs/liv01/wp-c.md`）。
- **未运行验证**：容器启动、SRS 是否接受该配置、RTMP 收流、HLS/FLV 实际播放、
  回调实际抵达后端、延迟实测。上述需在 Docker 可用环境执行 §5 全部命令后确认。
