from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from typing import List, Optional, Dict, Any
from app.services.llm import llm_client

router = APIRouter()


class ChatMessage(BaseModel):
    role: str
    content: str


class ChatRequest(BaseModel):
    messages: List[ChatMessage]
    system_prompt: Optional[str] = None
    temperature: float = 0.3
    max_tokens: Optional[int] = 2000
    language: Optional[str] = "zh-CN"
    tools: Optional[List[Dict[str, Any]]] = None
    tool_choice: Optional[str] = None


class Source(BaseModel):
    type: str
    id: str
    title: str
    snippet: str


class ToolCall(BaseModel):
    id: str
    type: str = "function"
    function: Dict[str, Any]


class ChatResponse(BaseModel):
    answer: str
    sources: List[Source]
    confidence: float
    model: str
    tool_calls: Optional[List[ToolCall]] = None


@router.post("/chat", response_model=ChatResponse)
async def chat(request: ChatRequest):
    """
    AI 聊天接口 — 接收消息列表和系统提示，返回 LLM 回复。
    支持 Function Calling：传入 tools 后，LLM 可返回 tool_calls 而非直接回答。
    
    此接口由 Core Service 调用，Core Service 负责：
    - 权限过滤（基于 JWT 中的 legal_entity_id）
    - 系统上下文构建（合同数据、计量结果、分录等）
    - 来源引用提取
    - Tool 执行和结果回传
    
    AI Service 仅负责调用 LLM 并返回结果（含 tool_calls）。
    """
    try:
        lang = request.language or "zh-CN"
        system_prompt = request.system_prompt or ""
        if lang == "zh-CN":
            system_prompt += "\n\n请用简体中文回答。"
        elif lang == "zh-TW":
            system_prompt += "\n\n請用繁體中文回答。"
        elif lang == "en":
            system_prompt += "\n\nPlease answer in English."
        
        messages = []
        if system_prompt:
            messages.append({"role": "system", "content": system_prompt})
        
        for msg in request.messages:
            messages.append({"role": msg.role, "content": msg.content})
        
        kwargs = {}
        if request.tools:
            kwargs["tools"] = request.tools
        if request.tool_choice:
            kwargs["tool_choice"] = request.tool_choice
        
        result = await llm_client.chat_completion(
            messages=messages,
            temperature=request.temperature,
            max_tokens=request.max_tokens,
            **kwargs
        )
        
        message = result["choices"][0]["message"]
        answer = message.get("content", "")
        
        # Extract tool_calls if present
        tool_calls = None
        if "tool_calls" in message:
            tool_calls = [
                ToolCall(
                    id=tc.get("id", ""),
                    type=tc.get("type", "function"),
                    function=tc.get("function", {})
                )
                for tc in message["tool_calls"]
            ]
        
        # Sources are extracted by Core Service from the context it built
        return ChatResponse(
            answer=answer,
            sources=[],
            confidence=0.9,
            model=llm_client.get_model_name(),
            tool_calls=tool_calls
        )
    except ValueError as e:
        raise HTTPException(status_code=503, detail=f"LLM 配置错误: {str(e)}")
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"LLM 调用失败: {str(e)}")
