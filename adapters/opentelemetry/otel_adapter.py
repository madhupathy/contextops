"""
ContextOps OpenTelemetry Span Processor
========================================
Drop this processor into any OTEL-instrumented Python application to
automatically capture LLM / RAG spans and submit them to ContextOps as
structured trace runs.

Supported instrumentors
-----------------------
* opentelemetry-instrumentation-openai  (span name starts with "openai")
* opentelemetry-instrumentation-anthropic  (span name starts with "anthropic")
* opentelemetry-instrumentation-langchain (chain / retriever / tool spans)
* Any OTEL span that sets the ``gen_ai.*`` semantic convention attributes.

Quick start
-----------
    from contextops.adapters.opentelemetry import ContextOpsSpanProcessor

    tracer_provider.add_span_processor(
        ContextOpsSpanProcessor(
            api_url="http://localhost:8080",
            api_key="your-api-key",
            project="my-rag-app",
        )
    )

Environment variable overrides
-------------------------------
CONTEXTOPS_API_URL   — overrides ``api_url`` constructor argument
CONTEXTOPS_API_KEY   — overrides ``api_key`` constructor argument
CONTEXTOPS_PROJECT   — overrides ``project`` constructor argument
CONTEXTOPS_DISABLED  — set to "1" / "true" to disable the processor at runtime
"""

from __future__ import annotations

import json
import logging
import os
import threading
import time
import urllib.request
import urllib.error
from typing import Any, Optional, Sequence

# ---------------------------------------------------------------------------
# Optional OTEL import — we guard so the file can be imported even without
# the opentelemetry-sdk installed (e.g. for unit tests that mock spans).
# ---------------------------------------------------------------------------
try:
    from opentelemetry.sdk.trace import ReadableSpan
    from opentelemetry.sdk.trace.export import SpanExporter, SpanExportResult
    from opentelemetry.sdk.trace import SpanProcessor
    from opentelemetry.trace import SpanKind, StatusCode

    _OTEL_AVAILABLE = True
except ImportError:  # pragma: no cover
    _OTEL_AVAILABLE = False

    class SpanProcessor:  # type: ignore[no-redef]
        """Stub when opentelemetry-sdk is not installed."""

    class ReadableSpan:  # type: ignore[no-redef]
        pass

    class StatusCode:  # type: ignore[no-redef]
        ERROR = 2

    SpanKind = None  # type: ignore[assignment]

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Attribute key constants (gen_ai semantic conventions + common instrumentors)
# ---------------------------------------------------------------------------
_GEN_AI_SYSTEM              = "gen_ai.system"
_GEN_AI_REQUEST_MODEL       = "gen_ai.request.model"
_GEN_AI_RESPONSE_MODEL      = "gen_ai.response.model"
_GEN_AI_USAGE_INPUT_TOKENS  = "gen_ai.usage.input_tokens"
_GEN_AI_USAGE_OUTPUT_TOKENS = "gen_ai.usage.output_tokens"
_GEN_AI_PROMPT              = "gen_ai.prompt"
_GEN_AI_COMPLETION          = "gen_ai.completion"

# LangChain / retrieval
_LC_RETRIEVAL_QUERY         = "langchain.retrieval.query"
_LC_RETRIEVED_DOCS          = "langchain.retrieval.documents"
_TRACELOOP_ENTITY_INPUT     = "traceloop.entity.input"
_TRACELOOP_ENTITY_OUTPUT    = "traceloop.entity.output"

# OpenAI instrumentor (opentelemetry-instrumentation-openai)
_OPENAI_PROMPT_TOKENS       = "openai.usage.prompt_tokens"
_OPENAI_COMPLETION_TOKENS   = "openai.usage.completion_tokens"

# Anthropic instrumentor
_ANTHROPIC_INPUT_TOKENS     = "anthropic.usage.input_tokens"
_ANTHROPIC_OUTPUT_TOKENS    = "anthropic.usage.output_tokens"

# Generic "llm.*" used by some community instrumentors
_LLM_PROMPTS                = "llm.prompts"
_LLM_COMPLETIONS            = "llm.completions"
_LLM_USAGE_TOTAL_TOKENS     = "llm.usage.total_tokens"

