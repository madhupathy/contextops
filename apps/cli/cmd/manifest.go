package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

var manifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Inspect context manifests",
}

var manifestShowCmd = &cobra.Command{
	Use:   "show <run-id>",
	Short: "Show the context manifest for a run",
	Args:  cobra.ExactArgs(1),
	RunE:  runManifestShow,
}

var manifestDiffCmd = &cobra.Command{
	Use:   "diff <run-a> <run-b>",
	Short: "Diff context manifests between two runs",
	Args:  cobra.ExactArgs(2),
	RunE:  runManifestDiff,
}

func init() {
	rootCmd.AddCommand(manifestCmd)
	manifestCmd.AddCommand(manifestShowCmd)
	manifestCmd.AddCommand(manifestDiffCmd)

	manifestShowCmd.Flags().String("format", "text", "Output format (text, json)")
}

func runManifestShow(cmd *cobra.Command, args []string) error {
	runID := args[0]
	format, _ := cmd.Flags().GetString("format")

	url := fmt.Sprintf("%s/api/v1/runs/%s/context-manifest", apiURL(), runID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Tenant-ID", tenantID())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	var manifest map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}

	if format == "json" {
		out, _ := json.MarshalIndent(manifest, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("Context Manifest: %s\n", runID)
	fmt.Println(strings.Repeat("=", 60))

	if cm, ok := manifest["context_manifest"].(map[string]interface{}); ok {
		fmt.Printf("\n  Total Tokens:  %v\n", cm["total_tokens"])
		fmt.Printf("  Doc Tokens:    %v\n", cm["doc_tokens"])
		fmt.Printf("  Memory Tokens: %v\n", cm["memory_tokens"])
		fmt.Printf("  System Tokens: %v\n", cm["system_tokens"])
	}

	if rc, ok := manifest["retrieval_candidates"].([]interface{}); ok {
		selected := 0
		blocked := 0
		rejected := 0
		for _, c := range rc {
			if cm, ok := c.(map[string]interface{}); ok {
				if s, ok := cm["selected"].(bool); ok && s {
					selected++
				} else if a, ok := cm["acl_passed"].(bool); ok && !a {
					blocked++
				} else {
					rejected++
				}
			}
		}
		fmt.Printf("\n  Sources Considered: %d\n", len(rc))
		fmt.Printf("  Sources Selected:  %d\n", selected)
		fmt.Printf("  Sources Rejected:  %d\n", rejected)
		fmt.Printf("  ACL Blocked:       %d\n", blocked)
	}

	if mc, ok := manifest["memory_candidates"].([]interface{}); ok && len(mc) > 0 {
		selected := 0
		stale := 0
		for _, m := range mc {
			if mm, ok := m.(map[string]interface{}); ok {
				if s, ok := mm["selected"].(bool); ok && s {
					selected++
				}
				if s, ok := mm["is_stale"].(bool); ok && s {
					stale++
				}
			}
		}
		fmt.Printf("\n  Memory Considered: %d\n", len(mc))
		fmt.Printf("  Memory Injected:   %d\n", selected)
		fmt.Printf("  Stale Memory:      %d\n", stale)
	}

	fmt.Println()
	return nil
}

func runManifestDiff(cmd *cobra.Command, args []string) error {
	runA := args[0]
	runB := args[1]

	timelineA, err := fetchTimeline(runA)
	if err != nil {
		return fmt.Errorf("fetch run %s: %w", runA, err)
	}
	timelineB, err := fetchTimeline(runB)
	if err != nil {
		return fmt.Errorf("fetch run %s: %w", runB, err)
	}

	runDataA, _ := timelineA["run"].(map[string]interface{})
	runDataB, _ := timelineB["run"].(map[string]interface{})

	cmA, _ := runDataA["context_manifest"].(map[string]interface{})
	cmB, _ := runDataB["context_manifest"].(map[string]interface{})

	fmt.Printf("Context Manifest Diff: %s vs %s\n", runA, runB)
	fmt.Println(strings.Repeat("=", 60))

	fmt.Printf("\n%-20s %-15s %-15s\n", "FIELD", runA[:8]+"...", runB[:8]+"...")
	fmt.Println(strings.Repeat("-", 50))

	fields := []string{"total_tokens", "doc_tokens", "memory_tokens", "system_tokens"}
	for _, f := range fields {
		valA := cmA[f]
		valB := cmB[f]
		marker := " "
		if fmt.Sprintf("%v", valA) != fmt.Sprintf("%v", valB) {
			marker = "*"
		}
		fmt.Printf("%s %-18s %-15v %-15v\n", marker, f, valA, valB)
	}

	rcA, _ := timelineA["retrieval_candidates"].([]interface{})
	rcB, _ := timelineB["retrieval_candidates"].([]interface{})
	fmt.Printf("\n  Retrieval: %d candidates → %d candidates\n", len(rcA), len(rcB))

	mcA, _ := timelineA["memory_candidates"].([]interface{})
	mcB, _ := timelineB["memory_candidates"].([]interface{})
	fmt.Printf("  Memory:    %d candidates → %d candidates\n", len(mcA), len(mcB))

	fmt.Println()
	return nil
}
