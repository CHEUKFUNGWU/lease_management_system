from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from typing import Optional, List, Dict, Any
import json

from app.services.llm import llm_client
from app.services.storage import download_from_minio, get_minio_client
from app.services.document_extractor import extract_text

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
    object_name: str  # MinIO object name for file download
    content_type: str = "application/pdf"  # MIME type for text extraction
    file_content: Optional[str] = None  # Optional: pre-extracted text (if provided, skip extraction)
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


def _check_currency_missing(extracted: dict) -> tuple[bool, list]:
    """检查货币是否缺失 — AI 不得猜测货币，必须询问用户确认"""
    missing = False
    warnings = []
    currency = extracted.get("currency")
    
    # Treat empty, null, or placeholder values as missing
    if not currency or str(currency).strip() == "" or str(currency).lower() in ("unknown", "null", "none"):
        missing = True
        warnings.append("【必须确认】AI 未识别到合同货币。根据 IFRS 16 计量要求，货币直接影响租赁负债现值计算和后续会计分录。")
        warnings.append("请在上传后手动选择货币（CNY / USD / EUR 等）。AI 不会猜测货币。")
    
    return missing, warnings


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
    
    # Extract text: use provided file_content or download from MinIO + PaddleOCR
    file_content = request.file_content
    if not file_content:
        # Download file from MinIO
        try:
            file_data = download_from_minio("ifrs16-uploads", request.object_name)
        except Exception as e:
            raise HTTPException(status_code=500, detail=f"文件下载失败: {str(e)}")
        
        # Extract text using PaddleOCR (primary) + fallback
        try:
            file_content = await extract_text(file_data, request.content_type)
        except Exception as e:
            raise HTTPException(status_code=500, detail=f"文本提取失败: {str(e)}")
        
        if len(file_content) > 15000:
            file_content = file_content[:15000] + "\n... (truncated)"
    
    prompt = f"""
    你是一位专业的 IFRS 16 租赁合同解析专家。请从以下合同文本中提取关键字段。
    
    【重要规则 — 必须遵守】
    1. 如果合同中未明确提到折现率/利率，不要猜测，标记为缺失
    2. 如果合同中未明确提到货币（币种），不要猜测，标记为缺失或 unknown
    3. 区分先付租金(prepaid)和后付租金(postpaid)
    4. 区分固定租金和变量租金
    5. 识别租赁成分和非租赁成分(CAM、服务费)
    
    合同文本:
    {file_content}
    
    请提取以下字段（JSON 格式）:
    - contract_number: 合同编号
    - contract_name: 合同名称
    - legal_entity: 法人主体
    - store_name: 门店名称
    - landlord: 出租方
    - commencement_date: 租赁起始日 (YYYY-MM-DD)
    - lease_start_date: 租赁开始日 (YYYY-MM-DD)
    - lease_end_date: 租期结束日 (YYYY-MM-DD)
    - currency: 币种 (CNY/USD/EUR)。如果合同中没有明确提到货币，返回 null 或空字符串，不要猜测
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
        
        # Check for currency missing — AI must not guess currency
        currency_missing, currency_warnings = _check_currency_missing(extracted)
        
        # Check for critical fields
        missing_fields, field_warnings = _check_critical_fields(extracted)
        
        all_warnings = dr_warnings + currency_warnings + field_warnings + parsed.get("warnings", [])
        
        # Determine if human confirmation is required
        requires_human = (
            dr_missing or
            currency_missing or
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
            "missing_fields": missing_fields + (["discount_rate"] if dr_missing else []) + (["currency"] if currency_missing else []),
            "warnings": all_warnings,
            "requires_human_confirmation": requires_human
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"解析失败: {str(e)}")


class PaymentScheduleParseRequest(BaseModel):
    file_id: str
    object_name: str
    content_type: str
    mode: str = "assist"


class PaymentScheduleItem(BaseModel):
    period_start: str
    period_end: str
    due_date: str
    amount: float
    payment_timing: str
    is_fixed: bool
    is_lease_component: bool
    amount_type: Optional[str] = "fixed_rent"
    currency: Optional[str] = "CNY"
    confidence: Optional[float] = 0.9


class PaymentScheduleParseResponse(BaseModel):
    task_id: str
    file_id: str
    mode: str
    draft_type: str
    status: str
    schedules: List[PaymentScheduleItem]
    confidence_scores: Dict[str, float]
    missing_fields: List[str]
    warnings: List[str]
    requires_human_confirmation: bool


@router.post("/parse/payment-schedule", response_model=PaymentScheduleParseResponse)
async def parse_payment_schedule(request: PaymentScheduleParseRequest):
    """
    解析租金表文件，提取付款计划草稿
    
    流程:
    1. 从 MinIO 下载文件
    2. 提取文本 (PDF/Excel)
    3. LLM 解析付款计划
    4. 返回结构化草稿 + 置信度
    """
    
    if request.mode != "assist":
        raise HTTPException(
            status_code=400,
            detail="当前仅支持 Assist Mode。Auto-Post Mode 需另行配置"
        )
    
    # Download file from MinIO
    try:
        file_data = download_from_minio("ifrs16-uploads", request.object_name)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"文件下载失败: {str(e)}")
    
    # Extract text
    try:
        file_content = await extract_text(file_data, request.content_type)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"文本提取失败: {str(e)}")
    
    if len(file_content) > 15000:
        file_content = file_content[:15000] + "\n... (truncated)"
    
    prompt = f"""
