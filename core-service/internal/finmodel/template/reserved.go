// Reserved sheet keys (D-F1): the sixteen row keys that tieouts.go reads by
// name (s.at("total_assets", period) …) and fold.go treats as period-end
// stocks. They are not a product preference — deleting or renaming one makes
// T1–T16 unrunnable, which is the reason this engine exists.
//
// The rejection copy must state the mechanical consequence, never frame it as
// a permission problem: a user told "无权限" will go ask for a permission that
// does not exist.
package template

import (
	"fmt"
	"strings"
)

// ReservedSheetKeys is the canonical list, single-sourced here;
// finmodel/fold.go derives its stock-key set from it.
var ReservedSheetKeys = []string{
	"cash", "ar", "inventory", "ppe", "rou_asset", "total_assets",
	"ap", "lease_liability", "borrowings", "total_liabilities",
	"share_capital", "retained_earnings", "total_equity",
	"nwc", "borrowings_opening", "ending_cash",
}

// ReservedKeysReason is the user-facing mechanical reason (D-F1). It names
// the structural consequence so the UI can explain why deletion/key-edit is
// impossible — the constraint has no external-product precedent, so the
// interface owes the user an explanation.
const ReservedKeysReason = "保留键是资产负债表勾稽的读取点，删除或改键后 T1–T16 无法运行"

// IsReservedSheetKey reports whether key carries tie-out semantics.
func IsReservedSheetKey(key string) bool {
	key = strings.TrimSpace(key)
	for _, reserved := range ReservedSheetKeys {
		if key == reserved {
			return true
		}
	}
	return false
}

// MissingReservedSheetKeys returns the reserved keys absent from keys, in
// canonical order. Empty means the sheet can run T1–T16.
func MissingReservedSheetKeys(keys []string) []string {
	present := make(map[string]bool, len(keys))
	for _, key := range keys {
		present[strings.TrimSpace(key)] = true
	}
	missing := make([]string, 0)
	for _, reserved := range ReservedSheetKeys {
		if !present[reserved] {
			missing = append(missing, reserved)
		}
	}
	return missing
}

// ErrMissingReservedKeys is the structured rejection for sheets that would
// break the tie-out contract. Detail lists every missing key.
type ErrMissingReservedKeys struct {
	Missing []string
}

func (e *ErrMissingReservedKeys) Error() string {
	return fmt.Sprintf("%s；缺失保留键：%s", ReservedKeysReason, strings.Join(e.Missing, ", "))
}

// Fold semantics for the fold field (F1 D-F4).
const (
	FoldStock = "stock"
	FoldFlow  = "flow"
)
