// Command lease-agent is a thin CLI adapter for the Core Service Agent
// Gateway. It intentionally has no repository, SQL or business-rule access.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

const defaultBaseURL = "http://localhost:8080"

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(exitUsage)
	}

	var exit int
	switch os.Args[1] {
	case "auth":
		exit = runAuth(os.Args[2:])
	case "skills":
		exit = runSkills(os.Args[2:])
	case "tools":
		exit = runDescribe(os.Args[2:])
	case "execute":
		exit = runExecute(os.Args[2:])
	case "capability":
		exit = runCapability(os.Args[2:])
	case "draft-batch":
		exit = runDraftBatch(os.Args[2:])
	case "run":
		exit = runAgentRun(os.Args[2:])
	case "session":
		exit = runAgentSession(os.Args[2:])
	case "contract", "measurement", "journal", "event", "payment-schedule":
		exit = runBusiness(os.Args[1:])
	case "help", "-h", "--help":
		usage(os.Stdout)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage(os.Stderr)
		exit = exitUsage
	}
	os.Exit(exit)
}

const (
	exitOK        = 0
	exitUsage     = 2
	exitRejected  = 3
	exitFailed    = 4
	exitTransport = 5
)

func runAuth(args []string) int {
	if len(args) == 0 || args[0] != "refresh" {
		return printCLIError("auth currently supports refresh", exitUsage)
	}
	flags := flag.NewFlagSet("auth refresh", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	refreshToken := flags.String("refresh-token", os.Getenv("LEASE_AGENT_REFRESH_TOKEN"), "refresh token from a previous login/refresh response")
	if err := flags.Parse(args[1:]); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*refreshToken) == "" {
		return printCLIError("--refresh-token or LEASE_AGENT_REFRESH_TOKEN is required", exitUsage)
	}
	body, status, err := requestJSON(http.MethodPost, joinURL(*baseURL, "/api/v1/auth/refresh"), "", "", map[string]string{"refresh_token": strings.TrimSpace(*refreshToken)})
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	printJSON(body)
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	return exitOK
}

func runSkills(args []string) int {
	flags := flag.NewFlagSet("skills", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	token := flags.String("token", os.Getenv("LEASE_AGENT_TOKEN"), "JWT or delegated bearer token")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*token) == "" {
		return printCLIError("--token or LEASE_AGENT_TOKEN is required", exitUsage)
	}
	body, status, err := requestJSON(http.MethodGet, joinURL(*baseURL, "/api/v1/agent/skills"), *token, "", nil)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	printJSON(body)
	return exitOK
}

func runDescribe(args []string) int {
	flags := flag.NewFlagSet("tools", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	token := flags.String("token", os.Getenv("LEASE_AGENT_TOKEN"), "JWT or delegated bearer token")
	capabilityToken := flags.String("capability-token", os.Getenv("LEASE_AGENT_CAPABILITY_TOKEN"), "short-lived Agent capability token")
	runID := flags.String("run-id", "", "Agent run ID when using a capability token")
	skillID := flags.String("skill", "", "filter by Skill ID")
	level := flags.String("level", "", "filter by level: read, draft or command")
	includeSchema := flags.Bool("include-schema", false, "include input/output JSON Schemas")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*token) == "" {
		return printCLIError("--token or LEASE_AGENT_TOKEN is required", exitUsage)
	}

	describeURL := joinURL(*baseURL, "/api/v1/agent/tools")
	if strings.TrimSpace(*skillID) != "" {
		describeURL = addQuery(describeURL, "skill_id", strings.TrimSpace(*skillID))
	}
	if strings.TrimSpace(*level) != "" {
		describeURL = addQuery(describeURL, "level", strings.TrimSpace(*level))
	}
	if *includeSchema {
		describeURL = addQuery(describeURL, "include_schema", "true")
	}
	if strings.TrimSpace(*runID) != "" {
		describeURL = addQuery(describeURL, "run_id", strings.TrimSpace(*runID))
	}
	body, status, err := requestJSON(http.MethodGet, describeURL, *token, *capabilityToken, nil)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	printJSON(body)
	return exitOK
}

