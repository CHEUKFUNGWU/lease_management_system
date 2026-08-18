# MAX-002 Review 1

> ⚠️ **ARCHIVED 2026-08-18 — 不是现行依据**
> 归档理由：零售 MVP（MAX-001~009）转型执行的过程记录，全部工单已 ACCEPTED；未完成的延后治理项见 docs/execution/精简MVP路线与延后治理清单.md
> 现行入口：`docs/AI_文档索引与现行决策.md`

结论：`CHANGES_REQUESTED`  
Reviewer：Codex 主任务  
日期：2026-08-12

## 已通过

- 039 在现有开发库连续执行两次安全；
- 完整 `db/init/01_init.sql` 在临时空库执行成功，模拟数据集表和门店来源字段存在；
- 真实 PostgreSQL 集成测试通过：默认 60 店 × 181 天 = 10,860 条事实，A/B 法人隔离、幂等重放/冲突、来源追溯及 IFRS 16/production 零触达成立；
- `go test ./...`、`go vet ./...`、`git diff --check` 全部通过；
- 固定参数产生稳定 dataset version、门店编码、业务哈希和六类 anomaly manifest；
- 模拟门店和事实均明确标识 simulated，未扩展高级审计。

## 阻断问题：异常数据与 manifest 语义不一致

这是经营产品的数据正确性问题，不是治理增强，因此必须在 MAX-003 KPI 开发前修复。

### 1. “客流连续下降”并非连续下降

位置：`core-service/internal/services/retailsimulation/generator.go` 的 `applyAnomalies`

当前逻辑用每日自身 baseline 乘递减 factor，baseline 中仍含星期和随机噪声，无法保证异常窗口逐日下降。Reviewer 用默认固定数据集实测：

```text
2026-01-25  footfall=263
2026-01-26  footfall=290
2026-01-27  footfall=288
2026-01-28  footfall=270
2026-01-29  footfall=255
2026-01-30  footfall=193
2026-01-31  footfall=151
```

首日到次日由 263 上升到 290，与 manifest 的 `footfall_continuous_decline` 和描述“each successive day”矛盾。

### 2. 销售异常制造虚假毛利率和变量租金率

位置：同一 `applyAnomalies`

客流、转化和客单异常只下调 revenue/transactions/footfall，没有按原毛利率重算 gross profit，也没有按既定公式重算 variable rent。默认固定数据实测：

- 客流下降窗口毛利率从约 32.9% 飙到 61.0%，变量租金率从约 1.29% 飙到 2.35%；
- 转化率下降窗口毛利率达到约 66.1%–82.4%，变量租金率固定约 2.86%；
- 客单价下降窗口毛利率达到约 47.6%–59.8%，变量租金率约 2.07%。

这会让 MAX-003/门店 360 把“销售下滑”错误解释成“毛利率大幅改善、变量租金率异常”，与交付报告所述毛利率约 27%–35%、变量租金 1.2% 不一致。

## 最小返工要求

1. 让 `footfall_continuous_decline` 在 manifest 指定窗口内逐日严格下降或非递增；实现不能依赖随机波动恰好下降；
2. 对 footfall、conversion、average ticket 三类销售驱动异常：保存原始毛利率和按收入计提的成本率，调整收入后至少重算 `gross_profit`、`variable_rent`；凡交付报告声明为收入固定比例的成本也应保持该比例；
3. 保持 margin compression 只压缩毛利率、labor spike 只提高人工成本、occupancy burden 只提高占用成本的主要信号，不制造违反 manifest 的反向信号；
4. 强化测试：对六个异常窗口的每一天与同日 baseline 比较，验证目标指标方向；对客流连续下降验证整个窗口序列；验证非目标毛利率/变量租金率保持预期区间；
5. 更新 Golden/业务哈希和 `docs/execution/reports/MAX-002.md` 的算法说明；
6. 重跑生成器、Handler、真实 PostgreSQL、全量 Go 测试、vet 和 diff check。

迁移、API 契约、法人隔离、幂等、审计及 IFRS 16 边界不要求改动。不得进入 MAX-003。
