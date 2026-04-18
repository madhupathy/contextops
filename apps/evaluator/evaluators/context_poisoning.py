"""Context Poisoning Evaluator.

Detects context rot — when stale, conflicting, or overridden information
persists in context and contaminates the agent's answer.

Inspired by research (HydraDB manifesto, arxiv 2601.11653) showing that
transcript replay causes stale context to accumulate and produce
hallucination carryover across turns. This is the #1 multi-session failure.

Patterns detected:
1. Stale facts contradicted by newer retrieved docs
2. Prior turn assumptions overriding current retrieved evidence
3. Numeric/date value drift (earlier turn says "15 days", new doc says "20 days")
4. Self-contradictions within the answer
"""

from __future__ import annotations

import re
from typing import Any

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


class ContextPoisoningEvaluator(BaseEvaluator):
    name = "context_poisoning"
    version = "v1"
    category = "context_poisoning"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        run = run_data.run
        memory = run_data.memory_candidates
        retrieval = run_data.retrieval_candidates
        steps = run_data.reasoning_steps
        final_answer = run.get("final_answer") or ""

        if not final_answer:
            return self._result(
                score=1.0, passed=True,
                details={"reason": "no_answer"},
                reasoning="No answer to evaluate.",
                elapsed_ms=self._elapsed_ms(start),
            )

        violations: list[dict[str, Any]] = []
        warnings: list[dict[str, Any]] = []

        # ── Check 1: Stale memory content appearing in answer ──────────────
        stale_memories = [m for m in memory if m.get("is_stale") and m.get("selected")]
        for mem in stale_memories:
            content = mem.get("content", "")
            if not content:
                continue
            # Extract numbers/dates from stale memory
            stale_numbers = set(re.findall(r'\b\d+\b', content))
            answer_numbers = set(re.findall(r'\b\d+\b', final_answer))
            leaked = stale_numbers & answer_numbers
            if leaked:
                violations.append({
                    "type": "stale_memory_value_in_answer",
                    "memory_id": mem.get("memory_id"),
                    "stale_reason": mem.get("stale_reason"),
                    "leaked_values": list(leaked),
                    "severity": "high",
                })

        # ── Check 2: Contradiction between selected docs and answer ────────
        selected_docs = [d for d in retrieval if d.get("selected")]
        doc_numbers: set[str] = set()
        for doc in selected_docs:
            preview = doc.get("content_preview") or ""
            doc_numbers.update(re.findall(r'\b\d+\b', preview))

        answer_numbers = set(re.findall(r'\b\d+\b', final_answer))
        # Numbers in answer but not in any retrieved doc AND stale memory exists
        if stale_memories and doc_numbers:
            answer_only_numbers = answer_numbers - doc_numbers
            stale_numbers_all: set[str] = set()
            for mem in stale_memories:
                stale_numbers_all.update(re.findall(r'\b\d+\b', mem.get("content", "")))
            poisoned = answer_only_numbers & stale_numbers_all
            if poisoned:
                violations.append({
                    "type": "answer_matches_stale_not_current",
                    "values": list(poisoned),
                    "explanation": "Answer contains values matching stale memory but not current retrieved docs",
                    "severity": "critical",
                })

        # ── Check 3: Self-contradiction in answer ──────────────────────────
        contradictions = _detect_self_contradictions(final_answer)
        for c in contradictions:
            violations.append({
                "type": "self_contradiction",
                "details": c,
                "severity": "medium",
            })

        # ── Check 4: Prior reasoning step assumptions leaking ──────────────
        if steps:
            early_steps = steps[:2]
            late_steps = steps[-2:] if len(steps) > 2 else []
            for early in early_steps:
                early_nums = set(re.findall(r'\b\d+\b', early.get("content", "")))
                if not early_nums:
                    continue
                for late in late_steps:
                    late_content = late.get("content", "")
                    late_nums = set(re.findall(r'\b\d+\b', late_content))
                    # If early step numbers persist but docs show different numbers
                    early_leaking = early_nums & late_nums - doc_numbers
                    if early_leaking and doc_numbers:
                        warnings.append({
                            "type": "early_step_value_persistence",
                            "values": list(early_leaking),
                            "note": "Values from early reasoning steps persist in late steps without doc support",
                        })

        # ── Scoring ────────────────────────────────────────────────────────
        critical = [v for v in violations if v.get("severity") == "critical"]
        high = [v for v in violations if v.get("severity") == "high"]
        medium = [v for v in violations if v.get("severity") == "medium"]

        score = 1.0
        score -= len(critical) * 0.4
        score -= len(high) * 0.2
        score -= len(medium) * 0.1
        score -= len(warnings) * 0.05
        score = max(0.0, score)

        passed = len(critical) == 0 and len(high) == 0

        details = {
            "stale_memories_selected": len(stale_memories),
            "violations": violations,
            "warnings": warnings,
            "critical_count": len(critical),
            "high_count": len(high),
            "medium_count": len(medium),
        }

        reasoning_parts = []
        for v in critical:
            reasoning_parts.append(f"CRITICAL: {v['type']} — {v.get('explanation', v.get('values', ''))}")
        for v in high:
            reasoning_parts.append(f"HIGH: {v['type']} — memory {v.get('memory_id')} leaked values {v.get('leaked_values')}")
        for v in medium:
            reasoning_parts.append(f"MEDIUM: {v['type']}")
        if warnings:
            reasoning_parts.append(f"{len(warnings)} warning(s) about early-step value persistence")
        if not reasoning_parts:
            reasoning_parts.append("No context poisoning detected")

        return self._result(
            score=score, passed=passed, details=details,
            reasoning="; ".join(reasoning_parts),
            elapsed_ms=self._elapsed_ms(start),
        )


def _detect_self_contradictions(text: str) -> list[str]:
    """Detect obvious numeric self-contradictions within a single answer."""
    contradictions = []
    sentences = re.split(r'[.!?]\s+', text)
    num_sentences: dict[str, list[str]] = {}

    for sent in sentences:
        nums = re.findall(r'\b(\d+)\s*(days?|hours?|weeks?|months?|years?|%|percent)', sent, re.IGNORECASE)
        for num, unit in nums:
            key = unit.lower().rstrip('s')
            num_sentences.setdefault(key, []).append(f"{num} {unit} (in: '{sent[:80]}')")

    for unit, mentions in num_sentences.items():
        values = set(m.split()[0] for m in mentions)
        if len(values) > 1:
            contradictions.append(f"Conflicting {unit} values: {', '.join(mentions)}")

    return contradictions[:3]
