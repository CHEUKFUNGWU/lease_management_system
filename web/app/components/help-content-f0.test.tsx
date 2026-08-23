/**
 * F2-1（任务指令：财务视角的 UI/UX 与术语整改）：术语最密的三个页面
 * （financial-model / store-pnl / monthly-closing）的帮助内容测试。
 *
 * 内容口径：回答财务用户的真实疑问（三道闸拦什么、勾稽不过下一步、
 * 发布谁能看到；两口径为何不同、Decision Ready 不足还能不能用；与总账
 * 的关系、哪些动作不可逆），不是复述界面。
 *
 * 注：HelpDrawer 的 Drawer 体在 open 前不渲染（antd 懒加载），SSR 只出
 * 触发按钮，所以这里断言 HelpContent 对象本身；触发按钮的渲染另有一条
 * SSR 断言。文案全部经 t()，三语完整性由 i18n-keys 测试守护。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { HelpTrigger } from "./HelpDrawer";
import {
  financialModelHelpContent,
  monthlyClosingHelpContent,
  storePnlHelpContent,
} from "./help-content";

const language = "zh-CN" as const;

function renderTrigger(content: Parameters<typeof HelpTrigger>[0]["content"]) {
  return renderToStaticMarkup(React.createElement(HelpTrigger, { content, language }));
}

describe("F2-1 三页帮助内容", () => {
  it("financial-model：flow 复用页面自身 ①–⑤ 步骤键，sections 回答三问", () => {
    const content = financialModelHelpContent(language);
    // flow 与页面卡片编号同源（finmodel.step_*），不另造一套
    expect(content.flow.map((s) => s.label)).toEqual([
      "① 选模型",
      "② 填假设",
      "③ 校期初",
      "④ 运行",
      "⑤ 发布与导出",
    ]);
    expect(content.title).toBe("三表财务模型使用指南");
    expect(content.sections.map((s) => s.heading)).toEqual([
      "期初三道闸各拦什么",
      "勾稽不过时下一步做什么",
      "发布的计划版本谁能看到",
    ]);
    expect(content.sections[0].body).toContain("闸③");
    expect(content.sections[0].body).toContain("引擎是唯一权威");
    expect(content.sections[1].body).toContain("缺口清单");
    expect(content.sections[2].body).toContain("模拟数据的测算不能发布");
  });

  it("store-pnl：解释双口径差异与 Decision Ready 降级，保留口径澄清", () => {
    const content = storePnlHelpContent(language);
    expect(content.title).toBe("单店利润表使用指南");
    expect(content.sections.map((s) => s.heading)).toEqual([
      "经营口径与 IFRS 16 口径为什么不一样",
      "Decision Ready 不满足时这张表还能不能用",
      "每个数字从哪里来",
    ]);
    expect(content.sections[0].body).toContain("永不合计");
    expect(content.sections[1].body).toContain("不要据此做同业对比或考核结论");
    expect(content.sections[1].body).toContain("那代表没有数据，不是零成本");
    expect(content.sections[2].body).toContain("事实版本区间");
  });

  it("monthly-closing：说明与总账的关系和不可逆动作", () => {
    const content = monthlyClosingHelpContent(language);
    expect(content.title).toBe("月结中心使用指南");
    expect(content.flow.map((s) => s.label)).toEqual([
      "选期间与范围",
      "生成分录并复核",
      "审批确认",
      "过账回写并锁账",
    ]);
    expect(content.sections[0].body).toContain("过账才把分录写入总账并回传凭证号");
    expect(content.sections[1].heading).toBe("哪些动作不可逆");
    expect(content.sections[1].body).toContain("已锁定的期间不能覆盖");
    expect(content.sections[2].body).toContain("不是健康度评分");
  });

  it("三个内容的全部文案三语非空", () => {
    for (const [fn, lang] of [
      [financialModelHelpContent, "en"],
      [storePnlHelpContent, "zh-HK"],
      [monthlyClosingHelpContent, "en"],
      [financialModelHelpContent, "zh-HK"],
      [monthlyClosingHelpContent, "zh-CN"],
    ] as const) {
      const content = fn(lang);
      expect(content.title.length, `title[${lang}]`).toBeGreaterThan(0);
      for (const step of content.flow) expect(step.label.length, `flow[${lang}]`).toBeGreaterThan(0);
      for (const section of content.sections) {
        expect(section.heading.length, `heading[${lang}]`).toBeGreaterThan(0);
        expect(section.body.length, `body[${lang}]`).toBeGreaterThan(0);
      }
    }
  });

  it("帮助触发按钮可渲染且带 aria-label（无障碍）", () => {
    const html = renderTrigger(financialModelHelpContent(language));
    expect(html).toContain('aria-label="使用教程"');
  });
});
