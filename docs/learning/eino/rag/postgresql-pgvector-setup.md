# PostgreSQL + pgvector 本地安装

## 目标

使用 Docker 启动一个独立的 PostgreSQL 16 + pgvector 实例，为后续把 `MemoryVectorStore` 迁移为持久化 Store 做准备。

完成本文后应满足：

- PostgreSQL 容器处于健康状态。
- 数据目录使用独立 Docker Volume，重启容器后数据仍然存在。
- `vector` 扩展已经启用。
- 可以创建向量字段并执行余弦距离查询。

当前 `examples/rag-minimal/` 尚未连接 PostgreSQL。完成安装只代表外部依赖准备完成，不代表 Store 迁移已经完成。

## 版本与资源约定

| 项目 | 值 |
|---|---|
| Docker 镜像 | `pgvector/pgvector:pg16` |
| 容器名 | `eino-lab-pgvector` |
| Docker Volume | `eino-lab-pgvector-data` |
| PostgreSQL 容器端口 | `5432` |
| 本机监听地址 | `127.0.0.1:15432` |
| 数据库 | `eino_rag` |
| 用户 | `eino_rag` |

使用本机端口 `15432`，避免与已经存在的 PostgreSQL `5432` 冲突。端口只绑定到 `127.0.0.1`，不向局域网暴露。

2026-07-23 本机检查：Docker CLI `29.5.2` 和 Docker Compose `v5.1.3` 已安装，但 Docker Engine 尚未启动，`docker info` 无法连接用户目录下的 Docker socket。执行安装前需要先启动 Docker Desktop。

## 1. 检查 Docker

执行：

```bash
docker --version
docker info
```

预期：两个命令都成功，`docker info` 能显示正在运行的 Docker Engine 信息。如果 `docker info` 无法连接 daemon，先启动 Docker Desktop。

确认容器名和端口没有被占用：

```bash
docker ps -a --filter name=eino-lab-pgvector
lsof -nP -iTCP:15432 -sTCP:LISTEN
```

预期：第一次安装时没有同名容器，端口检查没有输出。如果同名容器已经存在，不要重复创建，先执行本文“日常启停与状态检查”中的检查命令。

## 2. 拉取镜像

```bash
docker pull pgvector/pgvector:pg16
```

查看本机镜像信息：

```bash
docker image inspect pgvector/pgvector:pg16 \
  --format 'image={{.Id}} architecture={{.Architecture}}'
```

预期：输出镜像 ID 和当前机器对应的架构。

## 3. 创建持久卷

```bash
docker volume create eino-lab-pgvector-data
docker volume inspect eino-lab-pgvector-data
```

预期：`docker volume inspect` 能找到 `eino-lab-pgvector-data`。

## 4. 启动 PostgreSQL + pgvector

在当前 zsh 终端中读取数据库密码。输入时终端不会回显密码：

```bash
read -s "RAG_PGVECTOR_PASSWORD?请输入 eino_rag 数据库密码: "
echo
```

启动容器：

```bash
docker run -d \
  --name eino-lab-pgvector \
  --restart unless-stopped \
  --health-cmd='pg_isready -U eino_rag -d eino_rag' \
  --health-interval=5s \
  --health-timeout=5s \
  --health-retries=12 \
  -e POSTGRES_USER=eino_rag \
  -e POSTGRES_PASSWORD="$RAG_PGVECTOR_PASSWORD" \
  -e POSTGRES_DB=eino_rag \
  -p 127.0.0.1:15432:5432 \
  -v eino-lab-pgvector-data:/var/lib/postgresql/data \
  pgvector/pgvector:pg16
```

清除当前终端中的临时密码变量：

```bash
unset RAG_PGVECTOR_PASSWORD
```

说明：密码不会被写入仓库，但 Docker 会将容器环境变量保存在本机容器配置中。本方式只适合本地学习环境；生产部署应使用平台的 Secret 管理能力。

## 5. 检查健康状态

```bash
docker ps --filter name=eino-lab-pgvector
docker inspect eino-lab-pgvector --format '{{.State.Health.Status}}'
```

预期：容器状态最终变为 `healthy`。如果仍是 `starting`，等待几秒后重新执行第二条命令。

如果状态变为 `unhealthy` 或容器退出，查看日志：

```bash
docker logs --tail 100 eino-lab-pgvector
```

## 6. 启用 vector 扩展

```bash
docker exec eino-lab-pgvector \
  psql -v ON_ERROR_STOP=1 -U eino_rag -d eino_rag \
  -c 'CREATE EXTENSION IF NOT EXISTS vector;'
```

查询扩展版本：

```bash
docker exec eino-lab-pgvector \
  psql -v ON_ERROR_STOP=1 -U eino_rag -d eino_rag \
  -c "SELECT extname, extversion FROM pg_extension WHERE extname = 'vector';"
```

预期：返回一行 `vector` 及其版本号。版本号由当前镜像实际包含的 pgvector 版本决定。

## 7. 执行向量查询冒烟

下面的命令只创建临时表，连接结束后自动删除，不会留下业务表：

