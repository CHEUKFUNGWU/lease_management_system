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
        tools: Optional[list[dict[str, Any]]] = None,
        tool_choice: Optional[str | dict[str, Any]] = None,
        **kwargs
    ) -> Dict[str, Any]:
        """调用 LLM 聊天接口，支持 Function Calling"""
        
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
        
        if tools:
            payload["tools"] = tools
            if tool_choice:
                payload["tool_choice"] = tool_choice
        
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

    def usage_metadata(self, response: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        """Normalize provider usage without inventing missing token counts.

        The model gateway owns the provider response and the configured price
        book. Core receives this as operational metadata only; it is not an
        accounting amount and must not be used for financial posting.
        """
        raw = response.get("usage") if isinstance(response, dict) else None
        if not isinstance(raw, dict):
            return None

        def non_negative_int(*keys: str) -> Optional[int]:
            for key in keys:
                value = raw.get(key)
                if isinstance(value, bool):
                    continue
                try:
                    parsed = int(value)
                except (TypeError, ValueError):
                    continue
                if parsed >= 0:
                    return parsed
            return None

        input_tokens = non_negative_int("prompt_tokens", "input_tokens")
        output_tokens = non_negative_int("completion_tokens", "output_tokens")
        total_tokens = non_negative_int("total_tokens")
        if total_tokens is None and input_tokens is not None and output_tokens is not None:
            total_tokens = input_tokens + output_tokens

        metadata: Dict[str, Any] = {
            "schema_version": "llm-usage.v1",
            "provider": self.provider,
            "model": self.model,
            "input_tokens": input_tokens,
            "output_tokens": output_tokens,
            "total_tokens": total_tokens,
            "pricing_version": self.settings.llm_pricing_version,
            "pricing_source": "configured_settings",
            "cost_currency": "USD",
            "cost_status": "unavailable",
        }

        # A provider may return a measured amount. Preserve it only when it
        # is numeric and non-negative; otherwise use the versioned local price
        # book only when both token dimensions are known and both rates are
        # explicitly positive.
        provider_cost = raw.get("cost_usd", raw.get("cost"))
        try:
            provider_cost_float = float(provider_cost)
        except (TypeError, ValueError):
            provider_cost_float = -1.0
        if provider_cost_float >= 0:
            metadata["cost_micros"] = round(provider_cost_float * 1_000_000)
            metadata["cost_status"] = "measured"
            metadata["pricing_source"] = "provider_usage"
        elif (
            input_tokens is not None
            and output_tokens is not None
            and self.settings.llm_input_price_usd_per_million > 0
            and self.settings.llm_output_price_usd_per_million > 0
            and self.settings.llm_pricing_version != "unconfigured"
        ):
            cost_usd = (
                input_tokens * self.settings.llm_input_price_usd_per_million
                + output_tokens * self.settings.llm_output_price_usd_per_million
            ) / 1_000_000
            metadata["cost_micros"] = round(cost_usd * 1_000_000)
            metadata["cost_status"] = "calculated"

        return metadata


# 全局客户端实例
llm_client = LLMClient()
