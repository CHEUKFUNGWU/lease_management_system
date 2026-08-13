export type LatestRequestGate = {
  begin: () => number;
  isCurrent: (requestID: number) => boolean;
  commit: (requestID: number, commit: () => void) => boolean;
};

/**
 * Small request gate used by the page itself. A response may arrive after a
 * newer query, but only the newest request can commit state.
 */
export function createLatestRequestGate(): LatestRequestGate {
  let latestRequestID = 0;
  const isCurrent = (requestID: number) => requestID === latestRequestID;
  return {
    begin: () => ++latestRequestID,
    isCurrent,
    commit: (requestID, commit) => {
      if (!isCurrent(requestID)) return false;
      commit();
      return true;
    },
  };
}
