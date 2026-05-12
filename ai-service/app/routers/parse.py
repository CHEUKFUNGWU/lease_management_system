from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from typing import Optional, List, Dict, Any

from app.services.llm import llm_client

router = APIRouter()


class ParseRequest(BaseModel):
    file_id: str
    file_type: str  # contract, payment_schedule, event
    task_type: Optional[str] = "contract_extraction"


class ParseResponse(BaseModel):
    task_id: str
    file_id: str
    status: str
    extracted_data: Optional[dict] = None
    confidence_scores: Optional[dict] = None
    warnings: Optional[List[str]] = None


class ContractDraftRequest(BaseModel):
    file_id: str
    file_content: str
    mode: str = "assist"  # "assist" or "auto-post"


class ContractDraftResponse(BaseModel):
    task_id: str
    file_id: str
    mode: str
    draft_type: str = "contract_draft"
    status: str
    extracted_data: Dict[str, Any]
    confidence_scores: Dict[str, float]
    missing_fields: List[str]
    warnings: List[str]
    requires_human_confirmation: bool


def _check_discount_rate_missing(extracted: dict) -> tuple[bool, list]:
    """检查折现率是否缺失"""
    missing = []
    warnings = []
    
    if not extracted.get("discount_rate_type") and not extracted.get("discount_rate"):
        missing.append("discount_rate")
        warnings.append("【关键】合同缺少折现率信息。AI 不得猜测折现率，需要人工确认。")
        warnings.append("建议处理方式：")
        warnings.append("1. 检查合同文本中是否明确提到利率")
        warnings.append("2. 从系统政策库中查找适用的 IBR")
        warnings.append("3. 按法人/租期区间/门店类型匹配利率政策")
        warnings.append("4. 如无法唯一确定，请人工输入或选择")
    
    return len(missing) > 0, warnings


def _check_critical_fields(extracted: dict) -> tuple[list, list]:
    """检查关键字段是否缺失或低置信度"""
    missing = []
    warnings = []
    
    critical_fields = [
        ("contract_number", "合同编号"),
        ("commencement_date", "租赁起始日"),
        ("lease_start_date", "租赁开始日"),
        ("lease_end_date", "租期结束日"),
        ("fixed_rent_amount", "固定租金金额"),
        ("payment_timing", "付款时点（先付/后付）"),
    ]
    
    for field, label in critical_fields:
        if not extracted.get(field):
            missing.append(field)
            warnings.append(f"【关键字段缺失】{label}({field}) 未识别到，必须人工补充")
    
    return missing, warnings


@router.post("/parse", response_model=ParseResponse)
async def parse_document(request: ParseRequest):
    """
    解析文档并抽取结构化字段
    """
    return {
        "task_id": "task_" + request.file_id,
        "file_id": request.file_id,
        "status": "pending",
        "extracted_data": None,
        "confidence_scores": None,
        "warnings": ["AI Assist Mode: 识别结果需人工确认后入库"]
    }


