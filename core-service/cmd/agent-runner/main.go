// Command agent-runner is the process boundary for a Pi-like worker. The
// worker has no database, MinIO or business repository access: it receives a
// server-approved Tool plan, calls Core through HTTPGateway, and persists
// events/checkpoints back to the owned Core Run.
//
// The JSON planner is intentionally deterministic for CI and air-gapped
// deployments. A production Pi/LLM planner can replace it without changing
// the Runner, Gateway, capability or checkpoint contracts.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agentrunner"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

const defaultGatewayURL = "http://localhost:8080"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("agent-runner", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("AGENT_GATEWAY_URL", defaultGatewayURL), "Core Agent Gateway URL")
	token := flags.String("token", os.Getenv("AGENT_GATEWAY_TOKEN"), "JWT bearer token")
	sessionID := flags.String("session-id", "", "existing Agent session ID; if empty, create one")
	runID := flags.String("run-id", "", "existing Agent run ID; if empty, create one")
	message := flags.String("message", "", "initial Agent instruction")
	title := flags.String("title", "Pi Agent Runner", "new session title")
	skillID := flags.String("skill", "", "Skill ID")
	skillVersion := flags.String("skill-version", "v1", "Skill version")
	planPath := flags.String("plan", "-", "JSON array of ToolCall objects, or - for stdin")
	plannerURL := flags.String("planner-url", os.Getenv("AGENT_PLANNER_URL"), "AI Service Agent Planner URL; when set, --plan is not required")
	plannerToken := flags.String("planner-token", os.Getenv("AGENT_PLANNER_TOKEN"), "optional bearer token for the AI Service planner")
	workerID := flags.String("worker-id", os.Getenv("AGENT_WORKER_ID"), "worker identity; claims a queued Run when --run-id is empty")
	workerLoop := flags.Bool("worker-loop", false, "keep claiming queued Runs instead of exiting after one Run")
	pollInterval := flags.Duration("poll-interval", 2*time.Second, "worker queue polling interval")
	maxRuns := flags.Int("max-runs", 0, "worker loop run limit; 0 means continue until interrupted")
	leaseToken := flags.String("lease-token", os.Getenv("AGENT_RUN_LEASE_TOKEN"), "existing Core Run lease token")
	leaseSeconds := flags.Int("lease-seconds", 60, "worker lease duration in seconds")
	resume := flags.Bool("resume", false, "resume the Core checkpoint for the run")
	maxToolCalls := flags.Int("max-tool-calls", 12, "maximum Tool calls")
	maxRetries := flags.Int("max-retries", 1, "maximum retry count")
	maxResultBytes := flags.Int("max-result-bytes", 2<<20, "maximum encoded Tool result bytes")
	maxModelTokens := flags.Int64("max-model-tokens", 12000, "maximum cumulative model tokens; 0 disables the limit")
	deadline := flags.Duration("deadline", 10*time.Minute, "run deadline")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*token) == "" {
		return errors.New("--token or AGENT_GATEWAY_TOKEN is required")
	}
	if *workerLoop {
		if strings.TrimSpace(*workerID) == "" {
			return errors.New("--worker-id or AGENT_WORKER_ID is required in --worker-loop mode")
		}
		if strings.TrimSpace(*runID) != "" || strings.TrimSpace(*sessionID) != "" || strings.TrimSpace(*message) != "" {
			return errors.New("--worker-loop cannot be combined with --run-id, --session-id or --message")
		}
		if strings.TrimSpace(*plannerURL) == "" && (strings.TrimSpace(*planPath) == "" || strings.TrimSpace(*planPath) == "-") {
			return errors.New("--plan must point to a reusable JSON plan file in --worker-loop mode")
		}
		if strings.TrimSpace(*plannerURL) != "" && strings.TrimSpace(*planPath) != "" && strings.TrimSpace(*planPath) != "-" {
			return errors.New("--planner-url and --plan cannot be combined")
		}
		if *maxRuns < 0 || *pollInterval <= 0 {
			return errors.New("--max-runs must be non-negative and --poll-interval must be positive")
		}
		var plan []agenttools.ToolCall
		if strings.TrimSpace(*plannerURL) == "" {
			loadedPlan, err := readPlan(*planPath)
			if err != nil {
				return err
			}
			plan = loadedPlan
		}
		gateway := agentrunner.NewHTTPGateway(*baseURL, *token, &http.Client{Timeout: 30 * time.Second})
		planner, err := newPlanner(plan, *plannerURL, *plannerToken)
		if err != nil {
			return err
		}
		return runWorkerLoop(context.Background(), gateway, planner, *workerID, *leaseSeconds, *pollInterval, *maxRuns, runnerLimits(*maxToolCalls, *maxRetries, *maxResultBytes, *maxModelTokens, *deadline))
	}
	if strings.TrimSpace(*message) == "" && strings.TrimSpace(*workerID) == "" {
		return errors.New("--message is required")
	}
	var plan []agenttools.ToolCall
	if strings.TrimSpace(*plannerURL) == "" {
		loadedPlan, err := readPlan(*planPath)
		if err != nil {
			return err
		}
		plan = loadedPlan
	}
	gateway := agentrunner.NewHTTPGateway(*baseURL, *token, &http.Client{Timeout: 30 * time.Second})
	ctx := context.Background()
	if strings.TrimSpace(*sessionID) == "" && strings.TrimSpace(*workerID) == "" {
		session, err := gateway.CreateSession(ctx, agentrunner.SessionRequest{Title: *title})
		if err != nil {
			return fmt.Errorf("create agent session: %w", err)
		}
		*sessionID = session.ID
	}
	if strings.TrimSpace(*runID) == "" {
		if strings.TrimSpace(*workerID) != "" {
			lease, err := gateway.ClaimRun(ctx, *workerID, *leaseSeconds)
			if err != nil {
				return fmt.Errorf("claim agent run: %w", err)
			}
			*runID = lease.Run.ID
			*sessionID = lease.Run.SessionID
			*leaseToken = lease.LeaseToken
			gateway = gateway.WithWorkerLease(*workerID, *leaseToken)
			if strings.TrimSpace(*skillID) == "" {
				*skillID = lease.Run.SkillID
			}
			if strings.TrimSpace(*skillVersion) == "v1" && lease.Run.SkillVersion != "" {
				*skillVersion = lease.Run.SkillVersion
			}
			if strings.TrimSpace(*message) == "" {
				instruction, err := gateway.LoadRunInstruction(ctx, *runID)
				if err != nil {
					return fmt.Errorf("load claimed run instruction: %w", err)
				}
				*message = instruction
			}
		} else {
			run, err := gateway.CreateRun(ctx, agentrunner.RunRequest{
				SessionID: *sessionID, Message: *message, SkillID: *skillID, SkillVersion: *skillVersion,
			})
			if err != nil {
				return fmt.Errorf("create agent run: %w", err)
			}
			*runID = run.ID
		}
	}
	if strings.TrimSpace(*workerID) != "" && strings.TrimSpace(*leaseToken) != "" && gateway.WorkerID == "" {
		gateway = gateway.WithWorkerLease(*workerID, *leaseToken)
	}
	planner, err := newPlanner(plan, *plannerURL, *plannerToken)
	if err != nil {
		return err
	}
	planner = bindPlannerUsage(planner, gateway)
	runner := &agentrunner.Runner{
		Gateway: gateway, Planner: planner, EventRecorder: gateway,
		Checkpoints: gateway,
	}
	result, err := runner.Run(ctx, agentrunner.Request{
		RunID: *runID, SessionID: *sessionID, Message: *message,
		SkillID: *skillID, SkillVersion: *skillVersion, Resume: *resume,
		WorkerID: *workerID, LeaseToken: *leaseToken, LeaseSeconds: *leaseSeconds,
		Limits: runnerLimits(*maxToolCalls, *maxRetries, *maxResultBytes, *maxModelTokens, *deadline),
	})
	encoded, marshalErr := json.Marshal(result)
	if marshalErr == nil {
		fmt.Println(string(encoded))
	}
	if err != nil {
		return err
	}
	return nil
}

