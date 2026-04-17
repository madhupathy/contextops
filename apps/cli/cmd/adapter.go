package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var adapterCmd = &cobra.Command{
	Use:   "adapter",
	Short: "Scaffold and validate adapters",
}

var adapterListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available adapter types",
	RunE:  runAdapterList,
}

var adapterScaffoldCmd = &cobra.Command{
	Use:   "scaffold <type>",
	Short: "Scaffold an adapter integration",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdapterScaffold,
}

func init() {
	rootCmd.AddCommand(adapterCmd)
	adapterCmd.AddCommand(adapterListCmd)
	adapterCmd.AddCommand(adapterScaffoldCmd)
}

func runAdapterList(cmd *cobra.Command, args []string) error {
	fmt.Println("Available adapter types:")
	fmt.Println()
	fmt.Printf("  %-20s %s\n", "rest", "Generic REST callback adapter")
	fmt.Printf("  %-20s %s\n", "langgraph", "LangGraph callback adapter")
	fmt.Printf("  %-20s %s\n", "openai-agents", "OpenAI Agents SDK adapter")
	fmt.Printf("  %-20s %s\n", "otel", "OpenTelemetry span adapter")
	fmt.Println()
	fmt.Println("Usage: contextops adapter scaffold <type>")
	return nil
}

