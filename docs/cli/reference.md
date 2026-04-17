# ContextOps CLI Reference

## Installation

```bash
# From source
go install github.com/contextops/contextops/apps/cli@latest

# From repo
cd apps/cli && go build -o contextops .

# Via Makefile
make build-cli
```

## Global Flags

```
--config string   Config file (default .contextops/config.yaml)
--api-url string  ContextOps API URL (default http://localhost:8080)
--tenant string   Tenant ID
```

## Commands

### contextops init

Initialize a ContextOps project in the current directory.

```bash
contextops init
contextops init --name my-agent-project
contextops init --template rag
```

Creates `.contextops/` with `config.yaml`, `gates.yaml`, `benchmarks/`, and `datasets/`.

### contextops server

Manage local ContextOps services.

```bash
contextops server start          # Start via Docker Compose
contextops server start --dev    # Development mode
contextops server stop           # Stop all services
contextops server status         # Show service status
```

### contextops trace

Work with traces (agent run records).

```bash
contextops trace ingest ./run.json           # Ingest a single trace
contextops trace ingest ./runs/ --recursive  # Ingest from directory
contextops trace show <run-id>               # Show trace details
contextops trace list                        # List all traces
contextops trace list --status failed        # Filter by status
contextops trace list --agent policy-agent   # Filter by agent
contextops trace list --since 7d             # Filter by time
contextops trace validate ./run.json         # Validate against schema
contextops trace export <run-id> --format json
```

### contextops manifest

Inspect context manifests (how context was assembled).

```bash
contextops manifest show <run-id>                   # Show manifest
contextops manifest show <run-id> --format json      # JSON output
contextops manifest diff <run-a> <run-b>             # Diff two manifests
```

### contextops eval

Run evaluators on runs or datasets.

```bash
contextops eval run <run-id>                                  # All evaluators
contextops eval run <run-id> --only correctness,groundedness  # Specific evaluators
contextops eval run <run-id> --explain                        # With explanations
contextops eval dataset enterprise-search-v1                  # Evaluate dataset
```

### contextops benchmark

Execute benchmark suites.

```bash
contextops benchmark list                                    # List suites
contextops benchmark run enterprise-search                   # Run a suite
contextops benchmark run workflow-agent --dataset ./datasets/
```

### contextops compare

Compare runs, datasets, or benchmark outputs.

```bash
contextops compare <run-a> <run-b>                           # Compare two runs
contextops compare <run-a> <run-b> --metric correctness      # Focus on metric
contextops compare benchmark enterprise-search --baseline v0.1 --candidate v0.2
```

### contextops gate

Enforce quality and safety thresholds.

```bash
contextops gate check --config .contextops/gates.yaml        # Check gates
contextops gate check --benchmark enterprise-search          # Check benchmark
contextops gate check --run-id <run-id>                      # Check specific run
```

Exit code 1 if any gate fails — use in CI pipelines.

### contextops report

Generate human-readable reports.

```bash
contextops report generate <run-id>                          # Text report
contextops report generate <run-id> --format markdown        # Markdown
contextops report generate <run-id> --format json            # JSON
contextops report generate <run-id> --format html            # HTML
```

### contextops adapter

Scaffold and manage framework integrations.

```bash
contextops adapter list                                      # List adapter types
contextops adapter scaffold rest                             # Scaffold REST adapter
contextops adapter scaffold langgraph                        # Scaffold LangGraph
contextops adapter scaffold openai-agents                    # Scaffold OpenAI
contextops adapter scaffold otel                             # Scaffold OTEL
```

### contextops dataset

Manage evaluation datasets.

```bash
contextops dataset list                                      # List datasets
contextops dataset create hr-policy-tests                    # Create dataset
contextops dataset validate ./datasets/hr-policy/            # Validate
```

### contextops config

View and manage configuration.

```bash
contextops config show                                       # Show config
contextops config validate                                   # Validate config
contextops config set api.url http://localhost:8080           # Set value
contextops config set tenant.default acme-dev                # Set tenant
```