你是一位专业的 IFRS 16 租金表解析专家。请从以下租金表内容中提取付款计划信息。

【重要规则 - 必须遵守】
1. 每笔付款必须识别：先付(prepaid)还是后付(postpaid)
   - 先付：在覆盖期间开始前支付（如月初预付当月租金）
   - 后付：在覆盖期间结束后支付（如月末支付当月租金）
2. 区分固定租金和变量租金（turnover rent / sales-based rent 必须标记为变量）
3. 区分租赁成分和非租赁成分（CAM、管理费、服务费等）
4. 金额必须是数字，不要包含货币符号
5. 日期格式必须为 YYYY-MM-DD
6. 如果租金表是月度数据，期间起始日=当月1日，期间结束日=当月最后一日
7. 如果某期金额为空或为0，跳过该行

租金表内容:
{file_content}

请提取每笔付款，以 JSON 数组格式输出。每个元素包含:
- period_start: 覆盖期间起始日 (YYYY-MM-DD)
- period_end: 覆盖期间结束日 (YYYY-MM-DD)
- due_date: 应付日期 (YYYY-MM-DD)
- amount: 金额 (纯数字)
- payment_timing: "prepaid" 或 "postpaid"
- is_fixed: true/false
- is_lease_component: true/false
- amount_type: "fixed_rent" | "turnover_rent" | "cam" | "service_fee" | "tax" | "deposit" | "other"
- currency: "CNY" 或文件中出现的币种。如果文件中未明确提到货币，返回 null 或空字符串，不要猜测
- confidence: 该笔识别的置信度 (0.0-1.0)

额外输出字段（JSON 对象顶层）:
- overall_confidence: 总体置信度 (0.0-1.0)
- missing_fields: 识别中遇到问题的字段列表
- warnings: 需要人工注意的问题列表（如：日期格式不确定、金额可能有误等）
- total_schedules: 识别到的付款笔数