```bash
docker exec eino-lab-pgvector \
  psql -v ON_ERROR_STOP=1 -U eino_rag -d eino_rag \
  -c "CREATE TEMP TABLE vector_smoke (id integer PRIMARY KEY, embedding vector(3)); INSERT INTO vector_smoke (id, embedding) VALUES (1, '[1,0,0]'), (2, '[0,1,0]'); SELECT id, embedding <=> '[1,0,0]'::vector AS cosine_distance FROM vector_smoke ORDER BY cosine_distance;"
```

预期结果：

```text
 id | cosine_distance
----+-----------------
  1 |               0
  2 |               1
```

`<=>` 表示余弦距离，距离越小表示向量方向越接近。

## 8. 验证容器重启后的持久化

重启容器：

```bash
docker restart eino-lab-pgvector
```

重新检查健康状态和扩展：

```bash
docker inspect eino-lab-pgvector --format '{{.State.Health.Status}}'
docker exec eino-lab-pgvector \
  psql -v ON_ERROR_STOP=1 -U eino_rag -d eino_rag \
  -c "SELECT extname, extversion FROM pg_extension WHERE extname = 'vector';"
```

预期：容器最终恢复为 `healthy`，`vector` 扩展仍然存在。这证明数据库目录已经挂载到持久卷。临时冒烟表不会保留，这是 PostgreSQL 临时表的预期行为，不代表持久化失败。

## 9. 准备项目环境变量

仓库根目录的 `.env` 已被 `.gitignore` 忽略。为后续 Store 迁移准备以下变量，密码填写第 4 步设置的真实值：

```dotenv
RAG_PG_HOST=127.0.0.1
RAG_PG_PORT=15432
RAG_PG_USER=eino_rag
RAG_PG_PASSWORD=请填写本地数据库密码
RAG_PG_DATABASE=eino_rag
RAG_PG_SSLMODE=disable
```

当前 Go 示例尚未读取这些变量。接入代码完成前，不要把“变量已配置”当作“应用已连接数据库”。不得提交 `.env` 或真实密码。

## 日常启停与状态检查

停止容器，但保留容器和数据：

```bash
docker stop eino-lab-pgvector
```

重新启动：

```bash
docker start eino-lab-pgvector
```

检查状态：

```bash
docker ps -a --filter name=eino-lab-pgvector
docker inspect eino-lab-pgvector --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}'
```

进入 `psql`：

```bash
docker exec -it eino-lab-pgvector psql -U eino_rag -d eino_rag
```

在 `psql` 中执行 `\q` 退出。

## 常见故障

### 容器名冲突

先确认现有容器是否就是本文创建的实例：

```bash
docker ps -a --filter name=eino-lab-pgvector
docker inspect eino-lab-pgvector --format '{{.Config.Image}} {{range .Mounts}}{{.Name}}:{{.Destination}} {{end}}'
```

如果镜像和 Volume 符合本文约定，执行 `docker start eino-lab-pgvector`，不要再次运行 `docker run`。

### 本机端口 15432 被占用

```bash
lsof -nP -iTCP:15432 -sTCP:LISTEN
```

选择其他未占用端口，例如把启动命令中的：

```text
127.0.0.1:15432:5432
```

改为：

```text
127.0.0.1:25432:5432
```

同时将 `.env` 中的 `RAG_PG_PORT` 改为相同端口。

### vector 扩展不存在

确认容器使用的不是普通 `postgres` 镜像：

```bash
docker inspect eino-lab-pgvector --format '{{.Config.Image}}'
```

预期为 `pgvector/pgvector:pg16`。普通 `postgres:16` 镜像默认不包含 pgvector 扩展。

### 修改密码后仍无法登录

`POSTGRES_PASSWORD` 只在空数据目录首次初始化时生效。已经存在的 Volume 不会因为重新设置容器环境变量而自动修改数据库密码。需要保留数据时，应进入 PostgreSQL 使用 `ALTER ROLE` 修改密码，不要删除 Volume。

## 清理说明

停止容器不会删除数据。只有确认不再需要本地知识库数据时，才执行以下清理操作：

```bash
docker stop eino-lab-pgvector
docker rm eino-lab-pgvector
docker volume rm eino-lab-pgvector-data
```

其中 `docker volume rm eino-lab-pgvector-data` 会永久删除 PostgreSQL 数据，无法通过重新启动容器恢复。执行前必须确认没有需要保留的数据。

## 安装完成后的反馈信息

完成后提供以下三条命令的输出即可，不要提供密码或完整 `.env`：

```bash
docker ps --filter name=eino-lab-pgvector
docker inspect eino-lab-pgvector --format '{{.State.Health.Status}}'
docker exec eino-lab-pgvector \
  psql -U eino_rag -d eino_rag \
  -c "SELECT extname, extversion FROM pg_extension WHERE extname = 'vector';"
```

安装验证通过后，下一步才是设计 PostgreSQL 表结构，并预测把 `MemoryVectorStore` 替换为 pgvector Store 会影响哪些文件和运行链路。

## 官方参考

- [pgvector 源码与使用说明](https://github.com/pgvector/pgvector)
- [pgvector Docker 镜像](https://hub.docker.com/r/pgvector/pgvector)
