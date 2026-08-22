// RH2 Display Basis Guard（模块设计 §4，D-R11）：一个数值在某个展示语境下
// 是否可用。口径不一致就不可用——不存在「设备口径 → 零售口径」的换算，
// 所以只有两条路：可用则显示，不可用则显示具名空态/「—」加原因。
// 给第三条路（估算、近似、改名）就会有人走，R0-1 之前 /performance 走的正是那条路。
//
// metricCode 不参与判定；它的作用是让调用点自证「我在拿哪个指标做这件事」，
// 并为将来按指标的例外规则留挂点。

export type Basis = "retail_store" | "equipment" | "unknown";

export type BasisResolution = { usable: true } | { usable: false; reasonKey: string };

/** 口径不一致（或来源口径未知）时不可用，reasonKey 指向一条三语 i18n 文案。 */
export function resolveBasis(
  metricCode: string,
  sourceBasis: Basis,
  displayContext: Basis,
): BasisResolution {
  void metricCode;
  // 来源口径 unknown 时同样不可用：不知道是什么口径的数，贴上任何语境标题
  // 都是在伪造语义。
  if (sourceBasis !== "unknown" && sourceBasis === displayContext) {
    return { usable: true };
  }
  return { usable: false, reasonKey: "lib.display_basis.mismatch" };
}
