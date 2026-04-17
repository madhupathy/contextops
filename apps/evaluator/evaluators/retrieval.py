"""Retrieval quality evaluator.

Evaluates whether the agent retrieved the right documents, ranked them correctly,
and didn't miss better sources. This is the core differentiator vs RAGAS —
we evaluate the full retrieval pipeline, not just final answer quality.
"""

from __future__ import annotations

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


class RetrievalQualityEvaluator(BaseEvaluator):
    name = "retrieval_quality"
    version = "v1"
    category = "retrieval_quality"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        candidates = run_data.retrieval_candidates
        run = run_data.run

        if not candidates:
            return self._result(
                score=0.0,
                passed=False,
                details={"reason": "no_retrieval_candidates"},
                reasoning="No retrieval candidates found in run data.",
                elapsed_ms=self._elapsed_ms(start),
            )

        selected = [c for c in candidates if c.get("selected")]
        rejected = [c for c in candidates if not c.get("selected")]
        acl_blocked = [c for c in candidates if not c.get("acl_passed", True)]

        expected_sources = run.get("expected_sources") or []
        if isinstance(expected_sources, str):
            import json
            try:
                expected_sources = json.loads(expected_sources)
            except Exception:
                expected_sources = []

        # Metrics
        total = len(candidates)
        selected_count = len(selected)
        acl_blocked_count = len(acl_blocked)

        # Recall: did we select the expected sources?
        recall = 1.0
        missed_sources = []
        if expected_sources:
            expected_ids = {s.get("doc_id") or s for s in expected_sources} if isinstance(expected_sources, list) else set()
            selected_ids = {c.get("doc_id") for c in selected}
            if expected_ids:
                found = expected_ids & selected_ids
                recall = len(found) / len(expected_ids) if expected_ids else 1.0
                missed_sources = list(expected_ids - selected_ids)

        # Ranking quality: are higher-scored docs selected?
        ranking_score = 1.0
        if selected and rejected:
            min_selected_score = min(c.get("score", 0) for c in selected)
            better_rejected = [c for c in rejected if c.get("score", 0) > min_selected_score and c.get("acl_passed", True)]
            if better_rejected:
                ranking_score = max(0.0, 1.0 - (len(better_rejected) / total))

        # Diversity: are we pulling from multiple sources?
        sources = set(c.get("source", "unknown") for c in selected)
        diversity_score = min(1.0, len(sources) / max(1, min(3, selected_count)))

        # Combined score
        score = (recall * 0.4) + (ranking_score * 0.35) + (diversity_score * 0.25)
        passed = score >= 0.6

        details = {
            "total_candidates": total,
            "selected_count": selected_count,
            "acl_blocked_count": acl_blocked_count,
            "recall": round(recall, 3),
            "ranking_score": round(ranking_score, 3),
            "diversity_score": round(diversity_score, 3),
            "missed_sources": missed_sources,
            "unique_sources": list(sources),
            "better_rejected": [
                {"doc_id": c.get("doc_id"), "score": c.get("score"), "title": c.get("title")}
                for c in rejected
                if c.get("score", 0) > (min(c2.get("score", 0) for c2 in selected) if selected else 0)
                and c.get("acl_passed", True)
            ][:5],
        }

        reasoning_parts = []
        if missed_sources:
            reasoning_parts.append(f"Missed expected sources: {missed_sources}")
        if acl_blocked_count:
            reasoning_parts.append(f"{acl_blocked_count} candidates blocked by ACL")
        if ranking_score < 1.0:
            reasoning_parts.append("Some higher-scored candidates were rejected despite passing ACL")
        if not reasoning_parts:
            reasoning_parts.append("Retrieval quality looks good")

        return self._result(
            score=score,
            passed=passed,
            details=details,
            reasoning="; ".join(reasoning_parts),
            elapsed_ms=self._elapsed_ms(start),
        )
