"use client";

import { Button, Input, Space } from "antd";
import { DeleteOutlined } from "@ant-design/icons";
import { t, type Language } from "../lib/i18n";
import type { OpeningContractRow } from "./workbench";

/**
 * F0-1（任务指令：财务视角的 UI/UX 与术语整改）：期初合约行的三个输入
 * 此前只有 placeholder，没有可视 label——用户开始输入后字段语义就消失了，
 * 而「租赁负债」与「使用权资产」两框左右相邻、外观相同，串行填反正是这张
 * 表要抓的头号错误。照 UIUX 审查报告 §9 P2-A 登录页的做法补可视 label：
 *
 * - 第一行渲染 `<label htmlFor>`，后续行只挂 aria-label（避免重复噪音，
 *   但 aria-label 不替代第一行的可视 label）；
 * - placeholder 降级为填写示例，不再兼任字段语义；
 * - 字段语义（label 键）与示例（placeholder 键)分开登记，删掉任一半边
 *   都会让 contract-rows.test.tsx 变红。
 */
const FIELDS = [
  {
    field: "contract_id",
    labelKey: "finmodel.opening_contract",
    exampleKey: "finmodel.opening_contract_example",
    className: "fm-input-contract",
  },
  {
    field: "lease_liability",
    labelKey: "finmodel.opening_liability",
    exampleKey: "finmodel.opening_liability_example",
    className: "fm-input-amount",
  },
  {
    field: "rou_asset",
    labelKey: "finmodel.opening_rou",
    exampleKey: "finmodel.opening_rou_example",
    className: "fm-input-amount",
  },
] as const;

export type ContractRowField = (typeof FIELDS)[number]["field"];

export function ContractRowInputs({
  row,
  index,
  idPrefix,
  language,
  onChange,
  onRemove,
}: {
  row: OpeningContractRow;
  index: number;
  /** DOM id 前缀：同一页面有两张合约子表（lease_ref / engine），id 必须互斥。 */
  idPrefix: string;
  language: Language;
  onChange: (field: ContractRowField, value: string) => void;
  onRemove: () => void;
}) {
  const isFirst = index === 0;
  return (
    <Space wrap>
      {FIELDS.map(({ field, labelKey, exampleKey, className }) => {
        const labelText = t(labelKey, language);
        const inputId = `${idPrefix}-${index}-${field}`;
        return (
          <span key={field} className="fm-field">
            {isFirst && (
              <label className="fm-field-label" htmlFor={inputId}>
                {labelText}
              </label>
            )}
            <Input
              id={inputId}
              className={className}
              aria-label={isFirst ? undefined : labelText}
              placeholder={t(exampleKey, language)}
              value={row[field]}
              onChange={(e) => onChange(field, e.target.value)}
            />
          </span>
        );
      })}
      <Button
        size="small"
        type="text"
        icon={<DeleteOutlined />}
        aria-label={t("finmodel.remove", language)}
        onClick={onRemove}
      />
    </Space>
  );
}
