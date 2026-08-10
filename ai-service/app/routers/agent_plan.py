import json
import re
from typing import Any, Dict, List, Optional

from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel, Field

from app.config import get_settings
from app.services.llm import llm_client

router = APIRouter()


class AgentPlanRequest(BaseModel):
    run_id: str
    session_id: Optional[str] = None
    message: str
    skill_id: Optional[str] = None
    skill_version: Optional[str] = None
    tools: List[Dict[str, Any]] = Field(default_factory=list)
    completed_results: List[Dict[str, Any]] = Field(default_factory=list)
    steer_instruction: Optional[str] = None


class AgentToolCall(BaseModel):
    call_id: Optional[str] = None
    tool_name: str
    tool_version: str = "v1"
    arguments: Dict[str, Any] = Field(default_factory=dict)
    idempotency_key: Optional[str] = None
    dry_run: bool = False


class AgentPlanResponse(BaseModel):
    tool_calls: List[AgentToolCall]
    model: str
    usage: Optional[Dict[str, Any]] = None


def _json_from_model_output(content: str) -> Any:
    """Accept strict JSON plus the fenced JSON commonly returned by models."""
    text = (content or "").strip()
    text = re.sub(r"^```(?:json)?\s*", "", text, flags=re.IGNORECASE)
    text = re.sub(r"\s*```$", "", text)
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        start = text.find("{")
        end = text.rfind("}")
        if start < 0 or end <= start:
            raise ValueError("model did not return a JSON object")
        return json.loads(text[start : end + 1])


def _normalize_plan(raw: Any, tools: List[Dict[str, Any]]) -> List[AgentToolCall]:
    if isinstance(raw, list):
        raw_calls = raw
    elif isinstance(raw, dict):
        raw_calls = raw.get("tool_calls", [])
    else:
        raw_calls = []
    if not isinstance(raw_calls, list) or not raw_calls:
        raise ValueError("model returned an empty Tool plan")

    allowed = {
        (str(tool.get("name", "")).strip(), str(tool.get("version", "")).strip())
        for tool in tools
        if str(tool.get("name", "")).strip()
    }
    calls: List[AgentToolCall] = []
    for raw_call in raw_calls:
        if not isinstance(raw_call, dict):
            raise ValueError("each planned Tool call must be an object")
        name = str(raw_call.get("tool_name", "")).strip()
        version = str(raw_call.get("tool_version", "v1")).strip() or "v1"
        if (name, version) not in allowed:
            raise ValueError(f"model selected a Tool outside the discovered descriptor set: {name}@{version}")
        arguments = raw_call.get("arguments", {})
        if not isinstance(arguments, dict):
            raise ValueError(f"arguments for {name} must be an object")
        calls.append(
            AgentToolCall(
                call_id=str(raw_call.get("call_id", "")).strip() or None,
                tool_name=name,
                tool_version=version,
                arguments=arguments,
                idempotency_key=str(raw_call.get("idempotency_key", "")).strip() or None,
                dry_run=bool(raw_call.get("dry_run", False)),
            )
        )
    return calls


@router.post("/agent/plan", response_model=AgentPlanResponse)
async def plan_agent_run(request: AgentPlanRequest, http_request: Request):
    """Ask the model for a Tool plan without giving it Tool execution access."""
    settings = get_settings()
    if settings.agent_planner_token:
        supplied = http_request.headers.get("authorization", "")
        expected = f"Bearer {settings.agent_planner_token}"
        if supplied != expected:
            raise HTTPException(status_code=401, detail="agent planner authorization is required")
    if not request.tools:
        raise HTTPException(status_code=422, detail="at least one discovered Tool is required")
    if not request.message.strip():
        raise HTTPException(status_code=422, detail="planner message is required")

    descriptor_view = [
        {
            "name": tool.get("name"),
            "version": tool.get("version"),
            "description": tool.get("description"),
            "level": tool.get("level"),
            "input_schema": tool.get("input_schema"),
            "supports_idempotency": tool.get("supports_idempotency", False),
        }
        for tool in request.tools
    ]
    allowed_tool_identifiers = [
        f"{tool['name']}@{tool['version']}"
        for tool in descriptor_view
        if tool.get("name") and tool.get("version")
    ]
    system_prompt = (
        "You are a constrained lease-management Agent planner. "
        "Return JSON only in the shape {\"tool_calls\":[{\"tool_name\":\"exact.name\",\"tool_version\":\"v1\",\"arguments\":{}}]}. "
        "You may select only a Tool in the supplied descriptor list. "
        "Copy the exact Tool name into tool_name; never put the @version suffix in tool_name, "
        "and never use @v1 or another placeholder as a Tool name. "
        "Never invent a Tool, SQL, URL, shell command, identity, permission, tenant or scope. "
        "Arguments must be a JSON object. For draft/write Tools, include a stable idempotency_key. "
        "Do not perform execution; the Core Runner will validate and execute each call. "
        f"The exact allowed Tool identifiers for this request are: {', '.join(allowed_tool_identifiers)}."
    )
    user_prompt = json.dumps(
        {
            "run_id": request.run_id,
            "session_id": request.session_id,
            "skill_id": request.skill_id,
            "skill_version": request.skill_version,
            "instruction": request.message,
            "steer_instruction": request.steer_instruction,
            "allowed_tool_identifiers": allowed_tool_identifiers,
            "discovered_tools": descriptor_view,
            "completed_results": request.completed_results,
        },
        ensure_ascii=False,
        default=str,
    )
    try:
        result = await llm_client.chat_completion(
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            temperature=0.0,
            max_tokens=1200,
            response_format={"type": "json_object"},
        )
        content = result["choices"][0]["message"].get("content", "")
        calls = _normalize_plan(_json_from_model_output(content), request.tools)
        return AgentPlanResponse(
            tool_calls=calls,
            model=llm_client.get_model_name(),
            usage=llm_client.usage_metadata(result),
        )
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=f"invalid structured Agent plan: {exc}")
    except Exception as exc:
        raise HTTPException(status_code=503, detail=f"Agent planner unavailable: {exc}")
