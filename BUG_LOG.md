# IFRS 16 项目 Bug 清单

> 记录日期：2026-05-12
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

**状态**: ⚠️ 未解决
**严重程度**: 高
**发现时间**: 2026-05-12

### 问题描述
Core Service 无法从主机连接到 Docker 中的 PostgreSQL，报错：
```
FATAL: role "ifrs16" does not exist (SQLSTATE 28000)
```

### 现象
- ✅ Docker 容器内可以连接：`docker exec ifrs16-postgres psql -U ifrs16 -d ifrs16`
- ❌ 主机连接失败：`psql -h localhost -U ifrs16 -d ifrs16`
- ❌ Core Service 连接失败

### 可能原因
1. Docker 端口映射使用 IPv6 (::1) 而 PostgreSQL 只监听 IPv4
2. Docker 网络配置问题
3. PostgreSQL 的 pg_hba.conf 配置拒绝了外部连接

### 建议解决方案
1. 检查 Docker 网络配置
2. 修改 pg_hba.conf 允许 127.0.0.1/32 连接
3. 或者使用 Docker Compose 启动 Core Service（在同一网络中）

---

## 5. AI Service - 缺少 OCR 依赖

**状态**: ⚠️ 待验证
**严重程度**: 中
**发现时间**: 2026-05-12

### 问题描述
`ai-service/requirements.txt` 包含 `paddlepaddle` 和 `paddleocr`，但在 Docker 构建时可能会失败，因为：
- PaddleOCR 依赖系统库（libgl1, libglib2.0 等）
- 某些系统架构可能不支持

### 建议
在 Dockerfile 中安装系统依赖后再安装 Python 包。

---

## 6. 前端 - Next.js 版本警告

**状态**: ⚠️ 低优先级
**严重程度**: 低
**发现时间**: 2026-05-12

### 问题描述
Next.js 14.2.0 有安全漏洞警告：
```
Next.js (14.2.0) is outdated
```

### 建议
升级到 Next.js 14.2.15+ 或 15.x

---

## 待办事项

### 高优先级
- [ ] 解决 PostgreSQL 连接问题（让 Core Service 能正常连接数据库）
- [ ] 运行数据库迁移（`make migrate` 或 goose up）
- [ ] 验证完整的用户注册/登录流程

### 中优先级
- [ ] 启动 AI Service 并验证文件上传
- [ ] 测试 AI 聊天窗口和文件解析
- [ ] 验证审批流程（提交→复核→审批）

### 低优先级
- [ ] 升级 Next.js 版本
- [ ] 添加前端 API 代理配置（解决 CORS）
- [ ] 完善错误处理和加载状态

---

## 已知限制

1. **MVP 阶段数据存储**：当前部分功能使用内存存储，未完全接入数据库
2. **前端 API 调用**：需要确保前端能正确调用后端 API（当前使用硬编码 URL）
3. **Docker 网络**：各服务之间的网络通信需要验证

---

## 下次工作建议

1. 优先解决 PostgreSQL 连接问题
2. 运行 `make up` 启动全部 Docker 服务
3. 验证端到端流程：注册 → 登录 → 创建合同 → 上传文件 → AI 解析
4. 修复测试中发现的新问题
