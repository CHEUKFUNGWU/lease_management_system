# IFRS 16 项目 Bug 清单

> 记录日期：2026-05-12
> 最后更新：2026-05-13
> 记录人：AI Agent

---

## 1. Web 前端 - 导入路径错误

**状态**: ✅ 已修复
**严重程度**: 高
**发现时间**: 2026-05-12

### 问题描述
app 子目录（如 `app/login/`, `app/contracts/`, `app/ai-chat/` 等）中的文件使用了错误的相对导入路径。

### 错误示例
```typescript
// 错误（在 app/login/page.tsx 中）
import AppLayout from "./components/AppLayout";

// 正确
import AppLayout from "../components/AppLayout";
```

### 受影响文件
- `app/login/page.tsx`
- `app/contracts/page.tsx`
- `app/ai-chat/page.tsx`
- `app/upload/page.tsx`
- `app/reports/page.tsx`
- `app/settings/page.tsx`
- `app/components/AppLayout.tsx`
- `app/components/ProtectedRoute.tsx`

### 修复提交
`f05f631` fix: correct import paths in web frontend

---

## 2. Core Service - 包结构错误

**状态**: ✅ 已修复
**严重程度**: 高
**发现时间**: 2026-05-12

### 问题描述
`internal/services/ifrs16.go` 文件直接放在 `services/` 目录下，但代码中导入路径期望的是 `internal/services/ifrs16/` 子目录。

### 错误信息
```
github.com/ifrs16/core-service/internal/handlers imports
	github.com/ifrs16/core-service/internal/services/ifrs16: module github.com/ifrs16/core-service/internal/services/ifrs16: git ls-remote ...
	remote: Repository not found.
```

### 修复方案
将 `internal/services/ifrs16.go` 移动到 `internal/services/ifrs16/calculation.go`

### 修复提交
`2d70376` feat: implement RBAC, approval workflow, AI assist mode

---

## 3. Core Service - Go 编译错误

**状态**: ✅ 已修复
**严重程度**: 中
**发现时间**: 2026-05-12

### 问题描述
多处 Go 编译错误：

1. **未使用的导入** (`internal/repository/role.go`):
   ```go
   "github.com/jackc/pgx/v5"  // 导入但未使用
   ```

2. **未使用的变量** (`internal/middleware/rbac.go`):
   ```go
   id, ok := userID.(string)  // id 未使用
   ```

3. **未使用的变量** (`internal/handlers/role.go`):
   ```go
   ctx := c.Request.Context()  // ctx 未使用
   ```

4. **错误的包引用** (`internal/handlers/calculation.go`):
   ```go
   payments = append(payments, ifrs16.LeasePayment{...})  // 应为 ifrs16svc
   result, err := ifrs16.Calculate(calculation)            // 应为 ifrs16svc
   ```

5. **未使用的导入** (`internal/handlers/calculation.go`):
   ```go
   "time"  // 导入但未使用
   ```

### 修复提交
`2d70376` feat: implement RBAC, approval workflow, AI assist mode

---

## 4. PostgreSQL 连接问题

**状态**: ✅ 已修复
**严重程度**: 高
**发现时间**: 2026-05-12
**修复时间**: 2026-05-13

### 问题描述
Core Service 无法从主机连接到 Docker 中的 PostgreSQL，报错：
```
FATAL: role "ifrs16" does not exist (SQLSTATE 28000)
```

### 根本原因
1. `postgres_data` 命名卷包含旧的初始化数据（可能使用不同凭据创建）
2. 迁移文件挂载到 `/docker-entrypoint-initdb.d/migrations/` 子目录，PostgreSQL 只执行 `/docker-entrypoint-initdb.d/` 根目录下的文件
3. `CREATE TYPE IF NOT EXISTS` 在 PostgreSQL 中不支持

### 修复方案
1. 创建 `db/init/01_init.sql` — 合并所有迁移的 Up 部分（无 goose 标记）
2. 将 `./db/init` 挂载到 `/docker-entrypoint-initdb.d/`（正确的初始化路径）
3. 修复 `CREATE TYPE IF NOT EXISTS` → 使用 `DO $$ ... EXCEPTION WHEN duplicate_object` 模式
4. 从 docker-compose.yml 移除过时的 `version: "3.8"` 字段
5. 添加 `make reset-db` 命令（删除卷并重建）

