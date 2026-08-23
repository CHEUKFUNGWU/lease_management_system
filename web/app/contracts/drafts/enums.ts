/**
 * Ch2 草稿复核工作台的枚举清单。
 *
 * CONTRACT-001：两份清单都登记进 `app/lib/code-lists-contract.test.ts`，
 * 与后端单一来源机械比对——
 *  - DRAFT_DATA_CLASSIFICATIONS ↔ db/init/01_init.sql 的
 *    ai_contract_drafts_classification_check（双向相等）；
 *  - DRAFT_REVIEW_STATUSES ⊆ draftreview 服务/repository 源内的状态字面量。
 */

/** ai_contract_drafts.status 的四个取值。 */
export const DRAFT_REVIEW_STATUSES = ["pending", "prepared", "approved", "rejected"] as const;
export type DraftReviewStatus = (typeof DRAFT_REVIEW_STATUSES)[number];

/** 数据分类（底线 2）：列表必须展示，模拟数据不得混入正式链路的观感。 */
export const DRAFT_DATA_CLASSIFICATIONS = ["production", "simulated", "mixed"] as const;
export type DraftDataClassification = (typeof DRAFT_DATA_CLASSIFICATIONS)[number];

export function isDraftReviewStatus(value: string): value is DraftReviewStatus {
  return (DRAFT_REVIEW_STATUSES as readonly string[]).includes(value);
}
