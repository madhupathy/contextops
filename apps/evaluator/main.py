"""ContextOps Evaluator Service.

12 evaluators covering the full AI agent quality stack:
  Core:        answer_correctness · groundedness · citation_precision · task_completion
  Retrieval:   retrieval_quality · permission_safety
  Memory:      memory_utility · context_poisoning · session_coherence
  Agent:       tool_correctness · trajectory_quality
  Cost:        cost_efficiency
"""

from __future__ import annotations

import logging
import os
import uuid
from contextlib import asynccontextmanager

import psycopg_pool
from fastapi import FastAPI, HTTPException, Query
from fastapi.middleware.cors import CORSMiddleware

from evaluators.registry import EvaluatorRegistry
from models import (
    EvaluateRequest,
    EvaluateResponse,
    EvaluationResult,
    HealthResponse,
    RunData,
)

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger("evaluator")

DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql://contextops:contextops@localhost:5432/contextops",
)
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY", "")

pool: psycopg_pool.AsyncConnectionPool | None = None
registry: EvaluatorRegistry | None = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global pool, registry
    pool = psycopg_pool.AsyncConnectionPool(
        conninfo=DATABASE_URL, min_size=2, max_size=10,
    )
    await pool.open()
    registry = EvaluatorRegistry(openai_api_key=OPENAI_API_KEY)
    logger.info(
        "Evaluator service started — %d evaluators registered: %s",
        len(registry.list_evaluators()),
        [e["category"] for e in registry.list_evaluators()],
    )
    yield
    await pool.close()
    logger.info("Evaluator service stopped")


