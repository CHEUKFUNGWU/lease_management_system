"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { AutoComplete, Input, Tag } from "antd";
import { FileTextOutlined, SearchOutlined } from "@ant-design/icons";
import { useRouter } from "next/navigation";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { contractApi } from "../lib/api";
import { t } from "../lib/i18n";

interface SearchableContract {
  id: string;
  contract_number: string;
  contract_name: string;
  store_name?: string;
  lessor_name?: string;
}

export default function GlobalSearch() {
  const router = useRouter();
  const { token } = useAuth();
  const { language } = useLanguage();
  const [focused, setFocused] = useState(false);
  const [keyword, setKeyword] = useState("");
  const [contracts, setContracts] = useState<SearchableContract[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const inputRef = useRef<any>(null);

  // ⌘K / Ctrl+K focuses the search box
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        inputRef.current?.focus();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  const ensureLoaded = async () => {
    if (loaded || loading || !token) return;
    setLoading(true);
    try {
      const res = await contractApi.list(token);
      setContracts(res.data || []);
      setLoaded(true);
    } catch (error) {
      console.error("Global search failed to load contracts:", error);
    } finally {
      setLoading(false);
    }
  };

  const options = useMemo(() => {
    const query = keyword.trim().toLowerCase();
    if (!query) return [];
    const matches = contracts
      .filter((contract) => {
        const haystack = [
          contract.contract_number,
          contract.contract_name,
          contract.store_name,
          contract.lessor_name,
        ]
          .filter(Boolean)
          .join(" ")
          .toLowerCase();
        return haystack.includes(query);
      })
      .slice(0, 8);

    if (!matches.length) {
      return [
        {
          value: "__empty__",
          disabled: true,
          label: (
            <span style={{ color: "#BFBFBF", fontSize: 13 }}>
              {loading ? t("search.loading", language) : t("search.no_results", language)}
            </span>
          ),
        },
      ];
    }

    return matches.map((contract) => ({
      value: contract.id,
      label: (
        <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "2px 0" }}>
          <FileTextOutlined style={{ color: "#8C8C8C", fontSize: 13, flexShrink: 0 }} />
          <span style={{ fontWeight: 500, fontSize: 13, whiteSpace: "nowrap" }}>
            {contract.contract_number}
          </span>
          <span
            style={{
              fontSize: 13,
              color: "#595959",
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
              flex: 1,
            }}
          >
            {contract.contract_name}
          </span>
          {contract.store_name && (
            <Tag style={{ fontSize: 11, marginRight: 0, flexShrink: 0 }}>{contract.store_name}</Tag>
          )}
        </div>
      ),
    }));
  }, [contracts, keyword, language, loading]);

  return (
    <div
      style={{
        position: "relative",
        width: focused ? 320 : 200,
        transition: "width 0.2s cubic-bezier(0.4, 0, 0.2, 1)",
      }}
    >
      <AutoComplete
        value={keyword}
        options={options}
        onChange={(value) => setKeyword(value)}
        onSelect={(value) => {
          if (value === "__empty__") return;
          setKeyword("");
          router.push(`/contracts/${value}`);
        }}
        style={{ width: "100%" }}
        popupMatchSelectWidth={420}
      >
        <Input
          ref={inputRef}
          placeholder={t("search.placeholder", language)}
          prefix={<SearchOutlined style={{ color: "#8C8C8C", fontSize: 14 }} />}
          onFocus={() => {
            setFocused(true);
            ensureLoaded();
          }}
          onBlur={() => setFocused(false)}
          allowClear
          style={{
            borderRadius: 9999,
            background: "#F5F5F5",
            border: "none",
            fontSize: 13,
            height: 34,
            paddingLeft: 14,
          }}
        />
      </AutoComplete>
      {!focused && (
        <kbd
          style={{
            position: "absolute",
            right: 10,
            top: "50%",
            transform: "translateY(-50%)",
            fontSize: 10,
            color: "#BFBFBF",
            background: "#fff",
            border: "1px solid #E5E5E5",
            borderRadius: 4,
            padding: "0 5px",
            lineHeight: "16px",
            fontFamily: "monospace",
            pointerEvents: "none",
          }}
        >
          ⌘K
        </kbd>
      )}
    </div>
  );
}
