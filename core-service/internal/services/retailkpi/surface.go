package retailkpi

import (
	"fmt"
	"sort"
	"strings"
)

// RH1 Metric Surface（模块设计 §3，D-R10）：指标露出做成受校验的清单，
// 不是自由字符串切片。
//
// 背景：retailstore360 曾同时维护两张中文名表（本包 chineseNames 与
// store360 的 labels map）加两个裸 []string 清单，加一个指标要改四处、
// 漏一处的表现是前端显示指标码。收敛后：
//
//   - 本文件的标签表是唯一真相源（Label 是唯一取名入口）；
//   - 每个页面区块持有 Surface 值，构造时经 ValidateSurface 校验闭包——
//     清单里的 code 必须真的有 Definition，否则启动即失败（CI 可强制的
//     不变量，不靠"记得四处都改"）。

// Surface 声明一个页面区块要露出哪些指标。
type Surface struct {
	Codes []string
}

// Label 返回指标的中文名。唯一的标签真相源。
// 未定义的 code 返回 ("", false)——调用方据此渲染「未识别指标」，
// 不静默显示码值。
func Label(code string) (string, bool) {
	name, ok := chineseNames[code]
	return name, ok
}

// ValidateSurface 在启动时校验清单闭包：任一 code 没有 Definition 即返回
// 错误，逐个列出全部未定义项（不是只报第一个），让修清单的人一次改完。
func ValidateSurface(s Surface) error {
	missing := make([]string, 0)
	for _, code := range s.Codes {
		if findDefinition(code) == nil {
			missing = append(missing, code)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("retailkpi: surface references undefined metric codes: %s", strings.Join(missing, ", "))
}