@router.post("/parse/contract", response_model=ContractDraftResponse)
async def parse_contract(request: ContractDraftRequest):
    """
    解析合同文件，生成合同草稿
    
    默认 Assist Mode：
    - AI 只生成草稿，不直接写入正式台账
    - 所有结果需 Editor 确认
    - 关键字段缺失时必须标记人工确认
    """
    
    if request.mode != "assist":
        raise HTTPException(
            status_code=400, 
            detail="当前仅支持 Assist Mode。Auto-Post Mode 需另行配置"
        )
    
    prompt = f"""
    你是一位专业的 IFRS 16 租赁合同解析专家。请从以下合同文本中提取关键字段。
    
    【重要规则】
    1. 如果合同中未明确提到折现率/利率，不要猜测，标记为缺失
    2. 区分先付租金(prepaid)和后付租金(postpaid)
    3. 区分固定租金和变量租金
    4. 识别租赁成分和非租赁成分(CAM、服务费)
    
    合同文本:
    {request.file_content}
    
    请提取以下字段（JSON 格式）:
    - contract_number: 合同编号
    - contract_name: 合同名称
    - legal_entity: 法人主体
    - store_name: 门店名称
    - landlord: 出租方
    - commencement_date: 租赁起始日 (YYYY-MM-DD)
    - lease_start_date: 租赁开始日 (YYYY-MM-DD)
    - lease_end_date: 租期结束日 (YYYY-MM-DD)
    - currency: 币种 (CNY/USD/EUR)
    - fixed_rent_amount: 固定租金金额
    - payment_frequency: 付款频率 (monthly/quarterly/yearly)
    - payment_timing: 付款时点 (prepaid/postpaid)
    - renewal_option: 是否有续租选择权 (true/false)
    - termination_option: 是否有终止选择权 (true/false)
    - cam_amount: 物业管理费 (如有)
    - service_fee: 服务费 (如有)
    - discount_rate_type: 折现率类型 (如合同中提及)
    - discount_rate: 折现率数值 (如合同中提及)
    
    请以 JSON 格式输出，包含:
    - extracted_fields: 提取的字段
    - confidence_scores: 每个字段的置信度 (0-1)
    - overall_confidence: 总体置信度
    - missing_fields: 识别为缺失的字段列表
    - warnings: 需要人工注意的问题列表
    """
    
    try:
        response = await llm_client.chat_completion(
            messages=[
                {"role": "system", "content": "你是一位专业的 IFRS 16 租赁合同解析专家。请准确提取合同字段。如果字段未在合同中出现，不要猜测，标记为缺失。"},
                {"role": "user", "content": prompt}
            ],
            temperature=0.1,
            max_tokens=2500,
            response_format={"type": "json_object"}
        )
        
        content = response["choices"][0]["message"]["content"]
        
        # Parse JSON response (simplified for MVP)
        import json
        try:
            parsed = json.loads(content)
        except:
            parsed = {
                "extracted_fields": {},
                "confidence_scores": {},
                "overall_confidence": 0.5,
                "missing_fields": [],
                "warnings": ["解析响应格式异常，请人工检查"]
            }
        
        extracted = parsed.get("extracted_fields", {})
        confidence = parsed.get("confidence_scores", {})
        
        # Check for discount rate missing
        dr_missing, dr_warnings = _check_discount_rate_missing(extracted)
        
        # Check for critical fields
        missing_fields, field_warnings = _check_critical_fields(extracted)
        
        all_warnings = dr_warnings + field_warnings + parsed.get("warnings", [])
        
        # Determine if human confirmation is required
        requires_human = (
            dr_missing or
            len(missing_fields) > 0 or
            parsed.get("overall_confidence", 1.0) < 0.8
        )
        
        return {
            "task_id": "task_" + request.file_id,
            "file_id": request.file_id,
            "mode": "assist",
            "draft_type": "contract_draft",
            "status": "draft_generated",
            "extracted_data": extracted,
            "confidence_scores": confidence,
            "missing_fields": missing_fields + (["discount_rate"] if dr_missing else []),
            "warnings": all_warnings,
            "requires_human_confirmation": requires_human
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"解析失败: {str(e)}")


@router.post("/parse/payment-schedule")
async def parse_payment_schedule(file_id: str, file_content: str):
    """
    解析租金表，提取付款计划草稿
    """
    prompt = f"""
    请从以下租金表中提取付款计划信息。
    
    【重要规则】
    1. 识别每期是固定租金还是变量租金
    2. 识别付款时点：先付(prepaid)还是后付(postpaid)
    3. 识别是否包含非租赁成分(CAM、服务费)
    
    租金表内容:
    {file_content}
    
    请提取每笔付款的以下字段（JSON 数组格式）:
    - period_start: 覆盖期间起始日 (YYYY-MM-DD)
    - period_end: 覆盖期间结束日 (YYYY-MM-DD)
    - due_date: 应付日期 (YYYY-MM-DD)
    - amount: 金额
    - payment_timing: 先付/后付 (prepaid/postpaid)
    - is_fixed: 是否固定租金 (true/false)
    - is_lease_component: 是否租赁成分 (true/false)
    
    请以 JSON 数组格式输出。
    """
    
    try:
        response = await llm_client.chat_completion(
            messages=[
                {"role": "system", "content": "你是一位专业的租金表解析专家。请准确提取付款计划信息。"},
                {"role": "user", "content": prompt}
            ],
            temperature=0.1,
            max_tokens=3000
        )
        
        content = response["choices"][0]["message"]["content"]
        
        return {
            "task_id": "task_" + file_id,
            "file_id": file_id,
            "mode": "assist",
            "draft_type": "payment_schedule_draft",
            "status": "draft_generated",
            "extracted_data": content,
            "confidence_scores": {"overall": 0.9},
            "warnings": ["AI Assist Mode: 付款计划草稿需人工确认后入库"]
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"解析失败: {str(e)}")
