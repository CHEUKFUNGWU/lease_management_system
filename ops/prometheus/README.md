# Agent Prometheus 配置

本目录提供可直接纳入 Prometheus 部署的 recording rules、告警 rules 和采集配置模板：

- `prometheus.yml.example`：Core Agent Gateway 的 scrape job。生产环境必须把最小权限 `agent_runtime:metrics` Token 通过 secret 注入到 `bearer_token_file`，不能写入 Git、Compose 文件或日志。
- `lease-agent.recording.yml`：5 分钟 Tool 失败率、平均延迟和 Review Gate 比率基线。
- `lease-agent.rules.yml`：失败率超过 20%、平均延迟超过 5 秒、成本价目不可用的告警。

部署前的最小检查：

```bash
promtool check config prometheus.yml
promtool check rules lease-agent.recording.yml lease-agent.rules.yml
```

默认阈值是技术基线，不是客户 SLA。上线前应根据至少一个完整结账周期的流量、Tool 分布和 LLM 价格版本校准，并配置告警接收人。
