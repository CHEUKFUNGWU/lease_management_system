.PHONY: help setup up down restart logs migrate web core ai db

help: ## 显示帮助信息
	@echo "IFRS 16 租赁管理系统 — 常用命令"
	@echo ""
	@echo "  make setup    复制 .env.example 到 .env 并初始化项目"
	@echo "  make up       启动所有 Docker 服务"
	@echo "  make down     停止所有 Docker 服务"
	@echo "  make restart  重启所有服务"
	@echo "  make logs     查看所有服务日志"
	@echo "  make db       进入 PostgreSQL 命令行"
	@echo "  make migrate  执行数据库迁移"
	@echo "  make web      进入前端开发容器"
	@echo "  make core     进入核心服务容器"
	@echo "  make ai       进入 AI 服务容器"
	@echo ""

setup: ## 初始化项目环境
	@if [ ! -f .env ]; then cp .env.example .env; echo "已创建 .env"; fi
	@echo "初始化完成。请编辑 .env 文件配置环境变量，然后运行 make up"

up: ## 启动所有服务	docker-compose up -d

down: ## 停止所有服务
	docker-compose down

restart: down up ## 重启所有服务

logs: ## 查看服务日志
	docker-compose logs -f

db: ## 进入 PostgreSQL 命令行
	docker-compose exec postgres psql -U ifrs16 -d ifrs16

migrate: ## 执行数据库迁移
	@echo "执行数据库迁移..."
	cd db && goose postgres "host=localhost port=5432 user=ifrs16 password=ifrs16_secret dbname=ifrs16 sslmode=disable" up

web: ## 进入前端开发容器
	docker-compose exec web sh

core: ## 进入核心服务容器
	docker-compose exec core-service sh

ai: ## 进入 AI 服务容器
	docker-compose exec ai-service bash
