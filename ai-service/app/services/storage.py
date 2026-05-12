import io
from minio import Minio
from minio.error import S3Error
from app.config import get_settings


def get_minio_client():
    """获取 MinIO 客户端"""
    settings = get_settings()
    return Minio(
        settings.minio_endpoint,
        access_key=settings.minio_access_key,
        secret_key=settings.minio_secret_key,
        secure=False
    )


def upload_to_minio(bucket_name: str, object_name: str, data: bytes, content_type: str = "application/octet-stream") -> str:
    """
    上传文件到 MinIO
    
    Returns:
        str: 文件访问 URL
    """
    client = get_minio_client()
    
    # 确保 bucket 存在
    if not client.bucket_exists(bucket_name):
        client.make_bucket(bucket_name)
    
    # 上传文件
    client.put_object(
        bucket_name=bucket_name,
        object_name=object_name,
        data=io.BytesIO(data),
        length=len(data),
        content_type=content_type
    )
    
    return f"/{bucket_name}/{object_name}"


def download_from_minio(bucket_name: str, object_name: str) -> bytes:
    """
    从 MinIO 下载文件
    
    Returns:
        bytes: 文件内容
    """
    client = get_minio_client()
    
    response = client.get_object(bucket_name, object_name)
    try:
        return response.read()
    finally:
        response.close()
        response.release_conn()