func runAdapterScaffold(cmd *cobra.Command, args []string) error {
	adapterType := args[0]

	templates := map[string]string{
		"rest": `# ContextOps REST Adapter
#
# Send traces to ContextOps via HTTP POST.
#
# Endpoint: POST /api/v1/runs
# Header:   X-Tenant-ID: <your-tenant-id>
# Body:     JSON trace (see schema below)

import requests
import json

CONTEXTOPS_URL = "http://localhost:8080"
TENANT_ID = "00000000-0000-0000-0000-000000000001"

def send_trace(trace: dict):
    """Send a trace to ContextOps."""
    resp = requests.post(
        f"{CONTEXTOPS_URL}/api/v1/runs",
        json=trace,
        headers={
            "Content-Type": "application/json",
            "X-Tenant-ID": TENANT_ID,
        },
    )
    resp.raise_for_status()
    return resp.json()

# Example usage:
# trace = {
#     "query": "What is the latest PTO policy?",
#     "model": "gpt-4o",
#     "final_answer": "The PTO policy allows...",
#     "retrieval_candidates": [...],
#     "tool_calls": [...],
# }
# result = send_trace(trace)
`,
		"langgraph": `# ContextOps LangGraph Adapter
#
# Captures LangGraph execution traces and sends them to ContextOps.

from typing import Any, Dict
import requests

CONTEXTOPS_URL = "http://localhost:8080"
TENANT_ID = "00000000-0000-0000-0000-000000000001"


class ContextOpsCallback:
    """LangGraph callback handler that captures traces for ContextOps."""

    def __init__(self, api_url: str = CONTEXTOPS_URL, tenant_id: str = TENANT_ID):
        self.api_url = api_url
        self.tenant_id = tenant_id
        self.trace: Dict[str, Any] = {}
        self.steps = []
        self.tool_calls = []

    def on_chain_start(self, name: str, inputs: Dict[str, Any]):
        self.trace["query"] = inputs.get("query", inputs.get("input", ""))
        self.trace["started_at"] = self._now()

    def on_tool_start(self, name: str, args: Dict[str, Any]):
        self.tool_calls.append({
            "tool_name": name,
            "tool_args": args,
            "status": "running",
        })

    def on_tool_end(self, name: str, result: Any):
        for tc in self.tool_calls:
            if tc["tool_name"] == name and tc["status"] == "running":
                tc["status"] = "success"
                tc["tool_result"] = str(result)
                break

    def on_chain_end(self, output: Any):
        self.trace["final_answer"] = str(output)
        self.trace["tool_calls"] = self.tool_calls
        self.trace["reasoning_steps"] = self.steps
        self._send()

    def _send(self):
        requests.post(
            f"{self.api_url}/api/v1/runs",
            json=self.trace,
            headers={
                "Content-Type": "application/json",
                "X-Tenant-ID": self.tenant_id,
            },
        )

    def _now(self):
        from datetime import datetime, timezone
        return datetime.now(timezone.utc).isoformat()
`,
		"openai-agents": `# ContextOps OpenAI Agents SDK Adapter
#
# Wraps OpenAI Agents SDK to capture execution traces.

import requests
from typing import Any, Dict, List

CONTEXTOPS_URL = "http://localhost:8080"
TENANT_ID = "00000000-0000-0000-0000-000000000001"


class ContextOpsTracer:
    """Traces OpenAI Agent runs and sends them to ContextOps."""

    def __init__(self, api_url: str = CONTEXTOPS_URL, tenant_id: str = TENANT_ID):
        self.api_url = api_url
        self.tenant_id = tenant_id

    def trace_run(
        self,
        query: str,
        agent_id: str,
        model: str,
        final_answer: str,
        tool_calls: List[Dict[str, Any]] = None,
        retrieval_candidates: List[Dict[str, Any]] = None,
        tokens: Dict[str, int] = None,
        latency_ms: int = 0,
    ) -> dict:
        """Send a completed agent run trace to ContextOps."""
        trace = {
            "query": query,
            "agent_id": agent_id,
            "model": model,
            "final_answer": final_answer,
            "tool_calls": tool_calls or [],
            "retrieval_candidates": retrieval_candidates or [],
            "prompt_tokens": (tokens or {}).get("prompt", 0),
            "completion_tokens": (tokens or {}).get("completion", 0),
            "total_tokens": (tokens or {}).get("total", 0),
            "latency_ms": latency_ms,
            "status": "completed",
        }

        resp = requests.post(
            f"{self.api_url}/api/v1/runs",
            json=trace,
            headers={
                "Content-Type": "application/json",
                "X-Tenant-ID": self.tenant_id,
            },
        )
        resp.raise_for_status()
        return resp.json()
`,
		"otel": `# ContextOps OpenTelemetry Adapter
#
# Converts OTEL spans into ContextOps traces.
# This adapter can run as a collector processor or standalone converter.

import json
from typing import Any, Dict, List

CONTEXTOPS_URL = "http://localhost:8080"
TENANT_ID = "00000000-0000-0000-0000-000000000001"


def otel_spans_to_trace(spans: List[Dict[str, Any]]) -> dict:
    """Convert a list of OTEL spans into a ContextOps trace."""
    trace = {
        "status": "completed",
        "reasoning_steps": [],
        "tool_calls": [],
    }

    for span in spans:
        name = span.get("name", "")
        attrs = span.get("attributes", {})
        kind = span.get("kind", "")

        # Extract query from root span
        if "query" in attrs or "input" in attrs:
            trace["query"] = attrs.get("query", attrs.get("input", ""))

        # Extract model info
        if "llm.model" in attrs:
            trace["model"] = attrs["llm.model"]

        # Extract tool calls
        if "tool" in name.lower() or kind == "TOOL":
            trace["tool_calls"].append({
                "tool_name": attrs.get("tool.name", name),
                "tool_args": json.loads(attrs.get("tool.args", "{}")),
                "status": "success" if span.get("status", {}).get("code") != "ERROR" else "failure",
            })

        # Extract final answer
        if "output" in attrs or "response" in attrs:
            trace["final_answer"] = attrs.get("output", attrs.get("response", ""))

        # Extract token counts
        if "llm.token_count.prompt" in attrs:
            trace["prompt_tokens"] = int(attrs["llm.token_count.prompt"])
        if "llm.token_count.completion" in attrs:
            trace["completion_tokens"] = int(attrs["llm.token_count.completion"])

    return trace
`,
	}

	template, ok := templates[adapterType]
	if !ok {
		return fmt.Errorf("unknown adapter type: %s\nAvailable: %s", adapterType, strings.Join([]string{"rest", "langgraph", "openai-agents", "otel"}, ", "))
	}

	dir := filepath.Join("adapters", adapterType)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create adapter dir: %w", err)
	}

	filename := filepath.Join(dir, "adapter.py")
	if err := os.WriteFile(filename, []byte(template), 0o644); err != nil {
		return fmt.Errorf("write adapter: %w", err)
	}

	fmt.Printf("Scaffolded %s adapter at %s\n", adapterType, filename)
	return nil
}