func runWorkerLoop(ctx context.Context, gateway *agentrunner.HTTPGateway, planner agentrunner.Planner, workerID string, leaseSeconds int, pollInterval time.Duration, maxRuns int, limits agentrunner.Limits) error {
	processed := 0
	var lastErr error
	for maxRuns == 0 || processed < maxRuns {
		lease, err := gateway.ClaimRun(ctx, workerID, leaseSeconds)
		if err == agentrunner.ErrNoQueuedRun {
			timer := time.NewTimer(pollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("claim agent run: %w", err)
		}
		workerGateway := gateway.WithWorkerLease(workerID, lease.LeaseToken)
		instruction, err := workerGateway.LoadRunInstruction(ctx, lease.Run.ID)
		if err != nil {
			_ = gateway.ReleaseRunLease(context.WithoutCancel(ctx), lease.Run.ID, workerID, lease.LeaseToken, true)
			return fmt.Errorf("load claimed run instruction: %w", err)
		}
		skillID, skillVersion := lease.Run.SkillID, lease.Run.SkillVersion
		runPlanner := bindPlannerUsage(planner, workerGateway)
		runner := &agentrunner.Runner{Gateway: workerGateway, Planner: runPlanner, EventRecorder: workerGateway, Checkpoints: workerGateway}
		result, runErr := runner.Run(ctx, agentrunner.Request{
			RunID: lease.Run.ID, SessionID: lease.Run.SessionID, Message: instruction,
			SkillID: skillID, SkillVersion: skillVersion, WorkerID: workerID,
			LeaseToken: lease.LeaseToken, LeaseSeconds: lease.LeaseSeconds, Limits: limits,
		})
		encoded, marshalErr := json.Marshal(result)
		if marshalErr == nil {
			fmt.Println(string(encoded))
		}
		processed++
		if runErr != nil {
			lastErr = runErr
			fmt.Fprintf(os.Stderr, "agent worker run %s failed: %v\n", lease.Run.ID, runErr)
		}
	}
	return lastErr
}

func bindPlannerUsage(planner agentrunner.Planner, gateway *agentrunner.HTTPGateway) agentrunner.Planner {
	httpPlanner, ok := planner.(*agentrunner.HTTPPlanner)
	if !ok || httpPlanner == nil || gateway == nil {
		return planner
	}
	return httpPlanner.WithUsageRecorder(func(ctx context.Context, runID string, usage agentrunner.PlannerUsage) error {
		return gateway.AppendRunEvent(ctx, runID, agentrunner.Event{
			Type:    "planner_usage",
			RunID:   runID,
			Payload: usage,
		})
	})
}

func newPlanner(plan []agenttools.ToolCall, plannerURL, plannerToken string) (agentrunner.Planner, error) {
	if strings.TrimSpace(plannerURL) != "" {
		return agentrunner.NewHTTPPlanner(plannerURL, plannerToken, &http.Client{Timeout: 120 * time.Second}), nil
	}
	if len(plan) == 0 {
		return nil, errors.New("a Tool plan or --planner-url is required")
	}
	return agentrunner.PlannerFunc(func(_ context.Context, request agentrunner.PlanRequest) ([]agenttools.ToolCall, error) {
		calls := make([]agenttools.ToolCall, len(plan))
		copy(calls, plan)
		return calls, nil
	}), nil
}

func runnerLimits(maxToolCalls, maxRetries, maxResultBytes int, maxModelTokens int64, deadline time.Duration) agentrunner.Limits {
	return agentrunner.Limits{MaxToolCalls: maxToolCalls, MaxRetries: maxRetries, MaxResultBytes: maxResultBytes, MaxModelTokens: maxModelTokens, Deadline: deadline}
}

func readPlan(path string) ([]agenttools.ToolCall, error) {
	var reader io.Reader = os.Stdin
	var file *os.File
	if strings.TrimSpace(path) != "" && path != "-" {
		var err error
		file, err = os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open plan: %w", err)
		}
		defer file.Close()
		reader = file
	}
	var plan []agenttools.ToolCall
	if err := json.NewDecoder(reader).Decode(&plan); err != nil {
		return nil, fmt.Errorf("decode plan JSON: %w", err)
	}
	if len(plan) == 0 {
		return nil, errors.New("plan must contain at least one ToolCall")
	}
	return plan, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
