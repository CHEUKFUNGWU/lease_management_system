.PHONY: help setup up down restart logs migrate migrate-status migrate-baseline web core ai db reset-db ifrs16-regression

help: ## 显示帮助信息
	@echo "零售经营分析工作站 — 常用命令"
	@echo ""
	@echo "  make setup      复制 .env.example 到 .env 并初始化项目"
	@echo "  make up         启动所有 Docker 服务"
	@echo "  make down       停止所有 Docker 服务"
	@echo "  make restart    重启所有服务"
	@echo "  make logs       查看所有服务日志"
	@echo "  make db         进入 PostgreSQL 命令行"
	@echo "  make migrate           应用 db/migrations/ 下尚未执行的增量迁移"
	@echo "  make migrate-status    查看已应用 / 待应用的迁移（只读）"
	@echo "  make migrate-baseline  把全部迁移标记为已应用但不执行"
	@echo "  make reset-db   删除数据库卷并重建（清空所有数据）"
	@echo "  make ifrs16-regression  运行 IFRS 16 计量回归测试并生成对数报告"
	@echo "  make web        进入前端开发容器"
	@echo "  make core       进入核心服务容器"
	@echo "  make ai         进入 AI 服务容器"
	@echo ""

setup: ## 初始化项目环境
	@if [ ! -f .env ]; then cp .env.example .env; echo "已创建 .env"; fi
	@echo "初始化完成。请编辑 .env 文件配置环境变量，然后运行 make up"

up: ## 启动所有服务
	docker-compose up -d

down: ## 停止所有服务
	docker-compose down

restart: down up ## 重启所有服务

logs: ## 查看服务日志
	docker-compose logs -f

db: ## 进入 PostgreSQL 命令行
	docker-compose exec postgres psql -U lease -d lease

migrate: ## 应用 db/migrations/ 下尚未执行的增量迁移
	@scripts/migrate.sh

migrate-status: ## 查看已应用 / 待应用的迁移，不做任何修改
	@scripts/migrate.sh --status

migrate-baseline: ## 把全部迁移标记为已应用但不执行（用于 schema 已就位的库）
	@scripts/migrate.sh --baseline

reset-db: ## 删除数据库卷并重建
	@echo "⚠️  这将删除所有数据库数据！"
	@echo "按 Ctrl+C 取消，5 秒后继续..."
	@sleep 5
	docker-compose down -v
	@echo "数据库卷已删除，运行 make up 重建..."
	docker-compose up -d postgres
	@echo "等待 PostgreSQL 启动..."
	@sleep 5
	@echo "数据库已重建完成"

web: ## 进入前端开发容器
	docker-compose exec web sh

core: ## 进入核心服务容器
	docker-compose exec core-service sh

ai: ## 进入 AI 服务容器
	docker-compose exec ai-service bash

ifrs16-regression: ## 运行 IFRS 16 计量回归测试并生成对数报告
	cd core-service && GOCACHE=$$(pwd)/.gocache go test ./internal/services/ifrs16
	cd core-service && GOCACHE=$$(pwd)/.gocache go run ./cmd/ifrs16-regression -out ../docs/IFRS16_计量回归对数报告.md
	@echo "已生成 docs/IFRS16_计量回归对数报告.md"