app = FastAPI(
    title="ContextOps Evaluator",
    description=(
        "Pluggable evaluation engine for enterprise AI agent runs. "
        "12 evaluators covering correctness, retrieval, memory, context poisoning, "
        "session coherence, tool usage, and cost efficiency."
    ),
    version="0.2.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# ─────────────────────────────────────────────
# Health & discovery
# ─────────────────────────────────────────────

@app.get("/health", response_model=HealthResponse)
async def health():
    return HealthResponse(status="ok", service="contextops-evaluator", version="0.2.0")


@app.get("/evaluators")
async def list_evaluators():
    """List all available evaluators with their categories."""
    return {"evaluators": registry.list_evaluators(), "count": len(registry.list_evaluators())}


@app.get("/evaluators/profiles")
async def list_profiles():
    """Return preset evaluator profiles for common use cases."""
    return {
        "profiles": {
            "rag": {
                "description": "RAG pipeline quality — retrieval, grounding, citations, permission safety",
                "categories": registry.categories_for_intent("rag"),
            },
            "agent": {
                "description": "Agentic task execution — tool correctness, trajectory, task completion",
                "categories": registry.categories_for_intent("agent"),
            },
            "memory": {
                "description": "Memory & context quality — staleness, coherence, context poisoning",
                "categories": registry.categories_for_intent("memory"),
            },
            "enterprise": {
                "description": "Enterprise safety — permissions, citations, grounding, context poisoning",
                "categories": registry.categories_for_intent("enterprise"),
            },
            "full": {
                "description": "All 12 evaluators",
                "categories": registry.categories_for_intent("full"),
            },
        }
    }


# ─────────────────────────────────────────────
# Evaluation endpoints
# ─────────────────────────────────────────────

@app.post("/evaluate", response_model=EvaluateResponse)
async def evaluate_run(request: EvaluateRequest):
    """Evaluate a single run across specified (or all) categories."""
    run_data = await _fetch_run_data(request.run_id, request.tenant_id)
    if run_data is None:
        raise HTTPException(status_code=404, detail=f"Run {request.run_id!r} not found")

    categories = request.categories or registry.default_categories()
    results = await _run_evaluations(run_data, categories, request.run_id, request.tenant_id)
    return EvaluateResponse(run_id=request.run_id, results=results)


@app.post("/evaluate/profile")
async def evaluate_with_profile(
    run_id: str,
    tenant_id: str = "00000000-0000-0000-0000-000000000001",
    profile: str = Query(default="full", description="rag | agent | memory | enterprise | full"),
):
    """Evaluate a run using a named profile (preset category set)."""
    categories = registry.categories_for_intent(profile)
    if not categories:
        raise HTTPException(status_code=400, detail=f"Unknown profile: {profile!r}")

    run_data = await _fetch_run_data(run_id, tenant_id)
    if run_data is None:
        raise HTTPException(status_code=404, detail=f"Run {run_id!r} not found")

    results = await _run_evaluations(run_data, categories, run_id, tenant_id)
    passed = sum(1 for r in results if r.passed)
    avg_score = sum(r.score for r in results) / len(results) if results else 0.0

    return {
        "run_id": run_id,
        "profile": profile,
        "categories_evaluated": len(results),
        "passed": passed,
        "failed": len(results) - passed,
        "avg_score": round(avg_score, 3),
        "results": results,
    }


@app.post("/evaluate/batch")
async def evaluate_batch(request: dict):
    """Evaluate multiple runs in one call."""
    run_ids = request.get("run_ids", [])
    tenant_id = request.get("tenant_id", "00000000-0000-0000-0000-000000000001")
    categories = request.get("categories")
    profile = request.get("profile")

    if not run_ids:
        raise HTTPException(status_code=400, detail="run_ids must be a non-empty list")

    if profile and not categories:
        categories = registry.categories_for_intent(profile)

    results: dict = {}
    for run_id in run_ids:
        try:
            run_data = await _fetch_run_data(run_id, tenant_id)
            if run_data is None:
                results[run_id] = {"error": "run not found"}
                continue
            cats = categories or registry.default_categories()
            evals = await _run_evaluations(run_data, cats, run_id, tenant_id)
            results[run_id] = {
                "results": evals,
                "passed": sum(1 for r in evals if r.passed),
                "avg_score": round(sum(r.score for r in evals) / len(evals), 3) if evals else 0.0,
            }
        except Exception as exc:
            logger.exception("Batch eval failed for run %s", run_id)
            results[run_id] = {"error": str(exc)}

    return {"results": results, "total_runs": len(run_ids)}


@app.get("/runs/{run_id}/summary")
async def get_run_summary(
    run_id: str,
    tenant_id: str = "00000000-0000-0000-0000-000000000001",
):
    """Return a human-readable evaluation summary for a run."""
    async with pool.connection() as conn:
        rows = await conn.execute(
            """SELECT category, score, passed, reasoning
               FROM evaluations
               WHERE run_id = %s AND tenant_id = %s
               ORDER BY category""",
            (run_id, tenant_id),
        )
        evals = await rows.fetchall()

    if not evals:
        raise HTTPException(status_code=404, detail="No evaluations found for this run")

    passed = [(r[0], r[1]) for r in evals if r[2]]
    failed = [(r[0], r[1]) for r in evals if not r[2]]
    avg_score = sum(r[1] for r in evals) / len(evals)

    # Identify critical failures
    critical = []
    for r in evals:
        category, score, passed_flag, reasoning = r
        if not passed_flag and category in ("permission_safety", "context_poisoning"):
            critical.append({"category": category, "reasoning": reasoning})

    return {
        "run_id": run_id,
        "total_evaluators": len(evals),
        "passed": len(passed),
        "failed": len(failed),
        "avg_score": round(avg_score, 3),
        "grade": _score_to_grade(avg_score),
        "critical_failures": critical,
        "scores": {r[0]: round(r[1], 3) for r in evals},
        "reasoning": {r[0]: r[3] for r in evals if not r[2]},
    }


# ─────────────────────────────────────────────
# Internal helpers
# ─────────────────────────────────────────────

async def _run_evaluations(
    run_data: RunData,
    categories: list[str],
    run_id: str,
    tenant_id: str,
) -> list[EvaluationResult]:
    results: list[EvaluationResult] = []
    for category in categories:
        evaluator = registry.get(category)
        if evaluator is None:
            logger.warning("No evaluator registered for category: %s", category)
            continue
        try:
            result = await evaluator.evaluate(run_data)
            results.append(result)
            await _store_evaluation(run_id, tenant_id, result)
        except Exception as exc:
            logger.error("Evaluator %s failed for run %s: %s", category, run_id, exc)
            results.append(
                EvaluationResult(
                    evaluator_name=category,
                    evaluator_version="v1",
                    category=category,
                    score=0.0,
                    passed=False,
                    details={"error": str(exc)},
                    reasoning=f"Evaluator error: {exc}",
                )
            )
    return results


async def _fetch_run_data(run_id: str, tenant_id: str) -> RunData | None:
    """Fetch complete run data from the database."""
    async with pool.connection() as conn:
        row = await conn.execute(
            """SELECT id, tenant_id, agent_id, query, final_answer, citations,
                      expected_answer, expected_tools, expected_sources,
                      context_manifest, model, prompt_tokens, completion_tokens,
                      total_tokens, latency_ms, estimated_cost, status
               FROM runs WHERE id = %s AND tenant_id = %s""",
            (run_id, tenant_id),
        )
        run_row = await row.fetchone()
        if run_row is None:
            return None

        cols = [d.name for d in row.description]
        run_dict = dict(zip(cols, run_row))

        async def fetch_table(query: str) -> list[dict]:
            r = await conn.execute(query, (run_id,))
            desc = r.description
            rows = await r.fetchall()
            return [dict(zip([d.name for d in desc], row)) for row in rows]

        retrieval = await fetch_table(
            """SELECT doc_id, chunk_id, title, source, content_preview, score, rank,
                      retrieval_method, acl_passed, acl_reason, selected, rejection_reason, doc_metadata
               FROM retrieval_candidates WHERE run_id = %s ORDER BY score DESC"""
        )
        memory = await fetch_table(
            """SELECT memory_id, memory_type, content, relevance_score, recency_score,
                      is_stale, stale_reason, selected, rejection_reason
               FROM memory_candidates WHERE run_id = %s ORDER BY relevance_score DESC"""
        )
        tools = await fetch_table(
            """SELECT tool_name, tool_args, tool_result, step_number, status,
                      error_message, latency_ms, was_correct, expected_tool, expected_args,
                      requires_approval, side_effect_type
               FROM tool_calls WHERE run_id = %s ORDER BY step_number"""
        )
        steps = await fetch_table(
            """SELECT step_number, step_type, content, metadata, latency_ms, tokens_used
               FROM reasoning_steps WHERE run_id = %s ORDER BY step_number"""
        )

        return RunData(
            run=run_dict,
            retrieval_candidates=retrieval,
            memory_candidates=memory,
            tool_calls=tools,
            reasoning_steps=steps,
        )


async def _store_evaluation(run_id: str, tenant_id: str, result: EvaluationResult) -> None:
    """Persist evaluation result. Upsert to avoid duplicates on re-evaluation."""
    import json
    async with pool.connection() as conn:
        await conn.execute(
            """INSERT INTO evaluations
                 (id, run_id, tenant_id, evaluator_name, evaluator_version,
                  category, score, passed, details, reasoning,
                  model_used, eval_tokens, eval_latency_ms)
               VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
               ON CONFLICT (run_id, category) DO UPDATE SET
                 score = EXCLUDED.score,
                 passed = EXCLUDED.passed,
                 details = EXCLUDED.details,
                 reasoning = EXCLUDED.reasoning,
                 eval_latency_ms = EXCLUDED.eval_latency_ms""",
            (
                str(uuid.uuid4()),
                run_id, tenant_id,
                result.evaluator_name, result.evaluator_version,
                result.category, result.score, result.passed,
                json.dumps(result.details) if isinstance(result.details, dict) else result.details,
                result.reasoning, result.model_used, result.eval_tokens, result.eval_latency_ms,
            ),
        )


def _score_to_grade(score: float) -> str:
    if score >= 0.90:
        return "A"
    if score >= 0.80:
        return "B"
    if score >= 0.70:
        return "C"
    if score >= 0.60:
        return "D"
    return "F"


if __name__ == "__main__":
    import uvicorn
    uvicorn.run("main:app", host="0.0.0.0", port=8081, reload=True)