### 修复文件
- `db/init/01_init.sql` (新建)
- `docker-compose.yml` (修改 postgres volumes)
- `db/migrations/003_update_schema.sql` (修复 CREATE TYPE 语法)
- `Makefile` (添加 reset-db, 修复 migrate)

---

## 5. AI Service - Docker 构建失败

**状态**: ✅ 已修复
**严重程度**: 高
**发现时间**: 2026-05-13

### 问题描述
1. `ai-service/requirements.txt` 包含 `paddlepaddle` 和 `paddleocr`，在 ARM (Apple Silicon) 上无法安装
2. Dockerfile 中 `libgl1-mesa-glx` 在 Debian trixie 中已被移除
3. PaddleOCR 需要大量系统依赖，导致构建时间长、镜像体积大

### 修复方案
1. 从 `requirements.txt` 移除 PaddlePaddle/PaddleOCR，创建 `requirements-ocr.txt` 作为可选依赖
2. 添加 `app/services/ocr.py` — 带有优雅降级的 OCR 工具（检测 PaddleOCR 是否可用）
3. 更新 Dockerfile 移除不需要的系统依赖（`libgl1-mesa-glx` 等）

### 修复文件
- `ai-service/requirements.txt` (移除 PaddleOCR)
- `ai-service/requirements-ocr.txt` (新建，可选 OCR 依赖)
- `ai-service/Dockerfile` (精简系统依赖)
- `ai-service/app/services/ocr.py` (新建，可选 OCR 模块)

---

## 6. 前端 - Next.js 版本警告

**状态**: ✅ 已修复
**严重程度**: 低
**发现时间**: 2026-05-12

### 问题描述
Next.js 14.2.0 有安全漏洞警告。

### 修复方案
升级 Next.js 14.2.0 → 14.2.21，eslint-config-next 同步升级。

### 修复文件
- `web/package.json` (升级版本号)
- `web/package-lock.json` (npm install 更新)

---

## 7. Docker Compose - 卷挂载覆盖构建产物

**状态**: ✅ 已修复
**严重程度**: 高
**发现时间**: 2026-05-13

### 问题描述
docker-compose.yml 中 core-service、ai-service、web 服务的 `volumes: ./xxx:/app` 挂载会覆盖 Docker 镜像中的构建产物（编译后的二进制文件、node_modules 等），导致 `exec format error` 或找不到文件。

### 修复方案
移除所有服务的源码卷挂载，使用 Docker 镜像中的构建产物。

### 修复文件
- `docker-compose.yml` (移除 volumes 和 working_dir)

---

## 8. Core Service Dockerfile - Go 版本不匹配

**状态**: ✅ 已修复
**严重程度**: 高
**发现时间**: 2026-05-13

### 问题描述
Dockerfile 使用 `golang:1.22-alpine`，但 `go.mod` 要求 `go 1.23`，导致构建失败。

### 修复方案
更新 Dockerfile 为 `golang:1.23-alpine`。

### 修复文件
- `core-service/Dockerfile` (更新 Go 版本)

---

## 已验证的服务状态

| 服务 | 端口 | 状态 |
|------|------|------|
| PostgreSQL | 5432 | ✅ Running, Healthy, 17 tables initialized |
| MinIO | 9000/9001 | ✅ Running, Healthy |
| Core Service | 8080 | ✅ Connected to DB, `/health` OK |
| AI Service | 8081 | ✅ Running, DeepSeek provider configured |
| Web Frontend | 3000 | ✅ Running, Next.js 14.2.21 |

---

## 下次工作建议

1. 端到端测试：注册 → 登录 → 创建合同 → 提交审批 → IFRS 16 计算
2. 验证 AI 聊天窗口和文件上传
3. 验证审批流程（提交→复核→审批）
4. 测试 Working Report / Official Report 双模式
5. 完善 API 错误处理和前端加载状态