# Span names that identify LLM calls (prefix-matched, lower-cased)
_LLM_SPAN_PREFIXES = (
    "openai",
    "anthropic",
    "chat",
    "completion",
    "llm",
    "langchain.llm",
    "langchain.chat_models",
)

# Span names that identify retrieval steps
_RETRIEVAL_SPAN_PREFIXES = (
    "retriever",
    "langchain.retriever",
    "retrieve",
    "vector_store",
    "pinecone",
    "weaviate",
    "chroma",
    "qdrant",
    "faiss",
)


def _attr(span: "ReadableSpan", *keys: str, default: Any = None) -> Any:
    """Return the first matching attribute value from a span."""
    attrs = span.attributes or {}
    for key in keys:
        if key in attrs:
            return attrs[key]
    return default


def _span_name_matches(span: "ReadableSpan", prefixes: tuple[str, ...]) -> bool:
    name = (span.name or "").lower()
    return any(name.startswith(p) for p in prefixes)


def _parse_json_safe(value: Any) -> Any:
    if isinstance(value, str):
        try:
            return json.loads(value)
        except (json.JSONDecodeError, ValueError):
            return value
    return value


class ContextOpsSpanProcessor(SpanProcessor):
    """
    OpenTelemetry :class:`SpanProcessor` that submits completed LLM spans to
    the ContextOps API as trace runs.

    Parameters
    ----------
    api_url:
        Base URL of the ContextOps API server (e.g. ``http://localhost:8080``).
        Overridden by ``CONTEXTOPS_API_URL`` env var.
    api_key:
        API key for bearer-token authentication.
        Overridden by ``CONTEXTOPS_API_KEY`` env var.
    project:
        Logical project / agent name attached to every submitted run.
        Overridden by ``CONTEXTOPS_PROJECT`` env var.
    min_duration_ms:
        Ignore spans shorter than this (default 0 — capture everything).
    capture_prompts:
        If ``True`` (default), include prompt text in the submitted run.
        Set to ``False`` to avoid sending PII / sensitive prompts.
    capture_retrieved_contexts:
        If ``True`` (default), extract and forward retrieved document chunks.
    on_error:
        Callback ``(exc: Exception) -> None`` invoked on submission failures.
        Default: log a warning.
    max_queue_size:
        Max number of spans buffered before the background thread flushes.
    flush_interval_seconds:
        How often the background flush thread runs (default: 2 s).
    """

    def __init__(
        self,
        api_url: Optional[str] = None,
        api_key: Optional[str] = None,
        project: Optional[str] = None,
        *,
        min_duration_ms: float = 0,
        capture_prompts: bool = True,
        capture_retrieved_contexts: bool = True,
        on_error: Any = None,
        max_queue_size: int = 500,
        flush_interval_seconds: float = 2.0,
    ) -> None:
        self._api_url = (
            os.environ.get("CONTEXTOPS_API_URL") or api_url or "http://localhost:8080"
        ).rstrip("/")
        self._api_key = os.environ.get("CONTEXTOPS_API_KEY") or api_key or ""
        self._project = os.environ.get("CONTEXTOPS_PROJECT") or project or "default"
        self._min_duration_ms = min_duration_ms
        self._capture_prompts = capture_prompts
        self._capture_retrieved_contexts = capture_retrieved_contexts
        self._on_error = on_error or self._default_on_error
        self._disabled = os.environ.get("CONTEXTOPS_DISABLED", "").lower() in (
            "1",
            "true",
            "yes",
        )

        # Background flush queue
        self._queue: list[dict] = []
        self._lock = threading.Lock()
        self._max_queue_size = max_queue_size
        self._flush_interval = flush_interval_seconds
        self._shutdown = threading.Event()

        self._flush_thread = threading.Thread(
            target=self._flush_loop,
            name="contextops-otel-flush",
            daemon=True,
        )
        self._flush_thread.start()

    # ------------------------------------------------------------------
    # SpanProcessor interface
    # ------------------------------------------------------------------

    def on_start(self, span: "ReadableSpan", parent_context: Any = None) -> None:
        pass  # nothing to do at span start

    def on_end(self, span: "ReadableSpan") -> None:
        if self._disabled:
            return
        try:
            payload = self._extract_run(span)
        except Exception as exc:  # noqa: BLE001
            self._on_error(exc)
            return

        if payload is None:
            return  # span was not an LLM/retrieval span — skip

        with self._lock:
            if len(self._queue) < self._max_queue_size:
                self._queue.append(payload)
            else:
                logger.warning(
                    "contextops-otel: queue is full (%d items), dropping span %s",
                    self._max_queue_size,
                    span.name,
                )

    def shutdown(self) -> None:
        self._shutdown.set()
        self._flush_thread.join(timeout=10)
        self._flush_all()

    def force_flush(self, timeout_millis: int = 30_000) -> bool:  # noqa: ARG002
        self._flush_all()
        return True

    # ------------------------------------------------------------------
    # Span extraction
    # ------------------------------------------------------------------

    def _extract_run(self, span: "ReadableSpan") -> Optional[dict]:
        """
        Convert a completed OTEL span into a ContextOps run payload.
        Returns ``None`` if the span should be ignored.
        """
        is_llm_span = _span_name_matches(span, _LLM_SPAN_PREFIXES)
        is_retrieval_span = _span_name_matches(span, _RETRIEVAL_SPAN_PREFIXES)

        if not is_llm_span and not is_retrieval_span:
            # Also accept spans with gen_ai attributes regardless of name
            attrs = span.attributes or {}
            if not any(k.startswith("gen_ai.") or k.startswith("llm.") for k in attrs):
                return None

        # Duration filter
        if span.start_time and span.end_time:
            duration_ns = span.end_time - span.start_time
            duration_ms = duration_ns / 1_000_000
        else:
            duration_ms = 0

        if duration_ms < self._min_duration_ms:
            return None

        # --- Model ---
        model = _attr(
            span,
            _GEN_AI_RESPONSE_MODEL,
            _GEN_AI_REQUEST_MODEL,
            "llm.model",
            "model",
        )

        # --- Token counts ---
        input_tokens = _attr(
            span,
            _GEN_AI_USAGE_INPUT_TOKENS,
            _OPENAI_PROMPT_TOKENS,
            _ANTHROPIC_INPUT_TOKENS,
            "llm.usage.prompt_tokens",
        )
        output_tokens = _attr(
            span,
            _GEN_AI_USAGE_OUTPUT_TOKENS,
            _OPENAI_COMPLETION_TOKENS,
            _ANTHROPIC_OUTPUT_TOKENS,
            "llm.usage.completion_tokens",
        )
        total_tokens_raw = _attr(span, _LLM_USAGE_TOTAL_TOKENS, "llm.usage.total_tokens")
        if total_tokens_raw is None and input_tokens is not None and output_tokens is not None:
            total_tokens_raw = int(input_tokens) + int(output_tokens)
        total_tokens = int(total_tokens_raw) if total_tokens_raw is not None else 0

        # --- Query / response text ---
        query = ""
        response = ""

        if self._capture_prompts:
            raw_prompt = _attr(
                span, _GEN_AI_PROMPT, _LLM_PROMPTS,
                _TRACELOOP_ENTITY_INPUT, "llm.input",
            )
            if raw_prompt:
                parsed = _parse_json_safe(raw_prompt)
                if isinstance(parsed, list):
                    # Extract the last user message
                    for msg in reversed(parsed):
                        if isinstance(msg, dict) and msg.get("role") == "user":
                            query = msg.get("content", "")
                            break
                    if not query and parsed:
                        last = parsed[-1]
                        query = (
                            last.get("content", "") if isinstance(last, dict) else str(last)
                        )
                else:
                    query = str(parsed)[:2000]

            raw_completion = _attr(
                span, _GEN_AI_COMPLETION, _LLM_COMPLETIONS,
                _TRACELOOP_ENTITY_OUTPUT, "llm.output",
            )
            if raw_completion:
                parsed_c = _parse_json_safe(raw_completion)
                if isinstance(parsed_c, list) and parsed_c:
                    first = parsed_c[0]
                    if isinstance(first, dict):
                        # choices[0].message.content pattern
                        msg = first.get("message", first)
                        response = msg.get("content", str(first)) if isinstance(msg, dict) else str(first)
                    else:
                        response = str(first)
                else:
                    response = str(parsed_c)[:4000]

        # If span has no query, fall back to the span name so runs are identifiable
        if not query:
            query = f"[{span.name}]"

        # --- Retrieved contexts ---
        retrieved_contexts: list[str] = []
        if self._capture_retrieved_contexts:
            raw_docs = _attr(span, _LC_RETRIEVED_DOCS, "retrieval.documents", "documents")
            if raw_docs:
                parsed_docs = _parse_json_safe(raw_docs)
                if isinstance(parsed_docs, list):
                    for doc in parsed_docs[:10]:  # cap at 10 chunks
                        if isinstance(doc, dict):
                            retrieved_contexts.append(
                                doc.get("page_content")
                                or doc.get("content")
                                or doc.get("text")
                                or json.dumps(doc)
                            )
                        else:
                            retrieved_contexts.append(str(doc))
                elif isinstance(parsed_docs, str):
                    retrieved_contexts.append(parsed_docs)

        # --- Status ---
        status = "completed"
        if hasattr(span, "status") and span.status:
            if hasattr(StatusCode, "ERROR") and span.status.status_code == StatusCode.ERROR:
                status = "failed"

        # --- Trace / span IDs for correlation ---
        trace_id = ""
        span_id = ""
        if hasattr(span, "context") and span.context:
            trace_id = format(span.context.trace_id, "032x")
            span_id = format(span.context.span_id, "016x")

        payload: dict = {
            "query": query[:2000],
            "response": response[:8000],
            "model": model,
            "total_tokens": total_tokens,
            "latency_ms": int(duration_ms),
            "status": status,
            "metadata": {
                "project": self._project,
                "span_name": span.name,
                "otel_trace_id": trace_id,
                "otel_span_id": span_id,
                "gen_ai_system": _attr(span, _GEN_AI_SYSTEM, "llm.vendor"),
            },
        }
        if retrieved_contexts:
            payload["retrieved_contexts"] = retrieved_contexts

        # Strip None metadata values
        payload["metadata"] = {k: v for k, v in payload["metadata"].items() if v is not None}

        return payload

    # ------------------------------------------------------------------
    # HTTP submission
    # ------------------------------------------------------------------

    def _submit(self, payload: dict) -> None:
        url = f"{self._api_url}/api/v1/runs"
        body = json.dumps(payload).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=body,
            method="POST",
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {self._api_key}",
            },
        )
        try:
            with urllib.request.urlopen(req, timeout=10) as resp:
                if resp.status not in (200, 201):
                    raise RuntimeError(
                        f"ContextOps API returned status {resp.status} for run submission"
                    )
        except urllib.error.HTTPError as exc:
            raise RuntimeError(
                f"ContextOps API HTTP error {exc.code}: {exc.reason}"
            ) from exc

    # ------------------------------------------------------------------
    # Background flush
    # ------------------------------------------------------------------

    def _flush_loop(self) -> None:
        while not self._shutdown.is_set():
            time.sleep(self._flush_interval)
            self._flush_all()

    def _flush_all(self) -> None:
        with self._lock:
            items, self._queue = self._queue, []

        for payload in items:
            try:
                self._submit(payload)
            except Exception as exc:  # noqa: BLE001
                self._on_error(exc)

    @staticmethod
    def _default_on_error(exc: Exception) -> None:
        logger.warning("contextops-otel: failed to submit span — %s", exc)


# ---------------------------------------------------------------------------
# Convenience alias so ``from contextops.adapters.opentelemetry import ...``
# and ``from adapters.opentelemetry.otel_adapter import ...`` both work.
# ---------------------------------------------------------------------------
__all__ = ["ContextOpsSpanProcessor"]
