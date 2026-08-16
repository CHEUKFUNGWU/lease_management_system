from typing import Dict, List, Optional
import json
import re

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from app.services.llm import llm_client

router = APIRouter()

# 与 core-service retailingest.AllFields 保持一致的标准字段清单
# （CONTRACT-001：后端单一来源，AI 只返回这些字段名之一或 null）。
STANDARD_FIELDS = [
    "store", "business_date", "currency", "revenue", "gross_profit",
    "transactions", "footfall", "area_sqm", "labor_cost", "fixed_rent",
    "variable_rent", "non_lease_cost", "other_controllable_cost",
]


class ColumnProfile(BaseModel):
    header: str
    non_empty: int = 0
    numeric_like: int = 0
    date_like: int = 0
    masked_sample: Optional[str] = None


class SuggestMappingRequest(BaseModel):
    headers: List[str]
    column_profiles: List[ColumnProfile]


@router.post("/suggest-mapping")
async def suggest_mapping(request: SuggestMappingRequest):
    """Assist-Mode 列映射建议：只产出「表头 → 标准字段」的映射，人工确认后才进入
    确定性解析路径。输入只有表头与脱敏的列画像（计数 + 掩码样本），数字原文从不进入
    模型。"""
    if not request.headers:
        raise HTTPException(status_code=400, detail="headers must not be empty")

    system_prompt = (
        "你是门店经营数据导入的列映射助手。把文件列头映射到标准字段。"
        "标准字段：store(门店/店铺), business_date(日期), currency(币种), "
        "revenue(营业额/销售额), gross_profit(毛利), transactions(交易数), "
        "footfall(客流), area_sqm(面积), labor_cost(人工成本), fixed_rent(固定租金), "
        "variable_rent(变量租金), non_lease_cost(非租赁成本), other_controllable_cost(其他可控成本)。"
        "判定规则：date_like 高的列优先映射 business_date；numeric_like 高且语义为金额/数量的"
        "优先对应数值指标；含门店/店铺/店号/store 语义的映射 store；无法可靠判断的列映射为 null。"
        "只输出一个 JSON 对象 {列头: 字段名或 null}，字段名必须取自标准字段清单，"
        "不要输出任何解释或多余文本。"
    )
    profiles = [
        {
            "header": p.header,
            "non_empty": p.non_empty,
            "numeric_like": p.numeric_like,
            "date_like": p.date_like,
            "masked_sample": p.masked_sample,
        }
        for p in request.column_profiles
    ]
    user_content = json.dumps({"headers": request.headers, "column_profiles": profiles}, ensure_ascii=False)

    try:
        result = await llm_client.chat_completion(
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_content},
            ],
            temperature=0,
            max_tokens=800,
        )
        content = result.get("choices", [{}])[0].get("message", {}).get("content", "")
        # 剥掉可能的代码围栏
        content = re.sub(r"^```(?:json)?\s*|\s*```$", "", content.strip())
        suggestions: Dict[str, Optional[str]] = json.loads(content)
    except Exception as exc:  # 任何解析/调用失败都不该让导入流程失败
        raise HTTPException(status_code=502, detail=f"mapping suggestion unavailable: {exc}")

    # 只保留合法字段名，其余视为 null；未出现在模型输出里的表头补 null。
    cleaned: Dict[str, Optional[str]] = {}
    for header in request.headers:
        value = suggestions.get(header)
        cleaned[header] = value if value in STANDARD_FIELDS else None
    return {"suggestions": cleaned, "source": "ai"}
