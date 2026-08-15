"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { classifyDataState, type ClassifyInput, type DataState } from "../lib/dataState";

/**
 * FETCH-001: 零售页的取数接缝。统一 loading / 竞态 / token 注入，错误出口
 * 直接产出 STATE-001 的三分法结果（DataState）。参考 `contracts/[id]` 与
 * `ai-chat` 既有接缝的形态（transport + 受控状态），但面向查询场景——
 * 三个零售页不再各自手搓 `let active` / requestGate / useRef 竞态。
 *
 * 用法：
 *   const { loading, state, retry } = useRetailQuery({
 *     token, params, paramsKey,   // paramsKey 是参数字符串键（对象引用会变）
 *     fetcher: (params, token) => retailAnalyticsApi.xxx(params, token),
 *     actionFor, isEmpty,         // STATE-001 判定覆盖
 *   });
 *
 * 其余 35 个直接 import lib/api 的文件不在本接缝范围（渐进迁移，另开票）。
 */

export interface RetailQueryOptions<T, P> {
  token: string | null;
  /** null → 查询禁用（例如还没选门店）。 */
  params: P | null;
  /** 参数字符串键：对象引用每次渲染都变，依赖必须用字符串。 */
  paramsKey: string;
  fetcher: (params: P, token: string) => Promise<T>;
  actionFor?: ClassifyInput<T>["actionFor"];
  isEmpty?: ClassifyInput<T>["isEmpty"];
  /** 额外依赖（如分类切换的其它状态）。 */
  deps?: unknown[];
}

export function useRetailQuery<T, P>(options: RetailQueryOptions<T, P>) {
  const { token, params, paramsKey, fetcher, actionFor, isEmpty, deps = [] } = options;
  const [loading, setLoading] = useState(false);
  const [state, setState] = useState<DataState<T>>({ kind: "empty" });
  const [retryNonce, setRetryNonce] = useState(0);
  // 竞态门：只接受最新一次请求的结果；过期响应即使 resolve 也不 setState。
  const seq = useRef(0);
  const paramsRef = useRef(params);
  paramsRef.current = params;

  useEffect(() => {
    if (!token || paramsRef.current == null) {
      setState({ kind: "empty" } as DataState<T>);
      setLoading(false);
      return;
    }
    const id = ++seq.current;
    let active = true;
    setLoading(true);
    fetcher(paramsRef.current, token).then(
      (data) => {
        if (!active || id !== seq.current) return;
        setState(classifyDataState<T>({ error: null, data, isEmpty }));
        setLoading(false);
      },
      (error) => {
        if (!active || id !== seq.current) return;
        setState(classifyDataState<T>({ error, data: null, actionFor }));
        setLoading(false);
      },
    );
    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, paramsKey, retryNonce, ...deps]);

  const retry = useCallback(() => setRetryNonce((value) => value + 1), []);

  return { loading, state, retry };
}
