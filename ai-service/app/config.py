from pydantic_settings import BaseSettings
from functools import lru_cache


class Settings(BaseSettings):
    """AI Service 配置"""
    
    # LLM 提供商配置
    llm_provider: str = "deepseek"
    
    # DeepSeek 配置
    deepseek_api_key: str = ""
    deepseek_base_url: str = "https://api.deepseek.com"
    deepseek_model: str = "deepseek-v4-flash"
    
    # OpenAI 配置（备用）
    openai_api_key: str = ""
    openai_base_url: str = "https://api.openai.com/v1"
    openai_model: str = "gpt-4o"
    
    # Core Service 地址
    core_service_url: str = "http://core-service:8080"
    
    # MinIO 配置
    minio_endpoint: str = "minio:9000"
    minio_access_key: str = "minioadmin"
    minio_secret_key: str = "minioadmin"
    
    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"
        env_prefix = ""
        case_sensitive = False


@lru_cache()
def get_settings() -> Settings:
    return Settings()
