# Team-API Docker 部署指南

本文档介绍如何使用 Docker 部署 Team-API。

## 快速开始

### 一、准备配置文件

```bash
# 1. 进入 Docker 配置目录
cd manifest/docker

# 2. 复制配置模板
cp ../../manifest/config/config.example.yaml ./config.yaml

# 3. 编辑配置文件，修改数据库和 Redis 连接信息
vim config.yaml
```

**注意**：本配置不对外开放 PostgreSQL 和 Redis 端口，仅应用服务（18888 端口）对外暴露。如需外部访问数据库，请自行添加端口映射。


### 二、修改 docker-compose.yaml

编辑 `docker-compose.yaml` 文件，修改以下配置：

1. **数据库密码**（第 23 行）：
   ```yaml
   POSTGRES_PASSWORD: team_api_secret  # 修改为强密码
   ```

2. **Redis 密码**（第 45 行）：
   ```yaml
   command: redis-server --appendonly yes --requirepass redis_secret  # 修改为强密码
   ```

3. **管理员账号**（第 76-77 行，可选）：
   ```yaml
   INIT_ADMIN_USERNAME: admin          # 修改为你的管理员用户名
   INIT_ADMIN_PASSWORD: admin123       # 修改为你的管理员密码
   ```

### 三、修改 config.yaml

编辑 `config.yaml` 文件，确保数据库和 Redis 配置与 docker-compose.yaml 一致：

```yaml
database:
  default:
    type: "pgsql"
    link: "pgsql:team_api:team_api_secret@tcp(postgres:5432)/team_api?sslmode=disable"
                        # ^用户名    ^密码              ^主机名:端口    ^数据库名

redis:
  default:
    address: "redis:6379"  # 注意：主机名是 "redis" 而不是 localhost
    pass: "redis_secret"   # 密码需与 docker-compose.yaml 中设置的一致
    db: 0
```

### 四、启动服务

```bash
# 构建并启动所有服务
docker compose up -d

# 查看日志
docker compose logs -f app
```

### 五、访问应用

启动成功后，可以访问：

- **管理后台**: http://localhost:18888/admin
- **租户控制台**: http://localhost:18888
- **API 文档**: http://localhost:18888/api/docs

## 配置说明

### docker-compose.yaml 配置项

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `POSTGRES_PASSWORD` | 数据库密码 | `team_api_secret` |
| `--requirepass` | Redis 密码 | `redis_secret` |
| `INIT_ADMIN_USERNAME` | 管理员用户名 | `admin` |
| `INIT_ADMIN_PASSWORD` | 管理员密码 | `admin123` |

### config.yaml 配置项

#### 数据库配置

```yaml
database:
  default:
    link: "pgsql:用户名:密码@tcp(主机名:端口)/数据库名?sslmode=disable"
```

**注意**：Docker 网络中，主机名使用服务名（`postgres`、`redis`），而不是 `localhost`。

#### Redis 配置

```yaml
redis:
  default:
    address: "redis:6379"  # 主机名使用 "redis"
    db: 0
```

#### JWT 密钥（重要）

```yaml
jwt:
  secret: "change-me-to-a-random-secret"  # 生产环境必须修改
```

## 服务说明

Docker Compose 包含以下服务：

| 服务 | 容器名 | 端口 | 说明 |
|------|--------|------|------|
| `postgres` | `team-api-postgres` | 5432 | PostgreSQL 数据库 |
| `redis` | `team-api-redis` | 6379 | Redis 缓存 |
| `app` | `team-api-app` | 18888 | Team-API 应用 |

## 常用命令

```bash
# 启动服务
docker compose up -d

# 停止服务
docker compose down

# 查看日志
docker compose logs -f

# 仅查看应用日志
docker compose logs -f app

# 重启服务
docker compose restart app

# 重新构建镜像
docker compose build --no-cache
docker compose up -d

# 查看服务状态
docker compose ps
```

## 数据持久化

以下数据通过 Docker 卷持久化：

| 卷名 | 用途 |
|------|------|
| `postgres_data` | 数据库数据 |
| `redis_data` | Redis 数据 |
| `app_logs` | 应用日志 |

## 清理数据

```bash
# 停止并删除容器、网络
docker compose down

# 删除数据卷（注意：会删除所有数据！）
docker volume rm team-api-postgres_data team-api-redis_data team-api-app_logs
```

## 生产环境建议

### 1. 修改默认密码

务必修改以下密码：
- `docker-compose.yaml` 中的 `POSTGRES_PASSWORD`
- `docker-compose.yaml` 中的 `INIT_ADMIN_PASSWORD`
- `config.yaml` 中的 `jwt.secret`

### 2. 使用外部数据库

对于生产环境，建议使用托管数据库服务。修改 `config.yaml` 中的数据库连接信息即可。

### 3. 配置 HTTPS

使用反向代理（如 Nginx）配置 HTTPS：

```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:18888;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket 支持
    location /api/admin/ws {
        proxy_pass http://localhost:18888;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## 故障排查

### 应用无法启动

1. 检查日志：`docker compose logs app`
2. 确认 `config.yaml` 已创建且配置正确
3. 确认数据库和 Redis 就绪：`docker compose ps`

### 数据库连接失败

1. 确认 `config.yaml` 中的密码与 `docker-compose.yaml` 中的 `POSTGRES_PASSWORD` 一致
2. 确认主机名使用的是 `postgres` 而非 `localhost`
3. 查看数据库日志：`docker compose logs postgres`

### 无法访问应用

1. 确认端口未被占用：`lsof -i :18888`
2. 检查防火墙设置
3. 验证容器状态：`docker compose ps`

## 更新应用

```bash
# 拉取最新代码
cd ../../
git pull

# 重新构建并启动
cd manifest/docker
docker compose build
docker compose up -d
```

## 使用 Make 快捷命令

```bash
# 构建镜像
make docker-build

# 启动服务
make docker-up

# 停止服务
make docker-down

# 查看日志
make docker-logs

# 重新构建
make docker-rebuild

# 清理数据
make docker-clean
```
