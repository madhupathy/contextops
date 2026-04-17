"""Pydantic models for the evaluator service."""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field


class HealthResponse(BaseModel):
    status: str
    service: str
    version: str


class RunData(BaseModel):
    """Complete run data fetched from the database."""

    run: dict[str, Any]
    retrieval_candidates: list[dict[str, Any]] = Field(default_factory=list)
    memory_candidates: list[dict[str, Any]] = Field(default_factory=list)
    tool_calls: list[dict[str, Any]] = Field(default_factory=list)
    reasoning_steps: list[dict[str, Any]] = Field(default_factory=list)


class EvaluationResult(BaseModel):
    """Result of a single evaluation."""

    evaluator_name: str
    evaluator_version: str = "v1"
    category: str
    score: float = Field(ge=0.0, le=1.0)
    passed: bool
    details: dict[str, Any] = Field(default_factory=dict)
    reasoning: str | None = None
    model_used: str | None = None
    eval_tokens: int = 0
    eval_latency_ms: int = 0


class EvaluateRequest(BaseModel):
    """Request to evaluate a run."""

    run_id: str
    tenant_id: str = "00000000-0000-0000-0000-000000000001"
    categories: list[str] | None = None


class EvaluateResponse(BaseModel):
    """Response from evaluation."""

    run_id: str
    results: list[EvaluationResult]
