# 测试债专项执行报告 · gofmt 与测试 helper（2026-08-26）

> 对应任务：[测试债专项_gofmt与测试helper_2026-08-26.md](测试债专项_gofmt与测试helper_2026-08-26.md)。
> 两个 PR 的改动在本工作树中按文件域天然分离：T-A = 76 个 gofmt 文件 + CI 门禁；T-B = compare_test.go 单文件。

## T-A · gofmt 一次清零 + 门禁 —— ✅ 完成

**清零**：`gofmt -l` 实测 81 个漂移文件，其中 5 个 `third_party/picoclaw` 按红线排除，其余 **76 个**全部格式化。

**红线验证（比任务书要求的 grep 抽查更强）**：任务书给的「非空白字符变化为零」grep 在真实 gofmt 输出上不成立——gofmt 除空白外还会**重排 import 分组、拆单行 struct 声明**，这些在字符层面是非空白变化但正是 gofmt 规范动作。改用更强的机械判定：

```python
对 76 个文件逐一断言：open(新文件).read() == gofmt(git show HEAD:旧文件)
```

**76/76 逐字节相等**——整个 diff 恰好等于 gofmt 本身，无任何夹带。review 时 `git diff -w` 可进一步忽略行内空白。

**门禁**：`.github/workflows/ci.yml` Go job 内 `go vet` 之后新增一步：

```
test -z "$(gofmt -l . | grep -v third_party | grep -v gomodcache)"
```

排除名单带注释：third_party/picoclaw 是 vendor 切片（上游同步噪音），gomodcache 是本地模块缓存若落在仓库内的情况（本机实测确实存在 `.gomodcache/`，第一版门禁漏了它，本地模拟时被 3000+ 缓存文件淹没后修正）。

**闸门三态实测**：
1. 格式化后的树 → PASS；
2. 塞入一个未跟踪的漂移文件 → FAIL 并指名该文件（未跟踪新文件也在扫描范围）；
3. 清理后 → PASS；third_party 的 5 个有意漂移保持排除。

## T-B · compare_test.go helper 收敛 —— ✅ 完成（有一处超出预期的发现）

**发现（比任务书预设更糟）**：58 个 helper 不是「同义 float→string 函数」，而是一条 **57 层透传链终结于返回空串的 `fin5`**——每层都是 `"" + next(value)`，所以整条链坍缩为空串。唯一真实调用点
`TestCompare_ConclusionNamesTheWinnerAndTheGap` 里的

```go
strings.Contains(result.Conclusion, formatGap(other-winner))
```

因 `Contains(s, "")` 恒真而成为**恒真空检**——"conclusion 必须引用真实 gap" 这条断言从未真正起过作用。

按任务规则「停下来核对原输出」处置：期望值没有可保留的输出（是空串），测试名与注释给出了 evident intent——conclusion 用 `round2` + `%.2f` 写入 gap（对照 `compare.go` 的 `conclude()` 确认）。据此留恰好一个 helper 还原真实比对，57 个死层全删：

```go
func formatGap(value float64) string { return fmt.Sprintf("%.2f", round2(value)) }
```

还原后 **8/8 测试通过**（生产的 conclusion 确实正确引用了 gap=60835.72）——断言从恒真恢复为真实比对，期望值本身未改。代码注释里记录了这段历史。

**GUARD-001 实测**（有个细节值得记录）：把 `%.2f` 改成 `%.1f` 测试**不会红**——gap=60835.72 时 `"60835.7"` 是 `"60835.72"` 的前缀，Contains 仍真。改用非前缀破坏 `%.0f`（"60836"）→ 测试红并报 "does not quote the actual gap 60835.72"；恢复 → 绿。

**验收对照**：
- 文件内 func 数 = **11** = 8 真测试 + offerA/offerB 两个 fixture builder + 恰好 1 个格式化 helper（任务书 ≤9 的公式没算 fixture；「至多 1 个格式化 helper」的本意满足）；
- `go test ./services/leasescenario/` 绿；ratio_golden.json 零 diff ✅
- 文件从 450 行降到 235 行。

## 最终验证

```bash
cd core-service && GOCACHE=$(pwd)/.gocache go test ./...   # ✅ 全绿
GOCACHE=$(pwd)/.gocache go vet ./...                        # ✅
gofmt -l . | grep -v third_party | grep -v gomodcache       # ✅ 空
工具清单 / deals / ingest golden                            # ✅ 零 diff
```

web 本轮零改动（上一专项已全绿）。

## 明确不做（照任务书执行）

- 未重构 compare_test.go 的表驱动结构；
- third_party 5 个文件未动；
- 其它测试文件的同义 helper 问题未扩 scope（如发现请登记到下一个测试债专项）。
