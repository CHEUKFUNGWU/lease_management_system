import type { FPnAPlanVersion, PlanVersionStatus, VersionHierarchyNode } from "./types";

export function buildVersionTree(versions: FPnAPlanVersion[]): VersionHierarchyNode[] {
  if (!versions || versions.length === 0) return [];

  const byId = new Map<string, FPnAPlanVersion>();
  const childrenMap = new Map<string, FPnAPlanVersion[]>();

  for (const v of versions) {
    byId.set(v.id, v);
    if (v.prior_version_id) {
      const list = childrenMap.get(v.prior_version_id) || [];
      list.push(v);
      childrenMap.set(v.prior_version_id, list);
    }
  }

  const visited = new Set<string>();

  function buildNode(version: FPnAPlanVersion, level: number): VersionHierarchyNode {
    visited.add(version.id);
    const childVersions = childrenMap.get(version.id) || [];
    const children: VersionHierarchyNode[] = [];

    for (const child of childVersions) {
      if (!visited.has(child.id)) {
        children.push(buildNode(child, level + 1));
      }
    }

    // Sort children by created_at ascending (chronological lineage)
    children.sort((a, b) => new Date(a.version.created_at).getTime() - new Date(b.version.created_at).getTime());

    return {
      version,
      children,
      level,
    };
  }

  const roots: VersionHierarchyNode[] = [];
  for (const v of versions) {
    if (!v.prior_version_id || !byId.has(v.prior_version_id)) {
      if (!visited.has(v.id)) {
        roots.push(buildNode(v, 0));
      }
    }
  }

  // Handle any orphan / unvisited cycles
  for (const v of versions) {
    if (!visited.has(v.id)) {
      roots.push(buildNode(v, 0));
    }
  }

  // Sort roots by as_of_period desc, created_at desc
  roots.sort((a, b) => {
    if (a.version.as_of_period !== b.version.as_of_period) {
      return b.version.as_of_period.localeCompare(a.version.as_of_period);
    }
    return new Date(b.version.created_at).getTime() - new Date(a.version.created_at).getTime();
  });

  return roots;
}

export function isValidStatusTransition(current: PlanVersionStatus, next: PlanVersionStatus): boolean {
  if (current === next) return true;
  switch (current) {
    case "draft":
      return next === "review" || next === "approved" || next === "official" || next === "retired";
    case "review":
      return next === "approved" || next === "draft" || next === "retired";
    case "approved":
      return next === "official" || next === "retired";
    case "official":
      return next === "retired";
    case "retired":
      return false;
    default:
      return false;
  }
}

export function canFreeze(version: FPnAPlanVersion): boolean {
  return version.status === "draft" || version.status === "review";
}

export function canPromoteToOfficial(version: FPnAPlanVersion): boolean {
  return !version.is_official && version.status !== "retired";
}
