# Contributing to ContextOps

We welcome contributions from the community. Here's how to get started.

## Areas We Need Help

- **Evaluators** — New evaluation modules (e.g., citation coverage, multi-hop reasoning)
- **Adapters** — Integrations for agent frameworks (LangGraph, CrewAI, AutoGen, etc.)
- **Benchmark datasets** — Real-world test scenarios for different domains
- **Memory diagnostics** — Better stale memory detection, conflict resolution
- **Permissions test harnesses** — ACL simulation and compliance testing
- **Documentation** — Architecture guides, tutorials, API reference

## Development Setup

```bash
# Clone the repo
git clone https://github.com/contextops/contextops.git
cd contextops

# Start services
docker compose up -d

# Build the CLI
cd apps/cli && go build -o contextops . && cd ../..

# Run Go API tests
cd apps/api && go test ./... && cd ../..

# Run Python evaluator tests
cd apps/evaluator && python -m pytest tests/ -v && cd ../..
```

## Project Structure

- `apps/api/` — Go REST API (Gin)
- `apps/cli/` — Go CLI (Cobra)
- `apps/evaluator/` — Python evaluation engine (FastAPI)
- `apps/web/` — Next.js debugger UI
- `packages/` — Shared libraries and schemas
- `adapters/` — Framework integrations
- `benchmarks/` — Benchmark scenario packs
- `deploy/` — Docker and Kubernetes configs

## Adding an Evaluator

1. Create a new file in `apps/evaluator/evaluators/`
2. Extend `BaseEvaluator` from `evaluators/base.py`
3. Implement the `async evaluate(self, run_data: RunData) -> EvaluationResult` method
4. Register it in `evaluators/registry.py`
5. Add tests in `apps/evaluator/tests/`

## Adding an Adapter

```bash
contextops adapter scaffold <type>
```

Or manually create a file in `adapters/<type>/adapter.py`.

## Adding a Benchmark Pack

1. Create a directory under `benchmarks/<name>/`
2. Add a `README.md` describing the benchmark
3. Add test case JSON files in `benchmarks/<name>/cases/`
4. Each case should include: query, expected answer, expected sources, and pass criteria

## Code Style

- **Go**: `gofmt`, `golint`
- **Python**: `black`, `ruff`
- **TypeScript**: `prettier`, `eslint`

## Pull Request Process

1. Fork the repo and create a feature branch
2. Make your changes with tests
3. Ensure all tests pass
4. Submit a PR with a clear description
5. Address review feedback

## Commit Messages

Use conventional commits:

```
feat: add citation coverage evaluator
fix: handle missing memory candidates in eval
docs: add adapter scaffolding guide
test: add permission safety edge cases
```

## License

By contributing, you agree that your contributions will be licensed under the Apache-2.0 License.
