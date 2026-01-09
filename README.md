# Short URL Service

高性能短網址服務，Go + Redis + PostgreSQL。

## 快速開始

```bash
# 本地開發
cp env.local .env
docker-compose -f docker-compose.local.yml up -d
```

## API

| 方法 | 路徑 | 說明 |
|------|------|------|
| POST | `/api/v1/shorten` | 創建短網址 |
| GET | `/api/v1/stats/{code}` | 查詢統計 |
| GET | `/{code}` | 重定向 |
| GET | `/health` | 健康檢查（GKE 監控用） |
| GET | `/docs/index.html` | Swagger UI（需認證） |

### Swagger UI

訪問 `/docs/index.html`，需要 Basic Auth 認證（由 `AUTH_BASIC_USER` 和 `AUTH_BASIC_PASSWORD` 設定）。

## 環境變數

| 變數 | 說明 | 預設值 |
|------|------|--------|
| `APP_ENV` | 運行環境 | production |
| `APP_BASE_URL` | 短網址基礎 URL | http://localhost |
| `POSTGRES_HOST` | PostgreSQL 主機 | localhost |
| `POSTGRES_PORT` | PostgreSQL 端口 | 5432 |
| `POSTGRES_USER` | PostgreSQL 用戶 | shorturl |
| `POSTGRES_PASSWORD` | PostgreSQL 密碼 | shorturl |
| `POSTGRES_DB` | PostgreSQL 數據庫 | shorturl |
| `POSTGRES_SSLMODE` | SSL 模式 | disable |
| `REDIS_HOST` | Redis 主機 | localhost |
| `REDIS_PORT` | Redis 端口 | 6379 |
| `REDIS_PASSWORD` | Redis 密碼 | (空) |
| `REDIS_DB` | Redis DB | 0 |
| `REDIS_POOL_SIZE` | Redis 連接池大小 | 10 |
| `RATE_LIMIT_REQUESTS` | 請求限制 | 100 |
| `RATE_LIMIT_DURATION` | 限制時間窗口 | 1m |
| `AUTH_BASIC_USER` | Swagger Basic Auth 用戶 | (必填) |
| `AUTH_BASIC_PASSWORD` | Swagger Basic Auth 密碼 | (必填) |

## GKE 部署

使用 Cloud SQL（PostgreSQL）和集群內 Redis。

### 1. 首次部署

```bash
# 使用 commit SHA 作為版本號
VERSION=$(git rev-parse --short HEAD)
IMAGE=asia-east1-docker.pkg.dev/golang-short-url-service/golang-short-url/shortener

# 打包
gcloud builds submit --tag $IMAGE:$VERSION

# 部署（先 apply，再指定版本號）
kubectl apply -f deployment.yaml
kubectl set image deployment/shortener-deploy shortener-app=$IMAGE:$VERSION
```

### 2. 更新部署

```bash
# 使用 commit SHA 作為版本號
VERSION=$(git rev-parse --short HEAD)
IMAGE=asia-east1-docker.pkg.dev/golang-short-url-service/golang-short-url/shortener

# 打包
gcloud builds submit --tag $IMAGE:$VERSION

# 更新镜像（使用版本號，會自動觸發重新部署）
kubectl set image deployment/shortener-deploy shortener-app=$IMAGE:$VERSION

# 檢查更新狀態
kubectl rollout status deployment/shortener-deploy
```

> 需要回滾時：`kubectl set image deployment/shortener-deploy shortener-app=$IMAGE:<舊版本號>`

### 3. 資料庫 Migration

```bash
# 從本地連接 Cloud SQL Private IP 執行（需要能訪問 Private IP）
psql -h <Cloud-SQL-Private-IP> -U shorturl -d shorturl -f migrations/001_init.sql

# 或使用臨時 Pod 執行（需要先安裝 postgresql-client）
kubectl run postgres-client --rm -it --image=postgres:15 --restart=Never -- \
  psql -h <Cloud-SQL-Private-IP> -U shorturl -d shorturl -f - < migrations/001_init.sql
```

---

## License

MIT
