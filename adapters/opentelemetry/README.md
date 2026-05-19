# ContextOps OpenTelemetry Adapter

An [OpenTelemetry](https://opentelemetry.io/) `SpanProcessor` that captures
LLM and retrieval spans from any instrumented Python application and
automatically submits them to ContextOps as structured trace runs.

## How it works

```
Your App  →  OTEL Instrumentor  →  SpanProcessor  →  ContextOps API
                (OpenAI / Anthropic /              (POST /api/v1/runs)
                 LangChain / custom)
```

The processor sits in the OpenTelemetry span pipeline.  Every time an LLM
call, chat completion, or retrieval span completes, the processor:

1. Extracts the query, response, model, token counts, latency, and any
   retrieved document chunks from span attributes.
2. Maps them to the ContextOps run schema.
3. Queues the payload for non-blocking background delivery to the API.

## Installation

```bash
pip install opentelemetry-sdk

# Add whichever instrumentor matches your LLM provider
pip install opentelemetry-instrumentation-openai        # OpenAI
pip install opentelemetry-instrumentation-anthropic     # Anthropic
pip install opentelemetry-instrumentation-langchain     # LangChain / RAG
```

The adapter itself has **no extra runtime dependencies** beyond the
OpenTelemetry SDK — it uses only the Python standard library for HTTP.

## Quick start

```python
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter

# ContextOps adapter
from adapters.opentelemetry.otel_adapter import ContextOpsSpanProcessor

tracer_provider = TracerProvider()

# Add the ContextOps processor — it runs alongside your other exporters
tracer_provider.add_span_processor(
    ContextOpsSpanProcessor(
        api_url="http://localhost:8080",
        api_key="your-api-key",
        project="my-rag-app",
    )
)
```

### With OpenAI

```python
from opentelemetry.instrumentation.openai import OpenAIInstrumentor
from openai import OpenAI

OpenAIInstrumentor().instrument(tracer_provider=tracer_provider)
client = OpenAI()

# This call is automatically captured and submitted to ContextOps
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "What is our PTO policy?"}],
)
```

### With Anthropic

```python
from opentelemetry.instrumentation.anthropic import AnthropicInstrumentor
import anthropic

AnthropicInstrumentor().instrument(tracer_provider=tracer_provider)
client = anthropic.Anthropic()

message = client.messages.create(
    model="claude-opus-4-5",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Summarise our Q3 results."}],
)
```

### With LangChain (RAG pipeline)

```python
from opentelemetry.instrumentation.langchain import LangchainInstrumentor
from langchain.chains import RetrievalQA

LangchainInstrumentor().instrument(tracer_provider=tracer_provider)

# Retrieved document chunks are extracted automatically from retriever spans
qa_chain = RetrievalQA.from_chain_type(llm=llm, retriever=retriever)
result = qa_chain.run("What is the refund policy?")
```

## Configuration

| Parameter | Env var | Default | Description |
|-----------|---------|---------|-------------|
| `api_url` | `CONTEXTOPS_API_URL` | `http://localhost:8080` | ContextOps API base URL |
| `api_key` | `CONTEXTOPS_API_KEY` | `""` | Bearer token for auth |
| `project` | `CONTEXTOPS_PROJECT` | `"default"` | Logical project name |
| `capture_prompts` | — | `True` | Include prompt/completion text |
| `capture_retrieved_contexts` | — | `True` | Forward retrieved document chunks |
| `min_duration_ms` | — | `0` | Skip spans shorter than this |
| `max_queue_size` | — | `500` | Max buffered spans before dropping |
| `flush_interval_seconds` | — | `2.0` | Background flush cadence |

Set `CONTEXTOPS_DISABLED=1` to disable the processor at runtime without
removing it from code (useful for local dev without a running API server).

## What spans are captured

The processor captures a span when **any** of the following is true:

- The span name starts with a known LLM prefix: `openai`, `anthropic`,
  `chat`, `completion`, `llm`, `langchain.llm`, `langchain.chat_models`
- The span name starts with a retrieval prefix: `retriever`,
  `langchain.retriever`, `retrieve`, `vector_store`, or a known vector-DB
  name (`pinecone`, `weaviate`, `chroma`, `qdrant`, `faiss`)
- The span carries any `gen_ai.*` or `llm.*` attributes (generic
  semantic-convention approach)

All other spans are ignored and pass through transparently.

## Extracted attributes

| ContextOps field | OTEL attribute(s) tried (in order) |
|------------------|------------------------------------|
| `query` | `gen_ai.prompt`, `llm.prompts`, `traceloop.entity.input` |
| `response` | `gen_ai.completion`, `llm.completions`, `traceloop.entity.output` |
| `model` | `gen_ai.response.model`, `gen_ai.request.model`, `llm.model` |
| `total_tokens` | `gen_ai.usage.input_tokens + output_tokens`, `llm.usage.total_tokens` |
| `latency_ms` | Computed from span `start_time` / `end_time` |
| `retrieved_contexts` | `langchain.retrieval.documents`, `retrieval.documents` |
| `status` | Derived from OTEL `StatusCode` (ERROR → `"failed"`) |

## Error handling

Submission failures are logged at `WARNING` level by default and never
raise — they never interrupt your application.  To handle errors yourself:

```python
def my_error_handler(exc: Exception) -> None:
    sentry_sdk.capture_exception(exc)

ContextOpsSpanProcessor(
    api_url=...,
    api_key=...,
    on_error=my_error_handler,
)
```

## Status

Implemented in v0.4 — replaces the earlier `adapters/otel` stub.
