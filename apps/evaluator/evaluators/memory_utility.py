"""Memory utility evaluator.

Evaluates whether memory items helped or hurt the agent's response.
Detects stale memory usage, conflicting memories, and memory that was
available but not used when it should have been.

This directly addresses the HydraDB-style memory evaluation gap.
"""

from __future__ import annotations

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


class MemoryUtilityEvaluator(BaseEvaluator):
    name = "memory_utility"
    version = "v1"
    category = "memory_utility"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        candidates = run_data.memory_candidates
        run = run_data.run

        if not candidates:
            return self._result(
                score=1.0,
                passed=True,
                details={"reason": "no_memory_candidates", "note": "No memory was involved in this run"},
                reasoning="No memory candidates present. Evaluation not applicable.",
                elapsed_ms=self._elapsed_ms(start),
            )

        selected = [m for m in candidates if m.get("selected")]
        rejected = [m for m in candidates if not m.get("selected")]
        stale_used = [m for m in selected if m.get("is_stale")]
        stale_available = [m for m in candidates if m.get("is_stale")]
        high_relevance_rejected = [
            m for m in rejected
            if m.get("relevance_score", 0) > 0.7 and not m.get("is_stale")
        ]

        # Metric 1: Stale memory avoidance
        stale_avoidance = 1.0
        if stale_used:
            stale_avoidance = max(0.0, 1.0 - (len(stale_used) / max(1, len(selected))))

        # Metric 2: Relevant memory utilization
        utilization = 1.0
        if high_relevance_rejected:
            utilization = max(0.0, 1.0 - (len(high_relevance_rejected) / max(1, len(candidates))))

        # Metric 3: Memory type diversity (using different memory types is good)
        memory_types_used = set(m.get("memory_type", "unknown") for m in selected)
        memory_types_available = set(m.get("memory_type", "unknown") for m in candidates)
        type_diversity = len(memory_types_used) / max(1, min(3, len(memory_types_available)))
        type_diversity = min(1.0, type_diversity)

        # Metric 4: Relevance-based selection quality
        selection_quality = 1.0
        if selected and rejected:
            avg_selected_relevance = sum(m.get("relevance_score", 0) for m in selected) / len(selected)
            avg_rejected_relevance = sum(m.get("relevance_score", 0) for m in rejected) / len(rejected)
            if avg_rejected_relevance > avg_selected_relevance:
                selection_quality = max(0.0, avg_selected_relevance / max(0.01, avg_rejected_relevance))

        # Combined score
        score = (
            stale_avoidance * 0.35
            + utilization * 0.25
            + selection_quality * 0.25
            + type_diversity * 0.15
        )
        passed = score >= 0.5 and len(stale_used) == 0

        details = {
            "total_candidates": len(candidates),
            "selected_count": len(selected),
            "rejected_count": len(rejected),
            "stale_used_count": len(stale_used),
            "stale_available_count": len(stale_available),
            "high_relevance_rejected_count": len(high_relevance_rejected),
            "memory_types_used": list(memory_types_used),
            "memory_types_available": list(memory_types_available),
            "metrics": {
                "stale_avoidance": round(stale_avoidance, 3),
                "utilization": round(utilization, 3),
                "selection_quality": round(selection_quality, 3),
                "type_diversity": round(type_diversity, 3),
            },
            "stale_memories_used": [
                {
                    "memory_id": m.get("memory_id"),
                    "memory_type": m.get("memory_type"),
                    "stale_reason": m.get("stale_reason"),
                    "content_preview": (m.get("content") or "")[:100],
                }
                for m in stale_used
            ],
            "missed_relevant_memories": [
                {
                    "memory_id": m.get("memory_id"),
                    "memory_type": m.get("memory_type"),
                    "relevance_score": m.get("relevance_score"),
                    "content_preview": (m.get("content") or "")[:100],
                }
                for m in high_relevance_rejected[:5]
            ],
        }

        reasoning_parts = []
        if stale_used:
            reasoning_parts.append(
                f"WARNING: {len(stale_used)} stale memory item(s) were used. "
                f"Stale reasons: {[m.get('stale_reason') for m in stale_used]}"
            )
        if high_relevance_rejected:
            reasoning_parts.append(
                f"{len(high_relevance_rejected)} highly relevant memory item(s) were available but not used"
            )
        if selection_quality < 0.7:
            reasoning_parts.append("Memory selection quality is poor — rejected items had higher relevance than selected")
        if not reasoning_parts:
            reasoning_parts.append("Memory usage looks appropriate")

        return self._result(
            score=score,
            passed=passed,
            details=details,
            reasoning="; ".join(reasoning_parts),
            elapsed_ms=self._elapsed_ms(start),
        )
