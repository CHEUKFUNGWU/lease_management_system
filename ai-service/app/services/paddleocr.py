"""PaddleOCR AI Studio 异步 API 集成。

调用流程：
1. 提交解析任务 (POST /api/v2/ocr/jobs) — 支持文件直传或 URL
2. 轮询任务状态 (GET /api/v2/ocr/jobs/{jobId})
3. 下载结果（Markdown + JSON）

支持模型：PaddleOCR-VL-1.5、PaddleOCR-VL、PP-StructureV3、PP-OCRv5
每日免费额度：3000 页
"""

import asyncio
import base64
import logging
import time
from typing import Optional

import httpx

from app.config import get_settings

logger = logging.getLogger(__name__)


class PaddleOCRClient:
    """PaddleOCR AI Studio 异步 API 客户端"""

    def __init__(self):
        settings = get_settings()
        self.api_url = settings.paddleocr_api_url
        self.access_token = settings.paddleocr_access_token
        self.model = settings.paddleocr_model
        self.enabled = settings.paddleocr_enabled
        self.max_poll_seconds = settings.paddleocr_max_poll_seconds
        self.poll_interval = settings.paddleocr_poll_interval

    def is_available(self) -> bool:
        return self.enabled and bool(self.access_token)

    async def submit_file(
        self,
        file_data: bytes,
        file_type: int = 0,
        use_doc_orientation_classify: bool = True,
        use_doc_unwarping: bool = True,
        use_chart_recognition: bool = False,
        page_ranges: Optional[str] = None,
    ) -> str:
        """提交文件解析任务。

        Args:
            file_data: 文件二进制内容
            file_type: 0=PDF, 1=图片
            use_doc_orientation_classify: 图片方向矫正
            use_doc_unwarping: 图片扭曲矫正
            use_chart_recognition: 图表识别
            page_ranges: 页码范围，如 "1-10"

        Returns:
            jobId: 任务 ID
        """
        file_b64 = base64.b64encode(file_data).decode("ascii")

        payload = {
            "file": file_b64,
            "model": self.model,
            "optionalPayload": {
                "useDocOrientationClassify": use_doc_orientation_classify,
                "useDocUnwarping": use_doc_unwarping,
                "useChartRecognition": use_chart_recognition,
            },
        }
        if page_ranges:
            payload["pageRanges"] = page_ranges

        headers = {
            "Authorization": f"bearer {self.access_token}",
            "Content-Type": "application/json",
        }

        async with httpx.AsyncClient(timeout=60) as client:
            resp = await client.post(self.api_url, json=payload, headers=headers)

        if resp.status_code == 429:
            raise RuntimeError("PaddleOCR: 每日配额已用完 (3000页)")
        if resp.status_code == 403:
            raise RuntimeError("PaddleOCR: Token 无效或 URL 不匹配")
        if resp.status_code != 200:
            raise RuntimeError(f"PaddleOCR: 提交任务失败 ({resp.status_code}): {resp.text}")

        data = resp.json()
        job_id = data.get("data", {}).get("jobId")
        if not job_id:
            raise RuntimeError(f"PaddleOCR: 响应中无 jobId: {data}")

        logger.info(f"PaddleOCR 任务已提交: jobId={job_id}, model={self.model}")
        return job_id

    async def submit_url(
        self,
        file_url: str,
        model: Optional[str] = None,
    ) -> str:
        """通过文件 URL 提交解析任务（适用于 MinIO presigned URL）。

        Args:
            file_url: 可公开访问的文件 URL
            model: 模型名称，默认使用配置中的模型

        Returns:
            jobId: 任务 ID
        """
        payload = {
            "fileUrl": file_url,
            "model": model or self.model,
        }

        headers = {
            "Authorization": f"bearer {self.access_token}",
            "Content-Type": "application/json",
        }

        async with httpx.AsyncClient(timeout=60) as client:
            resp = await client.post(self.api_url, json=payload, headers=headers)

        if resp.status_code != 200:
            raise RuntimeError(f"PaddleOCR: URL 提交失败 ({resp.status_code}): {resp.text}")

        data = resp.json()
        job_id = data.get("data", {}).get("jobId")
        if not job_id:
            raise RuntimeError(f"PaddleOCR: 响应中无 jobId: {data}")

        logger.info(f"PaddleOCR URL 任务已提交: jobId={job_id}")
        return job_id

    async def get_result(self, job_id: str) -> dict:
        """查询任务结果。

        Returns:
            任务结果 dict，包含 state、resultUrl 等
        """
        url = f"{self.api_url}/{job_id}"
        headers = {
            "Authorization": f"bearer {self.access_token}",
            "Content-Type": "application/json",
        }

        async with httpx.AsyncClient(timeout=30) as client:
            resp = await client.get(url, headers=headers)

        if resp.status_code != 200:
            raise RuntimeError(f"PaddleOCR: 查询失败 ({resp.status_code}): {resp.text}")

        return resp.json().get("data", {})

    async def wait_for_result(self, job_id: str) -> dict:
        """轮询等待任务完成。

        Returns:
            完成后的 data 对象，包含 resultUrl
        """
        start = time.time()
        while True:
            result = await self.get_result(job_id)
            state = result.get("state", "unknown")

            if state == "done":
                logger.info(f"PaddleOCR 任务完成: jobId={job_id}")
                return result
            elif state == "failed":
                error_msg = result.get("errorMsg", "未知错误")
                raise RuntimeError(f"PaddleOCR: 任务失败: {error_msg}")

            elapsed = time.time() - start
            if elapsed > self.max_poll_seconds:
                raise RuntimeError(
                    f"PaddleOCR: 轮询超时 ({self.max_poll_seconds}s), jobId={job_id}, state={state}"
                )

            # 显示进度
            progress = result.get("extractProgress", {})
            total = progress.get("totalPages", "?")
            done = progress.get("extractedPages", "?")
            logger.info(f"PaddleOCR 进度: {done}/{total} 页, state={state}")

            await asyncio.sleep(self.poll_interval)

    async def download_markdown(self, markdown_url: str) -> str:
        """下载 Markdown 结果。"""
        async with httpx.AsyncClient(timeout=60) as client:
            resp = await client.get(markdown_url)
        if resp.status_code != 200:
            raise RuntimeError(f"PaddleOCR: 下载 Markdown 失败 ({resp.status_code})")
        return resp.text

    async def extract_text_from_file(
        self,
        file_data: bytes,
        file_type: int = 0,
        page_ranges: Optional[str] = None,
    ) -> str:
        """一站式：提交文件 → 等待 → 返回 Markdown 文本。

        Args:
            file_data: 文件二进制内容
            file_type: 0=PDF, 1=图片
            page_ranges: 页码范围

        Returns:
            Markdown 格式的文档文本
        """
        job_id = await self.submit_file(
            file_data=file_data,
            file_type=file_type,
            page_ranges=page_ranges,
        )

        result = await self.wait_for_result(job_id)

        # 获取结果 URL
        result_url = result.get("resultUrl", {})
        markdown_url = result_url.get("markdownUrl")

        if not markdown_url:
            # 回退：尝试从 layoutParsingResults 获取
            logger.warning("PaddleOCR: 无 markdownUrl，尝试从 JSON 结果提取")
            json_url = result_url.get("jsonUrl")
            if json_url:
                async with httpx.AsyncClient(timeout=60) as client:
                    resp = await client.get(json_url)
                if resp.status_code == 200:
                    json_result = resp.json()
                    # 从 JSON 中拼接文本
                    parts = []
                    for page in json_result.get("layoutParsingResults", []):
                        md = page.get("markdown", {})
                        if md.get("text"):
                            parts.append(md["text"])
                    return "\n\n".join(parts)
            raise RuntimeError("PaddleOCR: 无可用的结果 URL")

        return await self.download_markdown(markdown_url)


# 全局单例
_paddleocr_client: Optional[PaddleOCRClient] = None


def get_paddleocr_client() -> PaddleOCRClient:
    global _paddleocr_client
    if _paddleocr_client is None:
        _paddleocr_client = PaddleOCRClient()
    return _paddleocr_client
