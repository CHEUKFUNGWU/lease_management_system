package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// runRetail dispatches the layer-1 shortcut commands of the retail fill seam
// (appendix A.4): import preview/commit and scenario evaluate/save. Layer-3
// raw access remains `lease-agent execute`; these commands add workflow value
// on top of the same APIs.
func runRetail(args []string) int {
	if len(args) < 2 {
		return printCLIError("a retail subcommand is required: import preview|commit, scenario evaluate|save", exitUsage)
	}
	switch args[0] + " " + args[1] {
	case "import preview":
		return runRetailImport(args[2:], false)
	case "import commit":
		return runRetailImport(args[2:], true)
	case "scenario evaluate":
		return runScenarioEvaluate(args[2:])
	case "scenario save":
		return runScenarioSave(args[2:])
	default:
		return printCLIError(fmt.Sprintf("unknown retail command %q", args[0]+" "+args[1]), exitUsage)
	}
}

// runRetailImport drives the store-day import seam: multipart upload to the
// preview or commit endpoint. Commit is the human path — agent capability
// tokens are refused here, dry-run is impossible, and --confirm is required
// (appendix A.4: commit belongs to humans).
func runRetailImport(args []string, commit bool) int {
	flags := flag.NewFlagSet("retail import", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	options := bindFriendlyOptions(flags)
	sourceSystem := flags.String("source-system", "", "source system identifier (required)")
	asOf := flags.String("as-of-at", "", "as-of timestamp, RFC3339 (required for commit)")
	confirm := flags.Bool("confirm", false, "required for commit: acknowledge that this writes business data")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	if commit {
		if strings.TrimSpace(options.capabilityToken) != "" {
			return printCLIError("commit commands require a human account token (--token), not an agent capability token", exitRejected)
		}
		if !*confirm {
			return printCLIError("commit writes business data: review the preview, then re-run with --confirm", exitUsage)
		}
		if strings.TrimSpace(*asOf) == "" {
			return printCLIError("--as-of-at is required for commit", exitUsage)
		}
		if strings.TrimSpace(options.idempotencyKey) == "" {
			return printCLIError("--idempotency-key is required for commit: replays must not duplicate records", exitUsage)
		}
	}
	if strings.TrimSpace(*sourceSystem) == "" {
		return printCLIError("--source-system is required", exitUsage)
	}
	if strings.TrimSpace(options.token) == "" {
		return printCLIError("--token or LEASE_AGENT_TOKEN is required", exitUsage)
	}
	fileArg := flags.Arg(0)
	if fileArg == "" {
		return printCLIError("a file path is required", exitUsage)
	}

	body, contentType, err := multipartFileBody(fileArg, map[string]string{
		"source_system": strings.TrimSpace(*sourceSystem),
		"as_of_at":      strings.TrimSpace(*asOf),
	})
	if err != nil {
		return printCLIError(err.Error(), exitUsage)
	}

	endpoint := "/retail/operating-facts/store-days/import/preview"
	if commit {
		endpoint = "/retail/operating-facts/store-days/import/commit"
	}
	headers := map[string]string{"Content-Type": contentType}
	if commit {
		headers["Idempotency-Key"] = strings.TrimSpace(options.idempotencyKey)
	}
	resp, status, err := requestMultipart(http.MethodPost, joinURL(options.baseURL, "/api/v1"+endpoint), options.token, body, headers)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	printJSON(resp)
	if status < 200 || status >= 300 {
		return printHTTPError(resp, status)
	}
	if commit {
		fmt.Fprintln(os.Stderr, "committed — records are now production data with the given idempotency key")
	}
	return exitOK
}

// runScenarioEvaluate wraps the deterministic scenario tool. Assumptions come
// from an optional JSON file; the server rebuilds Baseline from store facts.
func runScenarioEvaluate(args []string) int {
	flags := flag.NewFlagSet("scenario evaluate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	options := bindFriendlyOptions(flags)
	storeID := flags.String("store-id", "", "store ID (required)")
	horizon := flags.Int("horizon", 12, "horizon in months")
	input := flags.String("input", "", "optional JSON file with assumptions")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(*storeID) == "" {
		return printCLIError("--store-id is required", exitUsage)
	}
	assumptions := json.RawMessage(`{}`)
	if strings.TrimSpace(*input) != "" {
		raw, err := readJSONInput(*input)
		if err != nil {
			return printCLIError(err.Error(), exitUsage)
		}
		assumptions = raw
	}
	arguments, err := json.Marshal(map[string]any{
		"store_id":       strings.TrimSpace(*storeID),
		"horizon_months": *horizon,
		"assumptions":    json.RawMessage(assumptions),
	})
	if err != nil {
		return printCLIError(err.Error(), exitFailed)
	}
	return executeFriendly(options, "retail.store.scenario.evaluate", arguments, false)
}

// runScenarioSave posts a scenario action draft. It is a write seam: human
// token only, idempotency key and --confirm required.
func runScenarioSave(args []string) int {
	flags := flag.NewFlagSet("scenario save", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	options := bindFriendlyOptions(flags)
	storeID := flags.String("store-id", "", "store ID (required)")
	input := flags.String("input", "", "JSON file with the scenario action draft (required)")
	confirm := flags.Bool("confirm", false, "required: acknowledge that this writes a draft")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if strings.TrimSpace(options.capabilityToken) != "" {
		return printCLIError("save commands require a human account token (--token), not an agent capability token", exitRejected)
	}
	if !*confirm {
		return printCLIError("save writes a draft: re-run with --confirm", exitUsage)
	}
	if strings.TrimSpace(*storeID) == "" {
		return printCLIError("--store-id is required", exitUsage)
	}
	if strings.TrimSpace(options.idempotencyKey) == "" {
		return printCLIError("--idempotency-key is required for save", exitUsage)
	}
	payload, err := readJSONInput(*input)
	if err != nil {
		return printCLIError(err.Error(), exitUsage)
	}
	headers := map[string]string{"Idempotency-Key": strings.TrimSpace(options.idempotencyKey)}
	resp, status, err := requestJSONWithHeaders(http.MethodPost, joinURL(options.baseURL, "/api/v1/retail/stores/"+strings.TrimSpace(*storeID)+"/scenario-action-drafts"), options.token, "", payload, headers)
	if err != nil {
		return printCLIError(err.Error(), exitTransport)
	}
	printJSON(resp)
	if status < 200 || status >= 300 {
		return printHTTPError(resp, status)
	}
	return exitOK
}

// multipartFileBody builds a multipart body with one file part and optional
// form fields.
func multipartFileBody(path string, fields map[string]string) ([]byte, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for key, value := range fields {
		if strings.TrimSpace(value) != "" {
			if err := mw.WriteField(key, value); err != nil {
				return nil, "", err
			}
		}
	}
	part, err := mw.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, "", err
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), mw.FormDataContentType(), nil
}

func requestMultipart(method, url, token string, body []byte, headers map[string]string) ([]byte, int, error) {
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	request.Header.Set("Accept", "application/json")
	if strings.TrimSpace(token) != "" {
		request.Header.Set("Authorization", bearerToken(token))
	}
	client := &http.Client{Timeout: 120 * time.Second}
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

func requestJSONWithHeaders(method, url, token, capabilityToken string, payload any, headers map[string]string) ([]byte, int, error) {
	request, err := newJSONRequest(method, url, token, capabilityToken, payload)
	if err != nil {
		return nil, 0, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
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
