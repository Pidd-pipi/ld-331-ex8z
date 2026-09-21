# 智排（GBSCHED）医院智能排班系统

> **Docker Compose 一键启动**（首次启动前）：
>
> ```bash
> cp .env.example .env
> docker compose up -d --build
> ```
>
> 停止并清理测试数据：`docker compose down --volumes --remove-orphans`

智排是一套面向医院科室的全栈排班管理系统。它通过规则引擎生成周/月排班，提供日历式排班查看与手动微调、调班审批留痕、人员工时统计以及节假日策略配置，帮助主管降低人工协调成本并避免明显排班冲突。

## 主要功能

- **科室与岗位**：维护科室、岗位编制与技能标签；内置内科、主治医师和演示人员。
- **规则排班**：配置连续工作天数、周末轮循、节假日优先、夜班限制；一键生成指定日期范围的班表。
- **可视化班表**：按科室与日期筛选，白班/中班/夜班/休息采用不同颜色；下拉调整即为手动微调。
- **调班与替班**：员工提交双方班次的调换申请，主管审批时在一个数据库事务中交换排班，记录申请人、审批人与状态。
- **工时统计与导出**：按月汇总人员出勤、各类班次和夜班加班时长，可从前端下载统计表。
- **特殊日期**：维护法定节假日、特殊工作日和计班系数；在规则管理页查看。
- **安全与可观测性**：JWT 登录、管理员/主管/员工角色权限、统一 JSON 响应、请求 ID 结构化日志、健康检查。

## 访问地址与演示账号

| 服务 | 地址 |
|---|---|
| 前端 | http://localhost:18931 |
| 后端 API | http://localhost:19931/api/v1 |
| 健康检查 | http://localhost:19931/healthz |
| OpenAPI 摘要 | `backend/api/openapi.yaml` |

初始账号的密码均为 `admin123`：`admin`（管理员）、`supervisor`（主管）、`doctor`（员工）、`nurse`（员工）。

## 技术栈

| 层级 | 技术 |
|---|---|
| 前端 | Vue 3、TypeScript、Vite、Element Plus |
| 后端 | **Go 1.22 + Gin + GORM** |
| 数据库 | MySQL 8.0（本地开发可回退 SQLite） |
| 认证 | JWT (`github.com/golang-jwt/jwt/v5`) + RBAC |
| 导出 | Excelize（服务端依赖）/ 浏览器下载 |
| 部署 | Docker Compose、Nginx 多阶段构建 |

## 本地开发

### 后端

默认使用 SQLite 文件 `backend/scheduler.db`，无需先启动 MySQL：

```bash
cd backend
go mod tidy
go run ./cmd/server
# 构建与测试
go build ./...
go test ./...
```

如需连接 MySQL，设置 `DB_DRIVER=mysql`，并配置 `DB_HOST`、`DB_PORT`、`DB_NAME`、`DB_USER`、`DB_PASSWORD`。后端默认端口为 `19931`，使用 Docker 时为容器内 `8080`。

### 前端

```bash
cd frontend
npm install
npm run dev
# 类型检查与生产构建
npm run check
npm run build
```

Vite 开发服务器使用 `18931`，会将 `/api` 代理到本地 `19931` 后端。

## Docker 部署说明

1. `cp .env.example .env` 后按实际环境修改数据库密码与 `JWT_SECRET`。
2. 在项目根目录执行 `docker compose up -d --build`。
3. Compose 使用项目名 `gbsched`，启动 MySQL 健康检查后才启动后端；前端等待后端健康检查通过。
4. 前端 Nginx 将 `/api/` 请求转发给 Docker 网络内的 `backend:8080`，因此不依赖主机 `localhost`。
5. 数据库数据保存在命名卷 `db_data`，需要重置数据时执行 `docker compose down --volumes`。

## 环境变量

| 变量 | 说明 | 默认/示例 |
|---|---|---|
| `COMPOSE_PROJECT_NAME` | Compose 项目和容器名前缀 | `gbsched` |
| `DB_NAME` | MySQL 数据库名 | `hospital_scheduler` |
| `DB_USER` / `DB_PASSWORD` | 应用数据库账户 | `scheduler` / `scheduler_pass` |
| `DB_ROOT_PASSWORD` | MySQL root 密码 | `root_pass` |
| `JWT_SECRET` | JWT 签名密钥，生产环境请替换 | `local-development-secret-change-me` |
| `FRONTEND_PORT` | 前端宿主机端口 | `18931` |
| `BACKEND_PORT` | 后端宿主机端口 | `19931` |
| `DB_DRIVER` | 后端数据库驱动（本地使用） | `sqlite` 或 `mysql` |

## 项目结构

```text
.
├── frontend/                 # Vue 3 前端、页面、路由、API 与 Nginx
├── backend/
│   ├── cmd/server/           # 应用装配与启动入口
│   ├── internal/
│   │   ├── model/            # Department、Staff、Schedule 等实体
│   │   ├── repository/       # 数据访问层
│   │   ├── service/          # 排班/调班/统计业务层
│   │   ├── handler/          # HTTP 控制器
│   │   ├── router/           # API 路由与权限分组
│   │   ├── middleware/       # JWT、请求日志
│   │   ├── dto/              # 请求 DTO 与校验规则
│   │   └── config/           # 环境配置
│   ├── migrations/           # 迁移说明
│   └── api/openapi.yaml      # OpenAPI 摘要
├── database/init.sql         # MySQL 初始化入口
├── docker-compose.yml
└── .env.example
```

## API 概览

所有业务接口采用 `{ "code": 0, "message": "ok", "data": ... }` 结构，除登录外要求 `Authorization: Bearer <token>`。

- `POST /api/v1/auth/login`：登录获取 JWT。
- `GET/POST /api/v1/departments`、`GET/POST /api/v1/positions`、`GET/POST /api/v1/staff`：组织人员管理。
- `GET /api/v1/schedules`、`POST /api/v1/schedules/generate`、`PUT /api/v1/schedules/:id`：查询、生成和调整排班。
- `GET/PUT /api/v1/schedule-rules`、`GET/POST /api/v1/holidays`：排班规则与特殊日期。
- `GET/POST /api/v1/shift-requests`、`PUT /api/v1/shift-requests/:id/review`：调班申请与审批。
- `GET /api/v1/schedules/statistics`：月度出勤和工时统计。

## License

MIT License。此项目用于演示医院排班业务流程，不应作为生产医疗系统直接使用；正式部署前需补充数据加密、细粒度审计、真实考勤/短信等系统集成和安全评估。
