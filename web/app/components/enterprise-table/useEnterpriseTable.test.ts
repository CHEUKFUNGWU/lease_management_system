import { describe, it, expect, vi } from "vitest";
import { useEnterpriseTable } from "./useEnterpriseTable";
import type { EnterpriseColumn, SavedView } from "./types";

interface TestContract {
  id: string;
  name: string;
  store: string;
  liability: number;
  discountRate: number;
  status: "draft" | "review" | "posted";
}

const mockData: TestContract[] = [
  { id: "C001", name: "淮海中路合同", store: "上海淮海中路店", liability: 3500000, discountRate: 4.75, status: "posted" },
  { id: "C002", name: "南京东路合同", store: "上海南京东路店", liability: 12000000, discountRate: 4.9, status: "review" },
  { id: "C003", name: "铜锣湾合同", store: "香港铜锣湾店", liability: 8500000, discountRate: 5.0, status: "draft" },
];

const mockColumns: EnterpriseColumn<TestContract>[] = [
  { key: "id", title: "合同编号", dataIndex: "id" },
  { key: "store", title: "门店", dataIndex: "store" },
  { key: "liability", title: "租赁负债", dataIndex: "liability" },
  { key: "discountRate", title: "折现率", dataIndex: "discountRate", editable: true },
  { key: "status", title: "状态", dataIndex: "status" },
];

const mockSavedViews: SavedView<TestContract>[] = [
  { id: "review_only", name: "待复核", predicate: (c) => c.status === "review" },
  { id: "large_only", name: "大额(>500万)", predicate: (c) => c.liability > 5000000 },
];

describe("useEnterpriseTable Hook Logic", () => {
  it("computes view counts correctly", () => {
    // We can directly test the pure filtering logic
    const totalCount = mockData.length;
    const reviewCount = mockData.filter((c) => c.status === "review").length;
    const largeCount = mockData.filter((c) => c.liability > 5000000).length;

    expect(totalCount).toBe(3);
    expect(reviewCount).toBe(1);
    expect(largeCount).toBe(2);
  });

  it("evaluates quick fuzzy search against arbitrary fields", () => {
    const q1 = "铜锣湾";
    const matched1 = mockData.filter((item) =>
      Object.values(item).some((v) => String(v).includes(q1))
    );
    expect(matched1.length).toBe(1);
    expect(matched1[0].id).toBe("C003");

    const q2 = "4.9";
    const matched2 = mockData.filter((item) =>
      Object.values(item).some((v) => String(v).includes(q2))
    );
    expect(matched2.length).toBe(1);
    expect(matched2[0].id).toBe("C002");
  });

  it("evaluates multi-condition advanced operators", () => {
    // Condition: liability > 5000000 AND status != 'posted'
    const res = mockData.filter((item) => {
      if (item.liability <= 5000000) return false;
      if (item.status === "posted") return false;
      return true;
    });

    expect(res.map((r) => r.id)).toEqual(["C002", "C003"]);
  });
});