func runExecute(args []string) int {
	flags := flag.NewFlagSet("execute", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	token := flags.String("token", os.Getenv("LEASE_AGENT_TOKEN"), "JWT or delegated bearer token")
	capabilityToken := flags.String("capability-token", os.Getenv("LEASE_AGENT_CAPABILITY_TOKEN"), "short-lived Agent capability token")
	toolName := flags.String("tool", "", "registered Tool name")
	toolVersion := flags.String("version", "v1", "Tool version")
	callID := flags.String("call-id", "", "stable call ID")
	runID := flags.String("run-id", "", "Agent run ID")
	traceID := flags.String("trace-id", "", "optional trace ID")
	skillID := flags.String("skill", "", "Skill ID")
	skillVersion := flags.String("skill-version", "", "Skill version")
	idempotencyKey := flags.String("idempotency-key", "", "required for draft/command Tools")
	arguments := flags.String("arguments", "{}", "JSON object, or - to read stdin")
	dryRun := flags.Bool("dry-run", false, "request a dry-run when the Tool supports it")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*token) == "" || strings.TrimSpace(*toolName) == "" {
		return printCLIError("--token and --tool are required", exitUsage)
	}
	callIDValue := strings.TrimSpace(*callID)
	if callIDValue == "" {
		callIDValue = "cli-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	}
	runIDValue := strings.TrimSpace(*runID)
	if runIDValue == "" {
		runIDValue = callIDValue
	}
	rawArguments, err := readArguments(*arguments)
	if err != nil {
		return printCLIError(err.Error(), exitUsage)
	}
	if !isJSONObject(rawArguments) {
		return printCLIError("--arguments must be a JSON object", exitUsage)
	}

	call := agenttools.ToolCall{
		CallID: callIDValue, RunID: runIDValue, TraceID: strings.TrimSpace(*traceID),
		SkillID: strings.TrimSpace(*skillID), SkillVersion: strings.TrimSpace(*skillVersion),
		ToolName: strings.TrimSpace(*toolName), ToolVersion: strings.TrimSpace(*toolVersion),
		Arguments: rawArguments, IdempotencyKey: strings.TrimSpace(*idempotencyKey), DryRun: *dryRun,
	}
	body, status, err := requestJSON(http.MethodPost, joinURL(*baseURL, "/api/v1/agent/tools/execute"), *token, *capabilityToken, call)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	printJSON(body)
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	var result agenttools.ToolResult
	if err := json.Unmarshal(body, &result); err != nil {
		return exitFailed
	}
	switch result.Status {
	case agenttools.StatusCompleted, agenttools.StatusNeedsReview:
		return exitOK
	case agenttools.StatusRejected:
		return exitRejected
	default:
		return exitFailed
	}
}

func runCapability(args []string) int {
	if len(args) > 0 && args[0] == "revoke" {
		return runCapabilityRevoke(args[1:])
	}
	flags := flag.NewFlagSet("capability", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	token := flags.String("token", os.Getenv("LEASE_AGENT_TOKEN"), "normal JWT bearer token")
	sessionID := flags.String("session-id", "", "optional AI session ID")
	runID := flags.String("run-id", "", "Agent run ID to bind the capability to")
	skillID := flags.String("skill", "", "Skill ID to bind the capability to")
	skillVersion := flags.String("skill-version", "", "Skill version to bind the capability to")
	tools := flags.String("tools", "", "comma-separated registered Tool names")
	ttlSeconds := flags.Int("ttl-seconds", 0, "optional lifetime, capped by server configuration")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*token) == "" || strings.TrimSpace(*runID) == "" || len(splitCSV(*tools)) == 0 {
		return printCLIError("--token, --run-id and --tools are required", exitUsage)
	}
	payload := map[string]any{
		"session_id": *sessionID, "run_id": strings.TrimSpace(*runID), "skill_id": strings.TrimSpace(*skillID), "skill_version": strings.TrimSpace(*skillVersion), "allowed_tools": splitCSV(*tools),
	}
	if *ttlSeconds > 0 {
		payload["ttl_seconds"] = *ttlSeconds
	}
	body, status, err := requestJSON(http.MethodPost, joinURL(*baseURL, "/api/v1/agent/capabilities"), *token, "", payload)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	printJSON(body)
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	return exitOK
}

func runCapabilityRevoke(args []string) int {
	flags := flag.NewFlagSet("capability revoke", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	token := flags.String("token", os.Getenv("LEASE_AGENT_TOKEN"), "normal JWT bearer token")
	runID := flags.String("run-id", "", "run ID to revoke")
	tokenID := flags.String("token-id", "", "JWT ID to revoke")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*token) == "" || (strings.TrimSpace(*runID) == "" && strings.TrimSpace(*tokenID) == "") {
		return printCLIError("--token and --run-id or --token-id are required", exitUsage)
	}
	payload := map[string]string{}
	if strings.TrimSpace(*runID) != "" {
		payload["run_id"] = strings.TrimSpace(*runID)
	}
	if strings.TrimSpace(*tokenID) != "" {
		payload["token_id"] = strings.TrimSpace(*tokenID)
	}
	body, status, err := requestJSON(http.MethodPost, joinURL(*baseURL, "/api/v1/agent/capabilities/revoke"), *token, "", payload)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	printJSON(body)
	return exitOK
}

func runAgentRun(args []string) int {
	if len(args) == 0 {
		return printCLIError("run requires create, events, trace, claim, heartbeat, release, cancel, steer, follow-up or branch", exitUsage)
	}
	switch args[0] {
	case "create":
		return runAgentRunCreate(args[1:])
	case "events":
		return runAgentRunEvents(args[1:])
	case "trace":
		return runAgentRunTrace(args[1:])
	case "claim", "heartbeat", "release":
		return runAgentRunLease(args[0], args[1:])
	case "cancel", "steer", "follow-up", "branch":
		return runAgentRunControl(args[0], args[1:])
	default:
		return printCLIError("run supports create, events, trace, claim, heartbeat, release, cancel, steer, follow-up or branch", exitUsage)
	}
}

func runAgentSession(args []string) int {
	if len(args) == 0 || args[0] != "create" {
		return printCLIError("session currently supports create", exitUsage)
	}
	flags := flag.NewFlagSet("session create", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	token := flags.String("token", os.Getenv("LEASE_AGENT_TOKEN"), "JWT or delegated bearer token")
	title := flags.String("title", "", "session title")
	boundContractID := flags.String("bound-contract-id", "", "optional contract ID")
	contextSnapshot := flags.String("context-snapshot", "", "optional context snapshot JSON object")
	if err := flags.Parse(args[1:]); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*token) == "" {
		return printCLIError("--token or LEASE_AGENT_TOKEN is required", exitUsage)
	}
	payload := map[string]any{"title": strings.TrimSpace(*title), "bound_contract_id": strings.TrimSpace(*boundContractID)}
	if strings.TrimSpace(*contextSnapshot) != "" {
		var value any
		if err := json.Unmarshal([]byte(*contextSnapshot), &value); err != nil {
			return printCLIError("--context-snapshot must be valid JSON", exitUsage)
		}
		payload["context_snapshot"] = value
	}
	body, status, err := requestJSON(http.MethodPost, joinURL(*baseURL, "/api/v1/agent/sessions"), *token, "", payload)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	printJSON(body)
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	return exitOK
}

func runAgentRunCreate(args []string) int {
	flags := flag.NewFlagSet("run create", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	token := flags.String("token", os.Getenv("LEASE_AGENT_TOKEN"), "JWT or delegated bearer token")
	sessionID := flags.String("session-id", "", "AI session ID")
	message := flags.String("message", "", "initial Agent instruction")
	skillID := flags.String("skill", "", "Skill ID")
	skillVersion := flags.String("skill-version", "", "Skill version")
	pageContext := flags.String("page-context", "", "optional page context JSON object")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*token) == "" || strings.TrimSpace(*sessionID) == "" || strings.TrimSpace(*message) == "" {
		return printCLIError("--token, --session-id and --message are required", exitUsage)
	}
	payload := map[string]any{
		"session_id": strings.TrimSpace(*sessionID), "message": strings.TrimSpace(*message),
		"skill_id": strings.TrimSpace(*skillID), "skill_version": strings.TrimSpace(*skillVersion),
	}
	if strings.TrimSpace(*pageContext) != "" {
		var value any
		if err := json.Unmarshal([]byte(*pageContext), &value); err != nil {
			return printCLIError("--page-context must be valid JSON", exitUsage)
		}
		payload["page_context"] = value
	}
	body, status, err := requestJSON(http.MethodPost, joinURL(*baseURL, "/api/v1/agent/runs"), *token, "", payload)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	printJSON(body)
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	return exitOK
}

func runAgentRunEvents(args []string) int {
	flags := flag.NewFlagSet("run events", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	token := flags.String("token", os.Getenv("LEASE_AGENT_TOKEN"), "JWT or delegated bearer token")
	runID := flags.String("run-id", "", "Agent run ID")
	after := flags.Int("after", 0, "return events after this sequence")
	limit := flags.Int("limit", 200, "maximum event count")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*token) == "" || strings.TrimSpace(*runID) == "" || *after < 0 || *limit < 0 {
		return printCLIError("--token and --run-id are required; --after/--limit must be non-negative", exitUsage)
	}
	endpoint := joinURL(*baseURL, "/api/v1/agent/runs/"+url.PathEscape(strings.TrimSpace(*runID))+"/events")
	endpoint = addQuery(endpoint, "after_sequence", fmt.Sprintf("%d", *after))
	endpoint = addQuery(endpoint, "limit", fmt.Sprintf("%d", *limit))
	body, status, err := requestJSON(http.MethodGet, endpoint, *token, "", nil)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	printJSON(body)
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	return exitOK
}

func runAgentRunTrace(args []string) int {
	flags := flag.NewFlagSet("run trace", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	token := flags.String("token", os.Getenv("LEASE_AGENT_TOKEN"), "JWT or delegated bearer token")
	runID := flags.String("run-id", "", "Agent run ID")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*token) == "" || strings.TrimSpace(*runID) == "" {
		return printCLIError("--token and --run-id are required", exitUsage)
	}
	endpoint := joinURL(*baseURL, "/api/v1/agent/runs/"+url.PathEscape(strings.TrimSpace(*runID))+"/trace")
	body, status, err := requestJSON(http.MethodGet, endpoint, *token, "", nil)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	printJSON(body)
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	return exitOK
}

func runAgentRunLease(action string, args []string) int {
	flags := flag.NewFlagSet("run "+action, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	token := flags.String("token", os.Getenv("LEASE_AGENT_TOKEN"), "worker JWT bearer token")
	workerID := flags.String("worker-id", os.Getenv("AGENT_WORKER_ID"), "worker identity")
	runID := flags.String("run-id", "", "Agent run ID")
	leaseToken := flags.String("lease-token", os.Getenv("AGENT_RUN_LEASE_TOKEN"), "opaque run lease token")
	leaseSeconds := flags.Int("lease-seconds", 60, "lease duration in seconds")
	requeue := flags.Bool("requeue", false, "requeue the Run when releasing its lease")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*token) == "" || strings.TrimSpace(*workerID) == "" {
		return printCLIError("--token and --worker-id are required", exitUsage)
	}
	endpoint := joinURL(*baseURL, "/api/v1/agent/runs/claim")
	method := http.MethodPost
	var payload any = map[string]any{"worker_id": strings.TrimSpace(*workerID), "lease_seconds": *leaseSeconds}
	if action != "claim" {
		if strings.TrimSpace(*runID) == "" || strings.TrimSpace(*leaseToken) == "" {
			return printCLIError("--run-id and --lease-token are required", exitUsage)
		}
		endpoint = joinURL(*baseURL, "/api/v1/agent/runs/"+url.PathEscape(strings.TrimSpace(*runID))+"/lease/"+action)
		payload = map[string]any{"worker_id": strings.TrimSpace(*workerID), "lease_token": strings.TrimSpace(*leaseToken), "lease_seconds": *leaseSeconds, "requeue": *requeue}
	}
	body, status, err := requestJSON(method, endpoint, *token, "", payload)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	if len(body) > 0 {
		printJSON(body)
	}
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	return exitOK
}

func runAgentRunControl(action string, args []string) int {
	flags := flag.NewFlagSet("run "+action, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	token := flags.String("token", os.Getenv("LEASE_AGENT_TOKEN"), "JWT or delegated bearer token")
	runID := flags.String("run-id", "", "Agent run ID")
	instruction := flags.String("instruction", "", "steering or follow-up instruction")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*token) == "" || strings.TrimSpace(*runID) == "" {
		return printCLIError("--token and --run-id are required", exitUsage)
	}
	var payload any
	if action != "cancel" {
		if strings.TrimSpace(*instruction) == "" {
			return printCLIError("--instruction is required", exitUsage)
		}
		if action == "branch" {
			payload = map[string]string{"message": strings.TrimSpace(*instruction)}
		} else {
			payload = map[string]string{"instruction": strings.TrimSpace(*instruction)}
		}
	}
	endpoint := joinURL(*baseURL, "/api/v1/agent/runs/"+url.PathEscape(strings.TrimSpace(*runID))+"/"+action)
	body, status, err := requestJSON(http.MethodPost, endpoint, *token, "", payload)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	printJSON(body)
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	return exitOK
}

func runDraftBatch(args []string) int {
	if len(args) == 0 {
		return printCLIError("draft-batch requires get or retry", exitUsage)
	}
	flags := flag.NewFlagSet("draft-batch "+args[0], flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	baseURL := flags.String("base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	token := flags.String("token", os.Getenv("LEASE_AGENT_TOKEN"), "JWT or delegated bearer token")
	batchID := flags.String("id", "", "draft batch ID")
	artifactID := flags.String("artifact-id", "", "AI artifact ID, required for retry")
	input := flags.String("input", "-", "retry action payload JSON file, or - for stdin")
	comment := flags.String("comment", "", "retry review comment")
	if err := flags.Parse(args[1:]); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*token) == "" || strings.TrimSpace(*batchID) == "" {
		return printCLIError("--token and --id are required", exitUsage)
	}
	endpoint := joinURL(*baseURL, "/api/v1/ai/chat/draft-batches/"+url.PathEscape(strings.TrimSpace(*batchID)))
	method := http.MethodGet
	var payload any
	switch args[0] {
	case "get":
	case "retry":
		if strings.TrimSpace(*artifactID) == "" {
			return printCLIError("--artifact-id is required for retry", exitUsage)
		}
		actionPayload, err := readJSONInput(*input)
		if err != nil {
			return printCLIError(err.Error(), exitUsage)
		}
		var action map[string]any
		if err := json.Unmarshal(actionPayload, &action); err != nil {
			return printCLIError("retry input must be a JSON object", exitUsage)
		}
		payload = map[string]any{"artifact_id": strings.TrimSpace(*artifactID), "action_payload": action, "comment": *comment}
		endpoint += "/retry"
		method = http.MethodPost
	default:
		return printCLIError("draft-batch supports get or retry", exitUsage)
	}
	body, status, err := requestJSON(method, endpoint, *token, "", payload)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	printJSON(body)
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	return exitOK
}

type friendlyOptions struct {
	baseURL         string
	token           string
	capabilityToken string
	callID          string
	runID           string
	traceID         string
	skillID         string
	skillVersion    string
	idempotencyKey  string
	dryRun          bool
}

func bindFriendlyOptions(flags *flag.FlagSet) *friendlyOptions {
	options := &friendlyOptions{}
	flags.StringVar(&options.baseURL, "base-url", envOr("LEASE_AGENT_BASE_URL", defaultBaseURL), "Core Service base URL")
	flags.StringVar(&options.token, "token", os.Getenv("LEASE_AGENT_TOKEN"), "JWT or delegated bearer token")
	flags.StringVar(&options.capabilityToken, "capability-token", os.Getenv("LEASE_AGENT_CAPABILITY_TOKEN"), "short-lived Agent capability token")
	flags.StringVar(&options.callID, "call-id", "", "stable call ID")
	flags.StringVar(&options.runID, "run-id", "", "Agent run ID")
	flags.StringVar(&options.traceID, "trace-id", "", "optional trace ID")
	flags.StringVar(&options.skillID, "skill", "", "Skill ID")
	flags.StringVar(&options.skillVersion, "skill-version", "", "Skill version")
	flags.StringVar(&options.idempotencyKey, "idempotency-key", "", "required for draft commands")
	flags.BoolVar(&options.dryRun, "dry-run", false, "request a dry-run when the Tool supports it")
	return options
}

func runBusiness(args []string) int {
	if len(args) < 2 {
		return printCLIError("a business command and subcommand are required", exitUsage)
	}
	command, subcommand := args[0], args[1]
	switch command + " " + subcommand {
	case "contract search":
		return runFriendlyRead(args[2:], "lease.contract.search", func(flags *flag.FlagSet) func() (json.RawMessage, error) {
			search := flags.String("search", "", "search contract number, name, lessee, lessor or store")
			status := flags.String("status", "", "approval status")
			page := flags.Int("page", 1, "page number")
			pageSize := flags.Int("page-size", 20, "page size, maximum 100")
			sortBy := flags.String("sort-by", "", "contract_number, commencement_date, lease_end_date, approval_status or created_at")
			sortOrder := flags.String("sort-order", "", "asc or desc")
			return func() (json.RawMessage, error) {
				if *page < 1 || *pageSize < 1 || *pageSize > 100 {
					return nil, errors.New("--page must be >= 1 and --page-size must be between 1 and 100")
				}
				return json.Marshal(map[string]any{
					"search": strings.TrimSpace(*search), "status": strings.TrimSpace(*status),
					"page": *page, "page_size": *pageSize,
					"sort_by": strings.TrimSpace(*sortBy), "sort_order": strings.TrimSpace(*sortOrder),
				})
			}
		})
	case "contract get":
		return runFriendlyRead(args[2:], "lease.contract.get", func(flags *flag.FlagSet) func() (json.RawMessage, error) {
			contractID := flags.String("id", "", "contract ID")
			return func() (json.RawMessage, error) {
				if strings.TrimSpace(*contractID) == "" {
					return nil, errors.New("--id is required")
				}
				return json.RawMessage(fmt.Sprintf(`{"contract_id":%q}`, strings.TrimSpace(*contractID))), nil
			}
		})
	case "measurement list":
		return runFriendlyRead(args[2:], "lease.measurement.list", func(flags *flag.FlagSet) func() (json.RawMessage, error) {
			contractID := flags.String("contract-id", "", "contract ID")
			period := flags.String("period", "", "optional accounting period")
			return func() (json.RawMessage, error) {
				if strings.TrimSpace(*contractID) == "" {
					return nil, errors.New("--contract-id is required")
				}
				return marshalArguments(map[string]string{"contract_id": strings.TrimSpace(*contractID), "period": strings.TrimSpace(*period)})
			}
		})
	case "event list":
		return runFriendlyRead(args[2:], "lease.event.list", func(flags *flag.FlagSet) func() (json.RawMessage, error) {
			contractID := flags.String("contract-id", "", "contract ID")
			return func() (json.RawMessage, error) {
				if strings.TrimSpace(*contractID) == "" {
					return nil, errors.New("--contract-id is required")
				}
				return marshalArguments(map[string]string{"contract_id": strings.TrimSpace(*contractID)})
			}
		})
	case "journal list":
		return runFriendlyRead(args[2:], "lease.journal.list", func(flags *flag.FlagSet) func() (json.RawMessage, error) {
			contractID := flags.String("contract-id", "", "contract ID")
			period := flags.String("period", "", "optional accounting period")
			status := flags.String("status", "", "optional posting status")
			return func() (json.RawMessage, error) {
				if strings.TrimSpace(*contractID) == "" {
					return nil, errors.New("--contract-id is required")
				}
				return marshalArguments(map[string]string{"contract_id": strings.TrimSpace(*contractID), "period": strings.TrimSpace(*period), "status": strings.TrimSpace(*status)})
			}
		})
	case "contract draft-create":
		return runFriendlyDraft(args[2:], "lease.contract.draft.create")
	case "payment-schedule draft-create":
		return runFriendlyDraft(args[2:], "lease.payment_schedule.draft.create")
	default:
		return printCLIError(fmt.Sprintf("unknown business command %q", command+" "+subcommand), exitUsage)
	}
}

type friendlyArgumentBuilder func(*flag.FlagSet) func() (json.RawMessage, error)

func runFriendlyRead(args []string, toolName string, build friendlyArgumentBuilder) int {
	flags := flag.NewFlagSet(toolName, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	options := bindFriendlyOptions(flags)
	finalize := build(flags)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	arguments, err := finalize()
	if err != nil {
		return printCLIError(err.Error(), exitUsage)
	}
	return executeFriendly(options, toolName, arguments, false)
}

func runFriendlyDraft(args []string, toolName string) int {
	flags := flag.NewFlagSet(toolName, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	options := bindFriendlyOptions(flags)
	input := flags.String("input", "-", "JSON Tool arguments file, or - for stdin")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(options.idempotencyKey) == "" {
		return printCLIError("--idempotency-key is required for draft commands", exitUsage)
	}
	arguments, err := readJSONInput(*input)
	if err != nil {
		return printCLIError(err.Error(), exitUsage)
	}
	return executeFriendly(options, toolName, arguments, true)
}

func marshalArguments(value map[string]string) (json.RawMessage, error) {
	filtered := make(map[string]string, len(value))
	for key, item := range value {
		if strings.TrimSpace(item) != "" {
			filtered[key] = item
		}
	}
	data, err := json.Marshal(filtered)
	return data, err
}

func executeFriendly(options *friendlyOptions, toolName string, arguments json.RawMessage, requireIdempotency bool) int {
	if options == nil || strings.TrimSpace(options.token) == "" {
		return printCLIError("--token or LEASE_AGENT_TOKEN is required", exitUsage)
	}
	if requireIdempotency && strings.TrimSpace(options.idempotencyKey) == "" {
		return printCLIError("--idempotency-key is required for draft commands", exitUsage)
	}
	callID := strings.TrimSpace(options.callID)
	if callID == "" {
		callID = "cli-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	}
	runID := strings.TrimSpace(options.runID)
	if runID == "" {
		runID = callID
	}
	call := agenttools.ToolCall{
		CallID: callID, RunID: runID, TraceID: strings.TrimSpace(options.traceID), ToolName: toolName,
		SkillID: strings.TrimSpace(options.skillID), SkillVersion: strings.TrimSpace(options.skillVersion),
		ToolVersion: "v1", Arguments: arguments, IdempotencyKey: strings.TrimSpace(options.idempotencyKey), DryRun: options.dryRun,
	}
	body, status, err := requestJSON(http.MethodPost, joinURL(options.baseURL, "/api/v1/agent/tools/execute"), options.token, options.capabilityToken, call)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	printJSON(body)
	if status < 200 || status >= 300 {
		return printHTTPError(body, status)
	}
	var result agenttools.ToolResult
	if err := json.Unmarshal(body, &result); err != nil {
		return exitFailed
	}
	switch result.Status {
	case agenttools.StatusCompleted, agenttools.StatusNeedsReview:
		return exitOK
	case agenttools.StatusRejected:
		return exitRejected
	default:
		return exitFailed
	}
}

func readJSONInput(path string) (json.RawMessage, error) {
	var reader io.Reader = os.Stdin
	var file *os.File
	if strings.TrimSpace(path) != "" && path != "-" {
		opened, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		file = opened
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(io.LimitReader(reader, 8<<20))
	if err != nil {
		return nil, err
	}
	if !isJSONObject(data) {
		return nil, errors.New("input must be a JSON object")
	}
	return json.RawMessage(data), nil
}

func requestJSON(method, url, token, capabilityToken string, payload any) ([]byte, int, error) {
	request, err := newJSONRequest(method, url, token, capabilityToken, payload)
	if err != nil {
		return nil, 0, err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, response.StatusCode, err
	}
	return responseBody, response.StatusCode, nil
}

func newJSONRequest(method, url, token, capabilityToken string, payload any) (*http.Request, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if strings.TrimSpace(token) != "" {
		request.Header.Set("Authorization", bearerToken(token))
	}
	if strings.TrimSpace(capabilityToken) != "" {
		request.Header.Set("X-Agent-Capability", strings.TrimSpace(capabilityToken))
	}
	return request, nil
}

func readArguments(value string) (json.RawMessage, error) {
	if value == "-" {
		data, err := io.ReadAll(io.LimitReader(os.Stdin, 8<<20))
		if err != nil {
			return nil, err
		}
		return json.RawMessage(data), nil
	}
	return json.RawMessage(value), nil
}

func isJSONObject(raw []byte) bool {
	var value map[string]any
	return json.Unmarshal(raw, &value) == nil && value != nil
}

func bearerToken(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}
	return "Bearer " + token
}

func joinURL(baseURL, path string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + path
}

func addQuery(rawURL, key, value string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func printJSON(body []byte) {
	var value any
	if json.Unmarshal(body, &value) == nil {
		pretty, err := json.MarshalIndent(value, "", "  ")
		if err == nil {
			fmt.Println(string(pretty))
			return
		}
	}
	fmt.Println(string(body))
}

func printHTTPError(body []byte, status int) int {
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(status)
	}
	fmt.Fprintf(os.Stderr, "Gateway returned HTTP %d: %s\n", status, message)
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return exitRejected
	}
	return exitFailed
}

func printCLIError(message string, code int) int {
	if strings.TrimSpace(message) == "" {
		message = errors.New("unknown CLI error").Error()
	}
	fmt.Fprintln(os.Stderr, message)
	return code
}

func usage(writer io.Writer) {
	fmt.Fprintln(writer, `lease-agent: CLI adapter for the lease-management-system Agent Gateway

Usage:
  lease-agent auth refresh --refresh-token TOKEN [flags]
  lease-agent skills [flags]
  lease-agent tools [flags]
  lease-agent execute --tool NAME [flags]
  lease-agent capability --run-id RUN --tools NAME[,NAME...] [flags]
  lease-agent capability revoke --run-id RUN [flags]
  lease-agent session create [flags]
  lease-agent run create --session-id SESSION --message MESSAGE [flags]
  lease-agent run events --run-id RUN [flags]
  lease-agent run trace --run-id RUN [flags]
  lease-agent run claim --worker-id WORKER [flags]
  lease-agent run heartbeat --run-id RUN --worker-id WORKER --lease-token TOKEN [flags]
  lease-agent run release --run-id RUN --worker-id WORKER --lease-token TOKEN [flags]
  lease-agent run cancel --run-id RUN [flags]
  lease-agent run steer --run-id RUN --instruction TEXT [flags]
  lease-agent run follow-up --run-id RUN --instruction TEXT [flags]
  lease-agent draft-batch get --id BATCH_ID [flags]
  lease-agent draft-batch retry --id BATCH_ID --artifact-id ARTIFACT_ID --input selection.json [flags]
  lease-agent contract get --id CONTRACT_ID [flags]
  lease-agent measurement list --contract-id CONTRACT_ID [flags]
  lease-agent journal list --contract-id CONTRACT_ID [flags]
  lease-agent event list --contract-id CONTRACT_ID [flags]
  lease-agent contract draft-create --input contract-draft.json [flags]
  lease-agent payment-schedule draft-create --input payment-draft.json [flags]

Examples:
  lease-agent tools --token "$LEASE_AGENT_TOKEN" --level read
  lease-agent execute --token "$LEASE_AGENT_TOKEN" --tool lease.contract.get \
    --run-id run-1 --arguments '{"contract_id":"..."}'
  lease-agent capability --token "$LEASE_AGENT_TOKEN" --run-id run-1 \
    --tools lease.contract.get,lease.measurement.list
  lease-agent contract get --token "$LEASE_AGENT_TOKEN" --id CONTRACT_ID
  cat draft.json | lease-agent execute --token "$LEASE_AGENT_TOKEN" \
    --tool lease.contract.draft.create --idempotency-key draft-1 --arguments -

The CLI sends only ToolCall/ToolFilter/capability request payloads. User
identity, permissions, data scope and approval policy are resolved by Core
Service. A capability token is sent in X-Agent-Capability, never in ToolCall.`)
}
