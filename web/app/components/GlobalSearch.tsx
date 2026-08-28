"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { KeyboardEvent as ReactKeyboardEvent } from "react";
import { Button, Input, Modal, Space, Tag } from "antd";
import type { InputRef } from "antd";
import { ArrowRightOutlined, FileTextOutlined, SearchOutlined } from "@ant-design/icons";
import { useRouter } from "next/navigation";
import { hasRole, useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { contractApi, masterDataApi } from "../lib/api";
import { t } from "../lib/i18n";
import { PALETTE_PAGES } from "../lib/palette";

interface SearchableContract {
  id: string;
  contract_number: string;
  contract_name: string;
  store_name?: string;
  lessor_name?: string;
}

interface SearchableStore {
  id: string;
  code: string;
  name: string;
  brand?: string;
  region?: string;
}

interface CommandItem {
  id: string;
  label: string;
  description?: string;
  path: string;
  kind: "page" | "action" | "contract" | "store";
}

export default function GlobalSearch() {
  const router = useRouter();
  const { user, token } = useAuth();
  const { language } = useLanguage();
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState("");
  const [contracts, setContracts] = useState<SearchableContract[]>([]);
  const [stores, setStores] = useState<SearchableStore[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<InputRef>(null);

  // 页面路由来自 lib/palette.ts 注册表（U3 测试保证每个业务页面都已登记），
  // 可见性遵循与 AppLayout useMenuItems 相同的角色规则。
  const pageItems = useMemo<CommandItem[]>(() =>
    PALETTE_PAGES.filter((def) => def.visible(user)).map((def) => ({
      id: `page-${def.path}`,
      label: t(def.labelKey, language),
      description: t(`search.group_${def.group}`, language),
      path: def.path,
      kind: "page" as const,
    })),
  [language, user]);

  const actionItems = useMemo<CommandItem[]>(() => [
    { id: "action-new-contract", label: t("search.action_new_contract", language), path: "/contracts/new", kind: "action" },
    { id: "action-ai-entry", label: t("search.action_ai_entry", language), path: "/ai-chat", kind: "action" },
    { id: "action-todo", label: t("search.action_todo", language), path: "/todo", kind: "action" },
    { id: "action-reports", label: t("search.action_reports", language), path: "/reports", kind: "action" },
  ], [language]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setOpen(true);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  useEffect(() => {
    if (!open || !token) return;
    const query = keyword.trim();
    if (query.length < 2) {
      setContracts([]);
      setLoading(false);
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(async () => {
      setLoading(true);
      try {
        const res = await contractApi.list<{ data?: SearchableContract[] }>(token, { search: query, page: 1, page_size: 8 });
        if (!cancelled) setContracts(res.data || []);
      } catch {
        if (!cancelled) setContracts([]);
      }
      // 门店来源复用 master-data/stores（服务端按权限 scope，无需分类参数；
      // storeOptions 的 data_classification 语义对全局导航面板不自然）。
      try {
        const storeRes = (await masterDataApi.listStores(token)) as { stores?: SearchableStore[] };
        if (!cancelled) setStores(storeRes.stores || []);
      } catch {
        if (!cancelled) setStores([]);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }, 180);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [keyword, open, token]);

  const items = useMemo<CommandItem[]>(() => {
    const query = keyword.trim().toLowerCase();
    const staticItems = [...actionItems, ...pageItems].filter((item) => {
      if (!query) return true;
      return `${item.label} ${item.description || ""}`.toLowerCase().includes(query);
    });
    const contractItems = contracts
      .filter((contract) => `${contract.contract_number} ${contract.contract_name} ${contract.store_name || ""} ${contract.lessor_name || ""}`.toLowerCase().includes(query))
      .map((contract) => ({
        id: `contract-${contract.id}`,
        label: contract.contract_number,
        description: contract.contract_name,
        path: `/contracts/${contract.id}`,
        kind: "contract" as const,
      }));
    const storeItems = stores
      .filter((store) => `${store.code} ${store.name} ${store.brand || ""} ${store.region || ""}`.toLowerCase().includes(query))
      .map((store) => ({
        id: `store-${store.id}`,
        label: store.code,
        description: store.name,
        path: `/store-360?store_id=${encodeURIComponent(store.id)}`,
        kind: "store" as const,
      }));
    return [...staticItems, ...contractItems, ...storeItems].slice(0, 14);
  }, [actionItems, contracts, keyword, pageItems, stores]);

  useEffect(() => {
    setSelectedIndex(0);
  }, [keyword, open]);

  const close = () => {
    setOpen(false);
    setKeyword("");
    setSelectedIndex(0);
  };

  const navigateTo = (item: CommandItem) => {
    close();
    router.push(item.path);
  };

  const handleKeyDown = (event: ReactKeyboardEvent<HTMLInputElement>) => {
    if (!items.length) return;
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setSelectedIndex((index) => (index + 1) % items.length);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setSelectedIndex((index) => (index - 1 + items.length) % items.length);
    } else if (event.key === "Enter") {
      event.preventDefault();
      navigateTo(items[selectedIndex]);
    }
  };

  const kindLabel = (kind: CommandItem["kind"]) => {
    if (kind === "contract") return t("search.group_contracts", language);
    if (kind === "page") return t("search.group_pages", language);
    if (kind === "store") return t("search.group_stores", language);
    return t("search.group_actions", language);
  };

  return (
    <>
      <Button
        className="global-search-trigger"
        type="text"
        icon={<SearchOutlined />}
        onClick={() => setOpen(true)}
        aria-label={t("search.open_command_palette", language)}
      >
        <span>{t("search.placeholder_short", language)}</span>
        <kbd>⌘K</kbd>
      </Button>
      <Modal
        open={open}
        onCancel={close}
        footer={null}
        title={t("search.command_title", language)}
        width={620}
        destroyOnHidden
        afterOpenChange={(visible) => { if (visible) window.setTimeout(() => inputRef.current?.focus(), 0); }}
      >
        <Input
          ref={inputRef}
          autoFocus
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={t("search.command_placeholder", language)}
          prefix={<SearchOutlined style={{ color: "var(--fg-muted)" }} />}
          suffix={loading ? t("search.loading", language) : <kbd>↑↓ Enter</kbd>}
          size="large"
        />
        <div className="command-palette-results" role="listbox" aria-label={t("search.command_title", language)}>
          {items.length === 0 ? (
            <div className="command-palette-empty">{loading ? t("search.loading", language) : t("search.no_results", language)}</div>
          ) : items.map((item, index) => (
            <button
              key={item.id}
              type="button"
              className={`command-palette-item${index === selectedIndex ? " is-selected" : ""}`}
              onMouseEnter={() => setSelectedIndex(index)}
              onClick={() => navigateTo(item)}
              role="option"
              aria-selected={index === selectedIndex}
            >
              <span className="command-palette-item-icon"><FileTextOutlined /></span>
              <span className="command-palette-item-copy">
                <strong>{item.label}</strong>
                {item.description && <span>{item.description}</span>}
              </span>
              <Tag>{kindLabel(item.kind)}</Tag>
              <ArrowRightOutlined className="command-palette-item-arrow" />
            </button>
          ))}
        </div>
        <Space className="command-palette-hint" size={16}>
          <span>{t("search.command_keyboard_hint", language)}</span>
          <span>{t("search.command_scope_hint", language)}</span>
        </Space>
      </Modal>
    </>
  );
}
