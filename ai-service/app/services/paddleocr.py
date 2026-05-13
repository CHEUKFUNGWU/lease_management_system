"""PaddleOCR AI Studio 异步 API 集成。

调用流程：
1. 提交解析任务 (POST /api/v2/ocr/jobs) — 支持 fileUrl 或 multipart 文件上传
2. 轮询任务状态 (GET /api/v2/ocr/jobs/{jobId})
3. 下载结果（JSON 格式，从 layoutParsingResults 提取 Markdown）

支持模型：PaddleOCR-VL-1.5、PaddleOCR-VL、PP-StructureV3、PP-OCRv5
每日免费额度：3000 页

注意：base64 JSON 方式提交文件返回 500，必须使用 multipart form data 方式。
"""

import asyncio
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

    async def submit_url(
        self,
        file_url: str,
        model: Optional[str] = None,
    ) -> str:
        """通过文件 URL 提交解析任务（推荐方式，适用于公开可访问的文件）。

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

        if resp.status_code == 429:
            raise RuntimeError("PaddleOCR: 每日配额已用完 (3000页)")
        if resp.status_code == 403:
            raise RuntimeError("PaddleOCR: Token 无效或 URL 不匹配")
        if resp.status_code != 200:
            raise RuntimeError(f"PaddleOCR: URL 提交失败 ({resp.status_code}): {resp.text}")

        data = resp.json()
        job_id = data.get("data", {}).get("jobId")
        if not job_id:
            raise RuntimeError(f"PaddleOCR: 响应中无 jobId: {data}")

        logger.info(f"PaddleOCR URL 任务已提交: jobId={job_id}")
        return job_id

    async def submit_file(
        self,
        file_data: bytes,
        filename: str = "document.pdf",
        content_type: str = "application/pdf",
    ) -> str:
        """通过 multipart form data 提交文件解析任务。

        注意：PaddleOCR API 的 base64 JSON 方式返回 500，必须使用 multipart 方式。

        Args:
            file_data: 文件二进制内容
            filename: 文件名
            content_type: MIME 类型

        Returns:
            jobId: 任务 ID
        """
        headers = {
            "Authorization": f"bearer {self.access_token}",
        }

        files = {
            "file": (filename, file_data, content_type),
        }
        data = {
            "model": self.model,
        }

        async with httpx.AsyncClient(timeout=120) as client:
            resp = await client.post(self.api_url, files=files, data=data, headers=headers)

        if resp.status_code == 429:
            raise RuntimeError("PaddleOCR: 每日配额已用完 (3000页)")
        if resp.status_code == 403:
            raise RuntimeError("PaddleOCR: Token 无效或 URL 不匹配")
        if resp.status_code != 200:
            raise RuntimeError(f"PaddleOCR: 提交任务失败 ({resp.status_code}): {resp.text}")

        result = resp.json()
        job_id = result.get("data", {}).get("jobId")
        if not job_id:
            raise RuntimeError(f"PaddleOCR: 响应中无 jobId: {result}")

        logger.info(f"PaddleOCR 文件任务已提交: jobId={job_id}, model={self.model}")
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

    async def _extract_markdown_from_json_url(self, json_url: str) -> str:
        """从 JSON 结果 URL 下载并提取 Markdown 文本。

        PaddleOCR API 返回的 JSON 结构：
        {
            "logId": ...,
            "result": {
                "layoutParsingResults": [
                    {
                        "markdown": { "text": "..." },
                        "prunedResult": {...},
                        "outputImages": {...},
                        "inputImage": {...}
                    }
                ],
                "dataInfo": {...},
                "preprocessedImages": [...]
            },
            "errorCode": 0,
            "errorMsg": ""
        }
        """
        async with httpx.AsyncClient(timeout=60) as client:
            resp = await client.get(json_url)

        if resp.status_code != 200:
            raise RuntimeError(f"PaddleOCR: 下载 JSON 结果失败 ({resp.status_code})")

        json_result = resp.json()

        # 尝试从 result.layoutParsingResults 提取
        result_obj = json_result.get("result", {})
        layout_results = result_obj.get("layoutParsingResults", [])

        if layout_results:
            parts = []
            for i, page in enumerate(layout_results):
                md = page.get("markdown", {})
                text = md.get("text", "") if isinstance(md, dict) else str(md) if md else ""
                if text:
                    parts.append(f"--- Page {i + 1} ---\n{text}")
            if parts:
                logger.info(f"PaddleOCR: 从 JSON 结果提取了 {len(parts)} 页文本")
                return "\n\n".join(parts)

        # 回退：尝试顶层 layoutParsingResults（旧版 API 格式）
        layout_results = json_result.get("layoutParsingResults", [])
        if layout_results:
            parts = []
            for i, page in enumerate(layout_results):
                md = page.get("markdown", {})
                text = md.get("text", "") if isinstance(md, dict) else str(md) if md else ""
                if text:
                    parts.append(f"--- Page {i + 1} ---\n{text}")
            if parts:
                return "\n\n".join(parts)

        raise RuntimeError("PaddleOCR: JSON 结果中无可用文本")

    async def extract_text_from_url(self, file_url: str) -> str:
        """一站式：通过 URL 提交 → 等待 → 返回 Markdown 文本。

        Args:
            file_url: 可公开访问的文件 URL

        Returns:
            Markdown 格式的文档文本
        """
        job_id = await self.submit_url(file_url)
        result = await self.wait_for_result(job_id)
        return await self._extract_result_text(result)

    async def extract_text_from_file(
        self,
        file_data: bytes,
        filename: str = "document.pdf",
        content_type: str = "application/pdf",
    ) -> str:
        """一站式：提交文件 → 等待 → 返回 Markdown 文本。

        使用 multipart form data 方式提交文件（base64 JSON 方式不可用）。

        Args:
            file_data: 文件二进制内容
            filename: 文件名
            content_type: MIME 类型

        Returns:
            Markdown 格式的文档文本
        """
        job_id = await self.submit_file(
            file_data=file_data,
            filename=filename,
            content_type=content_type,
        )

        result = await self.wait_for_result(job_id)
        return await self._extract_result_text(result)

    async def _extract_result_text(self, result: dict) -> str:
        """从 PaddleOCR 任务结果中提取文本。

        优先级：
        1. markdownUrl（直接下载 Markdown）— 当前 API 不返回此字段
        2. jsonUrl（下载 JSON，从 layoutParsingResults 提取）— 主要方式
        """
        result_url = result.get("resultUrl", {})

        # 优先尝试 markdownUrl（当前 API 不返回，但保留兼容性）
        markdown_url = result_url.get("markdownUrl") if isinstance(result_url, dict) else None
        if markdown_url:
            logger.info("PaddleOCR: 使用 markdownUrl 下载结果")
            async with httpx.AsyncClient(timeout=60) as client:
                resp = await client.get(markdown_url)
            if resp.status_code == 200:
                return resp.text
            logger.warning(f"PaddleOCR: markdownUrl 下载失败 ({resp.status_code})，尝试 jsonUrl")

        # 主要方式：从 jsonUrl 提取
        json_url = result_url.get("jsonUrl") if isinstance(result_url, dict) else None
        if json_url:
            logger.info("PaddleOCR: 使用 jsonUrl 下载结果")
            return await self._extract_markdown_from_json_url(json_url)

        # 最后尝试：result 可能直接包含数据
        logger.warning("PaddleOCR: 无 resultUrl，尝试从结果直接提取")
        raise RuntimeError("PaddleOCR: 无可用的结果 URL")


# 全局单例
_paddleocr_client: Optional[PaddleOCRClient] = None


def get_paddleocr_client() -> PaddleOCRClient:
    global _paddleocr_client
    if _paddleocr_client is None:
        _paddleocr_client = PaddleOCRClient()
    return _paddleocr_client