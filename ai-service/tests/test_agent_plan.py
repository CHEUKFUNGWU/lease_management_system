import json
import unittest
from types import SimpleNamespace
from unittest.mock import AsyncMock, patch

from fastapi import HTTPException

from app.routers.agent_plan import (
    AgentPlanRequest,
    _json_from_model_output,
    _normalize_plan,
    plan_agent_run,
)


class AgentPlannerContractTest(unittest.IsolatedAsyncioTestCase):
    def test_normalizer_rejects_tool_outside_discovered_descriptors(self):
        with self.assertRaises(ValueError):
            _normalize_plan(
                {"tool_calls": [{"tool_name": "lease.contract.delete", "tool_version": "v1"}]},
                [{"name": "lease.contract.get", "version": "v1"}],
            )

    def test_json_parser_accepts_fenced_json_only_as_a_transport_compatibility(self):
        parsed = _json_from_model_output('```json\n{"tool_calls": []}\n```')
        self.assertEqual(parsed, {"tool_calls": []})

    async def test_planner_requires_internal_bearer_and_returns_descriptor_bound_plan(self):
        request = AgentPlanRequest(
            run_id="run-1",
            message="读取合同",
            tools=[
                {
                    "name": "lease.contract.get",
                    "version": "v1",
                    "description": "read contract",
                    "input_schema": {"type": "object"},
                }
            ],
        )
        llm_result = {
            "choices": [
                {
                    "message": {
                        "content": json.dumps(
                            {
                                "tool_calls": [
                                    {
                                        "call_id": "call-1",
                                        "tool_name": "lease.contract.get",
                                        "tool_version": "v1",
                                        "arguments": {"contract_id": "contract-1"},
                                    }
                                ]
                            }
                        )
                    }
                }
            ],
            "usage": {
                "prompt_tokens": 100,
                "completion_tokens": 25,
                "total_tokens": 125,
                "cost_usd": 0.000007,
            },
        }
        http_request = SimpleNamespace(headers={"authorization": "Bearer planner-secret"})
        with (
            patch("app.routers.agent_plan.get_settings", return_value=SimpleNamespace(agent_planner_token="planner-secret")),
            patch("app.routers.agent_plan.llm_client.chat_completion", new=AsyncMock(return_value=llm_result)),
            patch("app.routers.agent_plan.llm_client.get_model_name", return_value="deepseek/test"),
        ):
            response = await plan_agent_run(request, http_request)

        self.assertEqual(response.model, "deepseek/test")
        self.assertEqual(response.tool_calls[0].tool_name, "lease.contract.get")
        self.assertEqual(response.usage["schema_version"], "llm-usage.v1")
        self.assertEqual(response.usage["cost_status"], "measured")
        self.assertEqual(response.usage["cost_micros"], 7)

    async def test_planner_rejects_wrong_internal_bearer_before_llm_call(self):
        request = AgentPlanRequest(run_id="run-1", message="读取合同", tools=[{"name": "lease.contract.get", "version": "v1"}])
        http_request = SimpleNamespace(headers={"authorization": "Bearer wrong"})
        with (
            patch("app.routers.agent_plan.get_settings", return_value=SimpleNamespace(agent_planner_token="planner-secret")),
            patch("app.routers.agent_plan.llm_client.chat_completion", new=AsyncMock()) as completion,
        ):
            with self.assertRaises(HTTPException) as raised:
                await plan_agent_run(request, http_request)

        self.assertEqual(raised.exception.status_code, 401)
        completion.assert_not_awaited()


if __name__ == "__main__":
    unittest.main()
