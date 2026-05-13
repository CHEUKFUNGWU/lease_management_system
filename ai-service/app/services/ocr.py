"""OCR service with optional PaddleOCR support.

PaddleOCR is optional — if not installed, the service falls back to
pymupdf (PyMuPDF) for text extraction from PDFs, and LLM-based
parsing for scanned documents.
"""

import logging
from typing import Optional

logger = logging.getLogger(__name__)

_PADDLEOCR_AVAILABLE = False

try:
    from paddleocr import PaddleOCR
    _PADDLEOCR_AVAILABLE = True
    logger.info("PaddleOCR is available")
except ImportError:
    logger.info("PaddleOCR not installed — using pymupdf + LLM fallback for OCR")


def is_ocr_available() -> bool:
    """Check if PaddleOCR is available."""
    return _PADDLEOCR_AVAILABLE


_ocr_engine: Optional[object] = None


def get_ocr_engine():
    """Get or initialize the PaddleOCR engine (lazy loading)."""
    global _ocr_engine
    if not _PADDLEOCR_AVAILABLE:
        return None
    if _ocr_engine is None:
        _ocr_engine = PaddleOCR(
            use_angle_cls=True,
            lang="ch",
            show_log=False,
        )
    return _ocr_engine


def extract_text_with_ocr(image_bytes: bytes) -> Optional[str]:
    """Extract text from image bytes using PaddleOCR.

    Returns None if PaddleOCR is not available.
    """
    engine = get_ocr_engine()
    if engine is None:
        return None

    import numpy as np
    from PIL import Image
    import io

    image = Image.open(io.BytesIO(image_bytes))
    img_array = np.array(image)

    result = engine.ocr(img_array, cls=True)
    if not result or not result[0]:
        return ""

    lines = [line[1][0] for line in result[0] if line[1]]
    return "\n".join(lines)
