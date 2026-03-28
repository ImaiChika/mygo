# mygo

`mygo` 是一个面向实时聊天与未来协作编辑场景的全栈工程。当前版本已经包含可直接使用的 Go 后端和 Vue 3 + Vite 前端：

- 用户注册 / JWT 登录
- 实时聊天：基于 WebSocket 的双向消息推送
- 会话成员权限：支持 owner / admin / member
- 已读未读：支持按会话记录阅读位置与未读数
- 附件上传：支持本地存储接口，后续可平滑切换对象存储
- Vue 3 + Vite 前端：支持登录、建会话、搜索用户、拉成员、实时聊天与上传附件
- 多实例扩展：基于 Redis Pub/Sub 的跨节点消息广播
- 协作编辑预留：预留 Yjs 协作房间接入能力
- 数据持久化：基于 PostgreSQL 保存会话与消息
- 服务治理扩展：预留服务注册接口，后续可接 Consul 或 Kubernetes Service

## 目录说明

```text
cmd/server             程序入口
frontend               Vue 3 + Vite 前端
internal/app           应用装配与生命周期
internal/config        配置加载
internal/modules       业务模块（chat / collab / system）
internal/platform      基础设施（db / redis / ws / eventbus / auth）
deployments            本地开发环境示例
migrations             数据库初始化脚本
docs                   架构说明
```

## 本地启动

1. 复制环境变量：

```bash
cp .env.example .env
```

2. 推荐直接启动完整容器栈：

```bash
docker compose -f deployments/docker-compose.yml up -d --build
```

启动后访问：

- 前端：http://127.0.0.1:5173
- 后端：http://127.0.0.1:8080

如果你想查看容器状态：

```bash
docker compose -f deployments/docker-compose.yml ps
```

如果你想看日志：

```bash
docker compose -f deployments/docker-compose.yml logs -f
docker compose -f deployments/docker-compose.yml logs -f frontend
docker compose -f deployments/docker-compose.yml logs -f app
```

如果你想关闭容器但保留数据：

```bash
docker compose -f deployments/docker-compose.yml down
```

如果你想彻底清空容器和数据库卷：

```bash
docker compose -f deployments/docker-compose.yml down -v
```

如果你想本地运行 Go 二进制，也可以只启动 PostgreSQL 与 Redis，并使用相同的环境变量。

3. 如果数据库不是第一次初始化，请手动补跑迁移：

```bash
psql postgres://mygo:mygo@127.0.0.1:5432/mygo -f migrations/000001_init.sql
psql postgres://mygo:mygo@127.0.0.1:5432/mygo -f migrations/000002_feature_extensions.sql
```

4. 启动后端服务：

```bash
go run ./cmd/server
```

5. 如果你本机安装了 Node.js 20+，也可以单独启动前端开发模式：

```bash
cd frontend
npm install
npm run dev
```

## 当前提供的接口

- `GET /api/v1/health/liveness`
- `GET /api/v1/health/readiness`
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/me`
- `GET /api/v1/users/search?q=关键字`
- `GET /api/v1/conversations`
- `POST /api/v1/conversations`
- `GET /api/v1/conversations/{conversationID}/members`
- `POST /api/v1/conversations/{conversationID}/members`
- `GET /api/v1/conversations/{conversationID}/messages`
- `POST /api/v1/conversations/{conversationID}/messages`
- `POST /api/v1/conversations/{conversationID}/read`
- `POST /api/v1/conversations/{conversationID}/attachments`
- `GET /api/v1/ws`
- `POST /api/v1/collab/sessions`
- `GET /api/v1/collab/rooms/{roomID}`
- `GET /uploads/{storageKey}`

说明：

- 浏览器默认从前端容器的 `5173` 端口访问应用
- 前端通过 Nginx 反代 `/api/v1` 和 `/uploads`，所以浏览器不需要额外配置接口地址
- HTTP 接口统一使用 `Authorization: Bearer <token>`
- WebSocket 可通过 `Authorization` 请求头或 `?access_token=` 查询参数携带 JWT
- 当前附件存储为本地目录 `uploads/`，默认上传上限为 `10MB`，后续可替换为 S3 / OSS / MinIO

## 架构决策

更完整的设计说明见 [docs/architecture.md](/Users/imaichika/Documents/mygo/docs/architecture.md)。
