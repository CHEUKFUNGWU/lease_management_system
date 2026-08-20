from pydantic import Field, model_validator
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

    # Optional, versioned model pricing. Usage is still returned when these
    # rates are absent, but monetary cost remains explicitly unavailable.
    llm_pricing_version: str = "unconfigured"
    llm_input_price_usd_per_million: float = Field(default=0.0, ge=0)
    llm_output_price_usd_per_million: float = Field(default=0.0, ge=0)
    
    # PaddleOCR 配置（AI Studio 异步 API）
    paddleocr_api_url: str = "https://paddleocr.aistudio-app.com/api/v2/ocr/jobs"
    paddleocr_access_token: str = ""
    paddleocr_model: str = "PaddleOCR-VL-1.5"
    paddleocr_enabled: bool = False  # 需要配置 token 后手动开启
    paddleocr_max_poll_seconds: int = 120  # 轮询超时
    paddleocr_poll_interval: float = 2.0  # 轮询间隔
    
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

    @model_validator(mode="after")
    def validate_pricing_book(self):
        """Keep the cost state explicit and reject half-configured price books.

        The zero/unconfigured combination is valid for local development and
        for providers that return measured cost. Once a local price book is
        enabled, both token dimensions and its version must be present so a
        production deployment cannot silently calculate a partial amount.
        """
        version = self.llm_pricing_version.strip()
        input_price = self.llm_input_price_usd_per_million
        output_price = self.llm_output_price_usd_per_million
        if not version:
            raise ValueError("LLM_PRICING_VERSION must not be empty")
        if version.lower() == "unconfigured":
            if input_price != 0 or output_price != 0:
                raise ValueError(
                    "LLM_PRICING_VERSION=unconfigured requires both LLM prices to be 0"
                )
            return self
        if input_price <= 0 or output_price <= 0:
            raise ValueError(
                "configured LLM_PRICING_VERSION requires positive input and output prices"
            )
        return self


@lru_cache()
def get_settings() -> Settings:
    return Settings()
