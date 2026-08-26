# 测试债小专项 · gofmt 漂移与测试 helper 收敛（2026-08-26）

> 来源：架构重构 C2 执行报告申报的未顺手清存量。
> 原则：ponytail —— 两笔都是机械债，最小 diff 清掉，顺手加一道闸防复发。
> 工期预估：半天。

## 债务盘点（2026-08-26 实测）

| 债 | 规模 | 成因 |
|---|---|---|
| gofmt 漂移 | **76 个非 third_party 文件**（`gofmt -l` 实测） | CI 从未有过 gofmt 门禁 |
| compare_test.go 格式化 helper | **58 个**（450 行文件里 8 个是真测试） | 每写一个断言顺手造一个新的 float→string 函数：fin / val / toTwo / sprintTwo / sprintf / simpleFormat / trimZero / twoDecimals / valueTwo / writeTwo / theFormat / str / trueFormat / strconvFormat…全部是同义 |

---

## T-A · gofmt 一次清零 + 一道闸 【一个 PR】

**为什么不用「先闸门后清债」**：`gofmt -w` 是确定性机械操作，没有人工判断窗口，不存在「边清边涨」——与 UIUX 任务书 T3（Tag 替换需逐处判断极性）情况不同，一个 PR 同时做清和闸是安全的。

**步骤**：
```bash
cd core-service && gofmt -w $(gofmt -l . | grep -v gomodcache | grep -v third_party)
```

**加门禁**（二选一，选改动小的）：
- `Makefile` 加 `lint:` 目标：`gofmt -l . | grep -v third_party | tee /dev/stderr | grep -q . && exit 1 || true` 的等价正确写法；
- 或 `.github/workflows/ci.yml` 现有 Go job 里加一行 `test -z "$(gofmt -l . | grep -v third_party)"`。

**红线**：
- ❌ **不碰 `internal/agentkernel/third_party/picoclaw/**`**（5 个漂移文件是有意的）——vendor 切片代码，改格式会给上游同步制造噪音。门禁的排除名单里要有它，并注释原因。
- ❌ gofmt -w 产生的 diff 里不允许出现任何非空白字符变化。验收时抽查：`git diff | grep -E "^[+-]" | grep -vE "^[+-]{3}" | grep -vE "^\+\s*$|^-\s*$"` 应为空或仅空白差异。

**验收**：
- `gofmt -l . | grep -v third_party` 输出为空；
- `go test ./... && go vet ./...` 全绿（87 包基线）；
- golden 快照零 diff。

## T-B · compare_test.go 收敛到至多 1 个格式化 helper 【一个 PR】

**修法（最小 diff）**：
1. 58 个同义 helper 里留**恰好一个**（建议保留语义最直白的 `twoDecimals`，或干脆全删、断言处直接写 `strconv.FormatFloat(v, 'f', 2, 64)`——哪个 diff 小选哪个）;
2. 其余 57 个：全局替换调用点后删除函数。纯机械替换，不改任何测试的期望值。
3. 若替换后发现多个 helper 输出格式不一致（有的去尾零有的不去）——**停下来**，那说明测试期望值本身依赖了格式差异，逐个核对原输出后再统一，不许静默改期望值。

**验收**：
- `grep -c "^func " internal/services/leasescenario/compare_test.go` ≤ 9（8 测试 + 至多 1 helper）；
- `go test ./services/leasescenario/` 绿，golden（ratio_golden.json）不变；
- GUARD-001：把留下的那个 helper 改错一位小数，测试必须红。

---

## 明确不做

- 不重构 compare_test.go 的测试结构本身（表驱动化之类是另一回事，这次只删重复 helper）；
- 不动 third_party；
- 不给其它测试文件的类似问题扩 scope——其他文件若也有同义 helper，登记不清理，等下一个专项。

## 执行顺序

T-A 先（76 文件纯格式 diff 单独成 PR，review 时可直接 `git diff -w` 忽略）；T-B 后（逻辑 diff 干净可读）。两个 PR 都过 `go test ./... && go vet ./...` 全绿门。
