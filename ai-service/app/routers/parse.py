from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from typing import Optional, List

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


@router.post("/parse", response_model=ParseResponse)
async def parse_document(request: ParseRequest):
    """
    解析文档并抽取结构化字段
    """
    # TODO: 从 MinIO 获取文件内容
    # TODO: 调用 OCR（如果是扫描件）
    # TODO: 调用 LLM 抽取字段
    
    return {
        "task_id": "task_" + request.file_id,
        "file_id": request.file_id,
        "status": "pending",
        "extracted_data": None,
        "confidence_scores": None
    }


@router.post("/parse/contract")
async def parse_contract(file_id: str, file_content: str):
    """
    解析合同文件，抽取关键字段
    """
    prompt = f"""
    你是一位专业的 IFRS 16 租赁合同解析专家。请从以下合同文本中提取关键字段。
    
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
    - currency: 币种 (CNY/USD/EUR)
    - fixed_rent_amount: 固定租金金额
    - payment_frequency: 付款频率 (monthly/quarterly/yearly)
    - payment_timing: 付款时点 (prepaid/postpaid)
    - renewal_option: 是否有续租选择权 (true/false)
    - termination_option: 是否有终止选择权 (true/false)
    - cam_amount: 物业管理费 (如有)
    - service_fee: 服务费 (如有)
    
    请以 JSON 格式输出，包含 confidence_score (0-1)。
    """
    
    try:
        response = await llm_client.chat_completion(
            messages=[
                {"role": "system", "content": "你是一位专业的 IFRS 16 租赁合同解析专家。请准确提取合同字段并以 JSON 格式返回。"},
                {"role": "user", "content": prompt}
            ],
            temperature=0.1,
            max_tokens=2000
        )
        
        # 解析 LLM 响应
        content = response["choices"][0]["message"]["content"]
        
        return {
            "task_id": "task_" + file_id,
            "file_id": file_id,
            "status": "completed",
            "extracted_data": content,
            "confidence_scores": {"overall": 0.85}
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"解析失败: {str(e)}")


@router.post("/parse/payment-schedule")
async def parse_payment_schedule(file_id: str, file_content: str):
    """
    解析租金表，提取付款计划
    """
    prompt = f"""
    请从以下租金表中提取付款计划信息。
    
    租金表内容:
    {file_content}
    
    请提取每笔付款的以下字段（JSON 数组格式）:
    - period_start: 覆盖期间起始日 (YYYY-MM-DD)
    - period_end: 覆盖期间结束日 (YYYY-MM-DD)
    - due_date: 应付日期 (YYYY-MM-DD)
    - amount: 金额
    - payment_timing: 先付/后付 (prepaid/postpaid)
    - is_fixed: 是否固定租金 (true/false)
    
    请以 JSON 数组格式输出。
    """
    
    try:
        response = await llm_client.chat_completion(
            messages=[
                {"role": "system", "content": "你是一位专业的租金表解析专家。请准确提取付款计划信息并以 JSON 格式返回。"},
                {"role": "user", "content": prompt}
            ],
            temperature=0.1,
            max_tokens=3000
        )
        
        content = response["choices"][0]["message"]["content"]
        
        return {
            "task_id": "task_" + file_id,
            "file_id": file_id,
            "status": "completed",
            "extracted_data": content,
            "confidence_scores": {"overall": 0.9}
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"解析失败: {str(e)}")
