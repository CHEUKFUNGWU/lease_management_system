from fastapi import APIRouter, UploadFile, File, Form, HTTPException, BackgroundTasks
from fastapi.responses import JSONResponse
import os
import uuid
from datetime import datetime

from app.config import get_settings
from app.services.storage import upload_to_minio, get_minio_client

router = APIRouter()


@router.post("/files/upload")
async def upload_file(
    background_tasks: BackgroundTasks,
    file: UploadFile = File(...),
    task_type: str = Form("contract")
):
    """
    上传文件到 MinIO
    
    - task_type: contract, payment_schedule, event, scan_copy
    """
    settings = get_settings()
    
    # 验证文件类型
    allowed_types = {
        "application/pdf": ".pdf",
        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": ".xlsx",
        "application/vnd.ms-excel": ".xls",
        "image/jpeg": ".jpg",
        "image/png": ".png",
        "image/tiff": ".tiff",
    }
    
    if file.content_type not in allowed_types:
        raise HTTPException(
            status_code=400,
            detail=f"不支持的文件类型: {file.content_type}. 支持: PDF, Excel, JPG, PNG, TIFF"
        )
    
    allowed_task_types = {"contract", "payment_schedule", "event", "scan_copy"}
    if task_type not in allowed_task_types:
        raise HTTPException(
            status_code=400,
            detail=f"不支持的任务类型: {task_type}. 支持: contract, payment_schedule, event, scan_copy"
        )

    # 生成唯一文件名
    file_id = str(uuid.uuid4())
    original_name = file.filename
    ext = allowed_types.get(file.content_type, os.path.splitext(original_name)[1])
    object_name = f"{task_type}/{datetime.now().strftime('%Y/%m')}/{file_id}{ext}"
    
    # 读取文件内容
    content = await file.read()
    
    if len(content) > 50 * 1024 * 1024:  # 50MB 限制
        raise HTTPException(status_code=400, detail="文件大小超过 50MB 限制")
    
    # 上传到 MinIO
    try:
        file_url = upload_to_minio(
            bucket_name="lease-uploads",
            object_name=object_name,
            data=content,
            content_type=file.content_type
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"上传失败: {str(e)}")
    
    return {
        "file_id": file_id,
        "original_name": original_name,
        "object_name": object_name,
        "file_url": file_url,
        "file_size": len(content),
        "content_type": file.content_type,
        "task_type": task_type,
        "uploaded_at": datetime.now().isoformat()
    }


@router.get("/files/{file_id}/status")
async def get_file_status(file_id: str):
    """获取文件处理状态"""
    # TODO: 查询数据库获取任务状态
    return {
        "file_id": file_id,
        "status": "pending",
        "progress": 0
    }
