package llm

// RT1-L3-B: provider health signal WITHOUT active probing. The spec's hard
// constraint: never dial the provider just to fill a health dashboard — that
// is a paid external call. Instead every REAL chat round records its outcome
// into this cache.
//
// Coverage: ALL provider I/O in this repository funnels through Client.chat
// (client.go holds the only httpc.Do), and Client.StreamFunc delegates to
// Chat — so both direct callers and the agentcore interactive loop are
// recorded. TestStreamFuncRecordsCall pins this; if another provider exit
// point ever appears without recording, the health surface lies again.

import (
	"sync/atomic"
	"time"
)

var (
	lastSuccessAt atomic.Int64 // unix nanos; 0 = never succeeded since boot
	lastFailureAt atomic.Int64 // unix nanos; 0 = never failed since boot
)

func recordCallSuccess() { lastSuccessAt.Store(time.Now().UnixNano()) }

func recordCallFailure() { lastFailureAt.Store(time.Now().UnixNano()) }

// CallSignal is the cached outcome of real provider calls. Fields are true
// zero time.Time when no such call happened since process start — reported
// as unknown by consumers, never inferred as ok.
type CallSignal struct {
	LastSuccess time.Time
	LastFailure time.Time
}

// LastCallSignal returns the cached outcomes. A nanosecond value of 0 means
// "no call since boot"; it converts to the true zero time.Time here so
// consumers can use IsZero safely (time.Unix(0,0) would be 1970-01-01 and
// IsZero-blind — the exact bug RT1-L3-B review caught).
func LastCallSignal() CallSignal {
	return CallSignal{
		LastSuccess: nanosToTime(lastSuccessAt.Load()),
		LastFailure: nanosToTime(lastFailureAt.Load()),
	}
}

func nanosToTime(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// ResetCallSignal clears the cached outcomes. Test seam — production code has
// no reason to call it (the cache is monotonic by design).
func ResetCallSignal() {
	lastSuccessAt.Store(0)
	lastFailureAt.Store(0)
}
