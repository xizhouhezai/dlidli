# SRS DVR 录制目录（M4-LIV-14）
#
# 本目录由 docker-compose.srs.yaml 以 ./dvr:/data/dvr 挂载进 SRS 容器，
# 容器内 dvr_path = /data/dvr/[app]/[stream].[timestamp].flv（见 srs.conf）。
# 后端（跑在宿主）通过 live.replay.dvrDir 指向本目录读取录制文件，故两侧必须一致。
#
# 为什么用 bind mount 而不是命名卷：命名卷仅容器可见，后端每次都得 docker cp 取回，
# 回放的自动化链路就断了。
#
# 运行时产出的 *.flv 不入库（见 .gitignore），本文件只用于保住目录本身。
