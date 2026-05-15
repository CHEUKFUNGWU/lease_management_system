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


class Source(BaseModel):
    type: str
    id: str
    title: str
    snippet: str


class ChatResponse(BaseModel):
    answer: str
    sources: List[Source]
    confidence: float
    model: str


@router.post("/chat", response_model=ChatResponse)
async def chat(request: ChatRequest):
    """
    AI 聊天接口 — 接收消息列表和系统提示，返回 LLM 回复。
    
    此接口由 Core Service 调用，Core Service 负责：
    - 权限过滤（基于 JWT 中的 legal_entity_id）
    - 系统上下文构建（合同数据、计量结果、分录等）
    - 来源引用提取
    
    AI Service 仅负责调用 LLM 并返回结果。
    """
    try:
        messages = []
        if request.system_prompt:
            messages.append({"role": "system", "content": request.system_prompt})
        
        for msg in request.messages:
            messages.append({"role": msg.role, "content": msg.content})
        
        result = await llm_client.chat_completion(
            messages=messages,
            temperature=request.temperature,
            max_tokens=request.max_tokens
        )
        
        answer = result["choices"][0]["message"]["content"]
        
        # Sources are extracted by Core Service from the context it built
        return ChatResponse(
            answer=answer,
            sources=[],
            confidence=0.9,
            model=llm_client.get_model_name()
        )
    except ValueError as e:
        raise HTTPException(status_code=503, detail=f"LLM 配置错误: {str(e)}")
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"LLM 调用失败: {str(e)}")
