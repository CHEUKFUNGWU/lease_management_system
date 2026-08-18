import { describe, it, expect } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { EnterpriseTable } from "./EnterpriseTable";
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
  { key: "id", title: "合同编号", dataIndex: "id", fixed: "left" },
  { key: "store", title: "门店", dataIndex: "store" },
  { key: "liability", title: "租赁负债", dataIndex: "liability", align: "right" },
  { key: "discountRate", title: "折现率(%)", dataIndex: "discountRate", editable: true, editType: "number", align: "right" },
  { key: "status", title: "状态", dataIndex: "status" },
];

const mockSavedViews: SavedView<TestContract>[] = [
  { id: "review_only", name: "待复核", predicate: (c) => c.status === "review" },
  { id: "large_only", name: "大额(>500万)", predicate: (c) => c.liability > 5000000 },
];

describe("EnterpriseTable Deep Module", () => {
  it("renders table markup with sticky headers, columns, and saved view tabs", () => {
    const html = renderToStaticMarkup(
      <EnterpriseTable<TestContract>
        data={mockData}
        columns={mockColumns}
        rowKey={(r) => r.id}
        savedViews={mockSavedViews}
      />
    );

    // Saved Views rendered
    expect(html).toContain("已保存视图:");
    expect(html).toContain("全部");
    expect(html).toContain("待复核");
    expect(html).toContain("大额(&gt;500万)");

    // Headers and rows rendered
    expect(html).toContain("合同编号");
    expect(html).toContain("租赁负债");
    expect(html).toContain("C001");
    expect(html).toContain("上海淮海中路店");
    expect(html).toContain("C002");
    expect(html).toContain("C003");
  });

  it("renders empty state markup when dataset is empty", () => {
    const html = renderToStaticMarkup(
      <EnterpriseTable<TestContract>
        data={[]}
        columns={mockColumns}
        rowKey={(r) => r.id}
        emptyText="未找到符合条件的合同"
      />
    );

    expect(html).toContain("未找到符合条件的合同");
  });
});
