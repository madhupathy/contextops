# trace-schema

Shared run/trace schema definition for ContextOps.

Defines the canonical JSON structure for AI agent traces including:
- Input query and user identity
- Retrieval candidates and selections
- Memory candidates and selections
- Permission/ACL filter results
- Tool calls and reasoning steps
- Output, citations, and metrics

Used by: API service, CLI, evaluator, adapters.
