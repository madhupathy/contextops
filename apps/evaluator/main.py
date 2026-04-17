"""ContextOps Evaluator Service - FastAPI application."""

import os
import logging
from contextlib import asynccontextmanager

import psycopg_pool
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware

from evaluators.registry import EvaluatorRegistry
from models import (
    EvaluateRequest,
    EvaluateResponse,
    EvaluationResult,
    HealthResponse,
    RunData,
)

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
logger = logging.getLogger("evaluator")

DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql://contextops:contextops@localhost:5432/contextops",
)
REDIS_URL = os.getenv("REDIS_URL", "redis://localhost:6379/0")
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY", "")

pool: psycopg_pool.AsyncConnectionPool | None = None
registry: EvaluatorRegistry | None = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global pool, registry
    pool = psycopg_pool.AsyncConnectionPool(conninfo=DATABASE_URL, min_size=2, max_size=10)
    await pool.open()
    registry = EvaluatorRegistry(openai_api_key=OPENAI_API_KEY)
    logger.info("Evaluator service started")
    yield
    await pool.close()
    logger.info("Evaluator service stopped")


app = FastAPI(
    title="ContextOps Evaluator",
    description="Pluggable evaluation engine for enterprise AI agent runs",
    version="0.1.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/health", response_model=HealthResponse)
async def health():
    return HealthResponse(status="ok", service="contextops-evaluator", version="0.1.0")


@app.get("/evaluators")
async def list_evaluators():
    """List all available evaluators."""
    return {"evaluators": registry.list_evaluators()}


@app.post("/evaluate", response_model=EvaluateResponse)
async def evaluate_run(request: EvaluateRequest):
    """Evaluate a run across specified categories."""
    run_data = await _fetch_run_data(request.run_id, request.tenant_id)
    if run_data is None:
        raise HTTPException(status_code=404, detail="Run not found")

    results: list[EvaluationResult] = []
    categories = request.categories or registry.default_categories()

    for category in categories:
        evaluator = registry.get(category)
        if evaluator is None:
            logger.warning("No evaluator for category: %s", category)
            continue
        try:
            result = await evaluator.evaluate(run_data)
            results.append(result)
            await _store_evaluation(request.run_id, request.tenant_id, result)
        except Exception as e:
            logger.error("Evaluation failed for %s: %s", category, e)
            results.append(
                EvaluationResult(
                    evaluator_name=category,
                    evaluator_version="v1",
                    category=category,
                    score=0.0,
                    passed=False,
                    details={"error": str(e)},
                    reasoning=f"Evaluation failed: {e}",
                )
            )

    return EvaluateResponse(run_id=request.run_id, results=results)


@app.post("/evaluate/batch")
async def evaluate_batch(request: dict):
    """Evaluate multiple runs."""
    run_ids = request.get("run_ids", [])
    tenant_id = request.get("tenant_id", "00000000-0000-0000-0000-000000000001")
    categories = request.get("categories")

    results = {}
    for run_id in run_ids:
        try:
            resp = await evaluate_run(
                EvaluateRequest(run_id=run_id, tenant_id=tenant_id, categories=categories)
            )
            results[run_id] = resp.results
        except Exception as e:
            results[run_id] = [{"error": str(e)}]

    return {"results": results}


async def _fetch_run_data(run_id: str, tenant_id: str) -> RunData | None:
    """Fetch complete run data from database."""
    async with pool.connection() as conn:
        # Fetch run
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

        # Fetch retrieval candidates
        rc_rows = await conn.execute(
            """SELECT doc_id, chunk_id, title, source, content_preview, score, rank,
                      retrieval_method, acl_passed, acl_reason, selected, rejection_reason, doc_metadata
               FROM retrieval_candidates WHERE run_id = %s ORDER BY score DESC""",
            (run_id,),
        )
        retrieval = [dict(zip([d.name for d in rc_rows.description], r)) for r in await rc_rows.fetchall()]

        # Fetch memory candidates
        mc_rows = await conn.execute(
            """SELECT memory_id, memory_type, content, relevance_score, recency_score,
                      is_stale, stale_reason, selected, rejection_reason
               FROM memory_candidates WHERE run_id = %s ORDER BY relevance_score DESC""",
            (run_id,),
        )
        memory = [dict(zip([d.name for d in mc_rows.description], r)) for r in await mc_rows.fetchall()]

        # Fetch tool calls
        tc_rows = await conn.execute(
            """SELECT tool_name, tool_args, tool_result, step_number, status,
                      error_message, latency_ms, was_correct, expected_tool, expected_args,
                      requires_approval, side_effect_type
               FROM tool_calls WHERE run_id = %s ORDER BY step_number""",
            (run_id,),
        )
        tools = [dict(zip([d.name for d in tc_rows.description], r)) for r in await tc_rows.fetchall()]

        # Fetch reasoning steps
        rs_rows = await conn.execute(
            """SELECT step_number, step_type, content, metadata, latency_ms, tokens_used
               FROM reasoning_steps WHERE run_id = %s ORDER BY step_number""",
            (run_id,),
        )
        steps = [dict(zip([d.name for d in rs_rows.description], r)) for r in await rs_rows.fetchall()]

        cols = [d.name for d in row.description]
        run_dict = dict(zip(cols, run_row))

        return RunData(
            run=run_dict,
            retrieval_candidates=retrieval,
            memory_candidates=memory,
            tool_calls=tools,
            reasoning_steps=steps,
        )


async def _store_evaluation(run_id: str, tenant_id: str, result: EvaluationResult):
    """Store evaluation result in database."""
    import json
    import uuid

    async with pool.connection() as conn:
        await conn.execute(
            """INSERT INTO evaluations (id, run_id, tenant_id, evaluator_name, evaluator_version,
                      category, score, passed, details, reasoning, model_used, eval_tokens, eval_latency_ms)
               VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)""",
            (
                str(uuid.uuid4()),
                run_id,
                tenant_id,
                result.evaluator_name,
                result.evaluator_version,
                result.category,
                result.score,
                result.passed,
                json.dumps(result.details) if isinstance(result.details, dict) else result.details,
                result.reasoning,
                result.model_used,
                result.eval_tokens,
                result.eval_latency_ms,
            ),
        )


if __name__ == "__main__":
    import uvicorn

    uvicorn.run("main:app", host="0.0.0.0", port=8081, reload=True)