请以纯 JSON 格式输出，不要包含任何 markdown 代码块标记。
"""
    
    try:
        response = await llm_client.chat_completion(
            messages=[
                {"role": "system", "content": "你是一位专业的 IFRS 16 租金表解析专家。请准确提取付款计划信息，严格遵守日期格式和金额格式要求。"},
                {"role": "user", "content": prompt}
            ],
            temperature=0.1,
            max_tokens=4000
        )
        
        content = response["choices"][0]["message"]["content"]
        
        # Clean up markdown code blocks if present
        content = content.strip()
        if content.startswith("```json"):
            content = content[7:]
        if content.startswith("```"):
            content = content[3:]
        if content.endswith("```"):
            content = content[:-3]
        content = content.strip()
        
        # Parse JSON
        try:
            parsed = json.loads(content)
            # Handle case where LLM returns a JSON array directly
            if isinstance(parsed, list):
                parsed = {
                    "schedules": parsed,
                    "overall_confidence": 0.8,
                    "missing_fields": [],
                    "warnings": [],
                    "total_schedules": len(parsed)
                }
        except json.JSONDecodeError:
            # Try to extract JSON array from text
            try:
                start = content.find("[")
                end = content.rfind("]") + 1
                if start >= 0 and end > start:
                    arr = json.loads(content[start:end])
                    parsed = {"schedules": arr, "overall_confidence": 0.7, "missing_fields": [], "warnings": ["JSON 解析使用了启发式提取"], "total_schedules": len(arr)}
                else:
                    parsed = {"schedules": [], "overall_confidence": 0.0, "missing_fields": ["all"], "warnings": ["无法解析 LLM 输出"], "total_schedules": 0}
            except Exception:
                parsed = {"schedules": [], "overall_confidence": 0.0, "missing_fields": ["all"], "warnings": ["无法解析 LLM 输出"], "total_schedules": 0}
        
        schedules = parsed.get("schedules", [])
        
        # Validate and normalize schedules
        validated_schedules = []
        warnings = parsed.get("warnings", [])
        
        for i, s in enumerate(schedules):
            # Validate required fields
            if not s.get("due_date") or not s.get("amount"):
                warnings.append(f"第 {i+1} 行缺少必要字段 (due_date/amount)，已跳过")
                continue
            
            # Validate date format
            for date_field in ["period_start", "period_end", "due_date"]:
                if s.get(date_field) and len(s[date_field]) != 10:
                    warnings.append(f"第 {i+1} 行 {date_field} 日期格式可能不正确: {s[date_field]}")
            
            # Validate amount is numeric
            try:
                amount = float(s["amount"])
                if amount <= 0:
                    warnings.append(f"第 {i+1} 行金额 <= 0，已跳过")
                    continue
            except (ValueError, TypeError):
                warnings.append(f"第 {i+1} 行金额无法解析为数字: {s.get('amount')}")
                continue
            
            validated_schedules.append(PaymentScheduleItem(
                period_start=s.get("period_start", s.get("due_date", "")),
                period_end=s.get("period_end", s.get("due_date", "")),
                due_date=s["due_date"],
                amount=amount,
                payment_timing=s.get("payment_timing", "postpaid"),
                is_fixed=s.get("is_fixed", True),
                is_lease_component=s.get("is_lease_component", True),
                amount_type=s.get("amount_type", "fixed_rent"),
                currency=s.get("currency", "CNY"),
                confidence=s.get("confidence", 0.8),
            ))
        
        # Check for low confidence items
        low_confidence_count = sum(1 for s in validated_schedules if (s.confidence or 1.0) < 0.8)
        if low_confidence_count > 0:
            warnings.append(f"有 {low_confidence_count} 笔付款的置信度低于 0.8，建议人工复核")
        
        # Check for prepaid vs postpaid consistency
        timings = set(s.payment_timing for s in validated_schedules)
        if len(timings) > 1:
            warnings.append("租金表中同时出现先付和后付，请确认是否正确")
        
        overall_confidence = parsed.get("overall_confidence", 0.8)
        requires_human = (
            overall_confidence < 0.8 or
            len(validated_schedules) == 0 or
            low_confidence_count > len(validated_schedules) / 2
        )
        
        return {
            "task_id": "task_ps_" + request.file_id,
            "file_id": request.file_id,
            "mode": "assist",
            "draft_type": "payment_schedule_draft",
            "status": "draft_generated",
            "schedules": validated_schedules,
            "confidence_scores": {
                "overall": overall_confidence,
                "average_item": sum((s.confidence or 0.8) for s in validated_schedules) / max(len(validated_schedules), 1),
            },
            "missing_fields": parsed.get("missing_fields", []),
            "warnings": warnings + ["AI Assist Mode: 付款计划草稿需人工确认后入库"],
            "requires_human_confirmation": requires_human,
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"解析失败: {str(e)}")
