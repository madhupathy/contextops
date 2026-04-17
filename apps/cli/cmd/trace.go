package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var traceCmd = &cobra.Command{
	Use:   "trace",
	Short: "Ingest, validate, and inspect traces",
}

var traceIngestCmd = &cobra.Command{
	Use:   "ingest <file-or-dir>",
	Short: "Ingest trace files into ContextOps",
	Args:  cobra.ExactArgs(1),
	RunE:  runTraceIngest,
}

var traceShowCmd = &cobra.Command{
	Use:   "show <run-id>",
	Short: "Show a trace by run ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runTraceShow,
}

var traceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List ingested traces",
	RunE:  runTraceList,
}

var traceValidateCmd = &cobra.Command{
	Use:   "validate <file>",
	Short: "Validate a trace file against the schema",
	Args:  cobra.ExactArgs(1),
	RunE:  runTraceValidate,
}

var traceExportCmd = &cobra.Command{
	Use:   "export <run-id>",
	Short: "Export a trace",
	Args:  cobra.ExactArgs(1),
	RunE:  runTraceExport,
}

func init() {
	rootCmd.AddCommand(traceCmd)
	traceCmd.AddCommand(traceIngestCmd)
	traceCmd.AddCommand(traceShowCmd)
	traceCmd.AddCommand(traceListCmd)
	traceCmd.AddCommand(traceValidateCmd)
	traceCmd.AddCommand(traceExportCmd)

	traceIngestCmd.Flags().Bool("recursive", false, "Recursively ingest from directory")
	traceListCmd.Flags().String("agent", "", "Filter by agent name")
	traceListCmd.Flags().String("status", "", "Filter by status")
	traceListCmd.Flags().String("since", "", "Filter by time (e.g. 7d, 24h)")
	traceExportCmd.Flags().String("format", "json", "Output format (json, yaml)")
}

func runTraceIngest(cmd *cobra.Command, args []string) error {
	path := args[0]
	recursive, _ := cmd.Flags().GetBool("recursive")

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", path, err)
	}

	var files []string
	if info.IsDir() {
		err := filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if fi.IsDir() && !recursive && p != path {
				return filepath.SkipDir
			}
			if !fi.IsDir() && (strings.HasSuffix(p, ".json") || strings.HasSuffix(p, ".jsonl")) {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("walk directory: %w", err)
		}
	} else {
		files = []string{path}
	}

	if len(files) == 0 {
		return fmt.Errorf("no JSON files found in %s", path)
	}

	fmt.Printf("Ingesting %d trace file(s)...\n", len(files))
	successCount := 0
	for _, f := range files {
		if err := ingestFile(f); err != nil {
			fmt.Fprintf(os.Stderr, "  FAIL %s: %s\n", f, err)
		} else {
			fmt.Printf("  OK   %s\n", f)
			successCount++
		}
	}
	fmt.Printf("\nIngested %d/%d traces\n", successCount, len(files))
	return nil
}

func ingestFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/traces", apiURL())
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func runTraceShow(cmd *cobra.Command, args []string) error {
	runID := args[0]
	url := fmt.Sprintf("%s/api/v1/runs/%s/timeline", apiURL(), runID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Tenant-ID", tenantID())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("run %s not found", runID)
	}

	var timeline map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&timeline); err != nil {
		return err
	}

	run, _ := timeline["run"].(map[string]interface{})

	fmt.Printf("Run: %s\n", runID)
	fmt.Printf("Status: %v\n", run["status"])
	fmt.Printf("Query: %v\n", run["query"])
	fmt.Printf("Model: %v\n", run["model"])
	fmt.Printf("Tokens: %v\n", run["total_tokens"])
	fmt.Printf("Latency: %vms\n", run["latency_ms"])
	fmt.Printf("Cost: $%v\n", run["estimated_cost"])

	if answer, ok := run["final_answer"]; ok && answer != nil {
		fmt.Printf("\nAnswer:\n  %v\n", answer)
	}

	if rc, ok := timeline["retrieval_candidates"].([]interface{}); ok {
		fmt.Printf("\nRetrieval Candidates: %d\n", len(rc))
		for i, c := range rc {
			if cm, ok := c.(map[string]interface{}); ok {
				sel := ""
				if s, ok := cm["selected"].(bool); ok && s {
					sel = " [SELECTED]"
				}
				acl := ""
				if a, ok := cm["acl_passed"].(bool); ok && !a {
					acl = " [ACL BLOCKED]"
				}
				fmt.Printf("  %d. %v (score: %v)%s%s\n", i+1, cm["title"], cm["score"], sel, acl)
			}
		}
	}

	if mc, ok := timeline["memory_candidates"].([]interface{}); ok && len(mc) > 0 {
		fmt.Printf("\nMemory Candidates: %d\n", len(mc))
		for i, m := range mc {
			if mm, ok := m.(map[string]interface{}); ok {
				stale := ""
				if s, ok := mm["is_stale"].(bool); ok && s {
					stale = " [STALE]"
				}
				fmt.Printf("  %d. [%v] %v (relevance: %v)%s\n", i+1, mm["memory_type"], mm["content"], mm["relevance_score"], stale)
			}
		}
	}

	if tc, ok := timeline["tool_calls"].([]interface{}); ok && len(tc) > 0 {
		fmt.Printf("\nTool Calls: %d\n", len(tc))
		for i, t := range tc {
			if tm, ok := t.(map[string]interface{}); ok {
				fmt.Printf("  %d. %v → %v (%vms)\n", i+1, tm["tool_name"], tm["status"], tm["latency_ms"])
			}
		}
	}

	if evals, ok := timeline["evaluations"].([]interface{}); ok && len(evals) > 0 {
		fmt.Printf("\nEvaluations: %d\n", len(evals))
		for _, e := range evals {
			if em, ok := e.(map[string]interface{}); ok {
				pass := "FAIL"
				if p, ok := em["passed"].(bool); ok && p {
					pass = "PASS"
				}
				fmt.Printf("  [%s] %v: %.2f\n", pass, em["category"], em["score"])
			}
		}
	}

	return nil
}

func runTraceList(cmd *cobra.Command, args []string) error {
	url := fmt.Sprintf("%s/api/v1/runs", apiURL())

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Tenant-ID", tenantID())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	var runs []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&runs); err != nil {
		return err
	}

	if len(runs) == 0 {
		fmt.Println("No traces found.")
		return nil
	}

	// Apply client-side filters
	agent, _ := cmd.Flags().GetString("agent")
	status, _ := cmd.Flags().GetString("status")

	fmt.Printf("%-38s %-10s %-20s %-8s %-10s\n", "RUN ID", "STATUS", "MODEL", "TOKENS", "LATENCY")
	fmt.Println(strings.Repeat("-", 90))

	for _, r := range runs {
		if agent != "" {
			// filter by agent (would need agent name lookup in a real impl)
		}
		if status != "" {
			if s, ok := r["status"].(string); ok && s != status {
				continue
			}
		}
		fmt.Printf("%-38v %-10v %-20v %-8v %-10v\n",
			r["id"], r["status"], r["model"], r["total_tokens"],
			fmt.Sprintf("%vms", r["latency_ms"]))
	}
	return nil
}

func runTraceValidate(cmd *cobra.Command, args []string) error {
	path := args[0]
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", path, err)
	}

	var trace map[string]interface{}
	if err := json.Unmarshal(data, &trace); err != nil {
		fmt.Printf("INVALID: %s is not valid JSON: %s\n", path, err)
		return nil
	}

	issues := []string{}

	// Check required fields
	required := []string{"query"}
	optionalTop := []string{"tenant_id", "run_id", "agent", "user", "input", "retrieval", "memory", "tools", "output", "metrics"}
	_ = optionalTop

	// Check if using flat or nested schema
	if _, ok := trace["input"]; ok {
		// Nested schema
		if input, ok := trace["input"].(map[string]interface{}); ok {
			if _, ok := input["query"]; !ok {
				issues = append(issues, "missing input.query")
			}
		}
	} else {
		for _, f := range required {
			if _, ok := trace[f]; !ok {
				issues = append(issues, fmt.Sprintf("missing required field: %s", f))
			}
		}
	}

	if len(issues) > 0 {
		fmt.Printf("WARNINGS in %s:\n", path)
		for _, issue := range issues {
			fmt.Printf("  - %s\n", issue)
		}
	} else {
		fmt.Printf("VALID: %s\n", path)
	}

	return nil
}

func runTraceExport(cmd *cobra.Command, args []string) error {
	runID := args[0]
	format, _ := cmd.Flags().GetString("format")

	url := fmt.Sprintf("%s/api/v1/runs/%s/timeline", apiURL(), runID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Tenant-ID", tenantID())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	switch format {
	case "json":
		var out bytes.Buffer
		json.Indent(&out, body, "", "  ")
		fmt.Println(out.String())
	default:
		fmt.Println(string(body))
	}
	return nil
}
