import json
import unittest
from unittest.mock import AsyncMock, patch

from fastapi import HTTPException

from app.routers.mapping import STANDARD_FIELDS, SuggestMappingRequest, suggest_mapping


class SuggestMappingContractTest(unittest.IsolatedAsyncioTestCase):
    def test_standard_fields_are_the_known_set(self):
        # 与 core-service retailingest 同源：13 个字段，store/date/currency/revenue 必含。
        self.assertEqual(len(STANDARD_FIELDS), 13)
        for field in ("store", "business_date", "currency", "revenue"):
            self.assertIn(field, STANDARD_FIELDS)

    async def test_suggest_mapping_filters_unknown_fields_and_fills_nulls(self):
        request = SuggestMappingRequest(
            headers=["门店编号", "神秘列"],
            column_profiles=[],
        )
        mock_result = {
            "choices": [{"message": {"content": json.dumps(
                {"门店编号": "store", "神秘列": "不存在的字段"}, ensure_ascii=False
            )}}],
        }
        with patch("app.routers.mapping.llm_client.chat_completion", new=AsyncMock(return_value=mock_result)):
            result = await suggest_mapping(request)
        self.assertEqual(result["source"], "ai")
        self.assertEqual(result["suggestions"]["门店编号"], "store")
        self.assertIsNone(result["suggestions"]["神秘列"])

    async def test_suggest_mapping_propagates_llm_failure_as_502(self):
        request = SuggestMappingRequest(headers=["a"], column_profiles=[])
        with patch("app.routers.mapping.llm_client.chat_completion", new=AsyncMock(side_effect=RuntimeError("down"))):
            with self.assertRaises(HTTPException) as ctx:
                await suggest_mapping(request)
        self.assertEqual(ctx.exception.status_code, 502)

    async def test_headers_required(self):
        request = SuggestMappingRequest(headers=[], column_profiles=[])
        with self.assertRaises(HTTPException) as ctx:
            await suggest_mapping(request)
        self.assertEqual(ctx.exception.status_code, 400)
