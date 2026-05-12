from typing import Optional, Dict, Any
import httpx
from app.config import get_settings


class LLMClient:
    """LLM 客户端（支持 DeepSeek / OpenAI）"""
    
    def __init__(self):
        self.settings = get_settings()
        self.provider = self.settings.llm_provider.lower()
        
        if self.provider == "deepseek":
            self.api_key = self.settings.deepseek_api_key
            self.base_url = self.settings.deepseek_base_url
            self.model = self.settings.deepseek_model
        elif self.provider == "openai":
            self.api_key = self.settings.openai_api_key
            self.base_url = self.settings.openai_base_url
            self.model = self.settings.openai_model
        else:
            raise ValueError(f"不支持的 LLM 提供商: {self.provider}")
    
    async def chat_completion(
        self,
        messages: list[dict[str, str]],
        temperature: float = 0.1,
        max_tokens: Optional[int] = None,
        **kwargs
    ) -> Dict[str, Any]:
        """调用 LLM 聊天接口"""
        
        if not self.api_key:
            raise ValueError(f"{self.provider.upper()}_API_KEY 未配置")
        
        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json"
        }
        
        payload = {
            "model": self.model,
            "messages": messages,
            "temperature": temperature,
            **kwargs
        }
        
        if max_tokens:
            payload["max_tokens"] = max_tokens
        
        async with httpx.AsyncClient(timeout=120.0) as client:
            response = await client.post(
                f"{self.base_url}/chat/completions",
                headers=headers,
                json=payload
            )
            response.raise_for_status()
            return response.json()
    
    def get_model_name(self) -> str:
        return f"{self.provider}/{self.model}"


# 全局客户端实例
llm_client = LLMClient()
