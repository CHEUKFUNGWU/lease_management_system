from fastapi import FastAPI, UploadFile, File, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from contextlib import asynccontextmanager

from app.config import get_settings
from app.services.llm import llm_client
from app.routers import files, parse, chat


@asynccontextmanager
async def lifespan(app: FastAPI):
    """应用生命周期管理"""
    # 启动时检查配置
    settings = get_settings()
    print(f"AI Service starting with provider: {settings.llm_provider}")
    print(f"Model: {llm_client.get_model_name()}")
    yield
    # 关闭时清理
    print("AI Service shutting down")


app = FastAPI(
    title="IFRS 16 AI Service",
    description="AI document parsing and field extraction service",
    version="0.1.0",
    lifespan=lifespan
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 注册路由
app.include_router(files.router, prefix="/api/v1", tags=["files"])
app.include_router(parse.router, prefix="/api/v1", tags=["parse"])
app.include_router(chat.router, prefix="/api/v1", tags=["chat"])


@app.get("/health")
async def health_check():
    return {
        "status": "ok",
        "service": "ai-service",
        "provider": get_settings().llm_provider,
        "model": llm_client.get_model_name()
    }
