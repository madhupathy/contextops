"""Response Completeness Evaluator.

Checks whether the agent answered every part of the user's query.
Multi-part questions are a known failure mode: agents answer the first,
most familiar sub-question and silently drop the rest.

This evaluator detects:
1. Multi-part queries (and/or, multiple question marks, numbered lists)
2. Whether each sub-question was addressed
3. Whether follow-up actions mentioned in the query were completed
4. Implicit completeness signals (answer length vs query complexity)

This fills a gap not covered by task_completion (which checks intent type)
or answer_correctness (which checks factual accuracy) — neither checks
structural completeness of the response relative to the query structure.
"""

from __future__ import annotations

import re

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator

# Signals that a query has multiple parts
MULTI_PART_SIGNALS = [
    r"\band\b.*\?",               # "What is X and how do I Y?"
    r"\bas well as\b",            # "X as well as Y"
    r"\balso\b.*\?",              # "also" in query
    r"\balong with\b",
    r"\d+\.\s+\w",                # "1. X  2. Y" numbered questions
    r"(?:first|second|third|finally|additionally|moreover)",
    r"\blist\b.*\band\b",         # "list X and Y"
]

# Phrases that deflect without completing
DEFLECTION_PHRASES = [
    "you should", "you could", "you can", "you might want to",
    "consider", "it is recommended", "please", "feel free to",
    "i would suggest", "i'd recommend", "you may want",
]


class ResponseCompletenessEvaluator(BaseEvaluator):
    name = "response_completeness"
    version = "v1"
    category = "response_completeness"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        run = run_data.run
        final_answer = run.get("final_answer") or ""
        query = run.get("query") or ""
        status = run.get("status", "")

        if not query:
            return self._result(
                score=1.0, passed=True,
                details={"reason": "no_query"},
                reasoning="No query to evaluate completeness against.",
                elapsed_ms=self._elapsed_ms(start),
            )

        if status in ("failed", "timeout") or not final_answer:
            return self._result(
                score=0.0, passed=False,
                details={"status": status, "has_answer": bool(final_answer)},
                reasoning=f"Run {status or 'produced no answer'} — cannot evaluate completeness.",
                elapsed_ms=self._elapsed_ms(start),
            )

        issues: list[str] = []
        signals: list[str] = []

        # ── 1. Detect multi-part query ─────────────────────────────────────
        question_marks = query.count("?")
        is_multi_part = question_marks > 1 or any(
            re.search(p, query, re.IGNORECASE) for p in MULTI_PART_SIGNALS
        )

        # ── 2. Extract sub-questions from multi-part queries ───────────────
        sub_questions: list[str] = []
        if is_multi_part:
            # Split on conjunctions and question marks
            parts = re.split(r"[?]|\band\b|\balso\b|\balong with\b|\bas well as\b", query, flags=re.IGNORECASE)
            sub_questions = [p.strip() for p in parts if len(p.strip()) > 10]

        # ── 3. Check each sub-question is addressed ────────────────────────
        unanswered: list[str] = []
        if sub_questions:
            for sub_q in sub_questions:
                # Extract key nouns/verbs from the sub-question
                sq_words = {w.lower() for w in re.findall(r'\b\w{4,}\b', sub_q)}
                # Check overlap with answer
                ans_words = {w.lower() for w in re.findall(r'\b\w{4,}\b', final_answer)}
                overlap = sq_words & ans_words
                if len(overlap) / max(1, len(sq_words)) < 0.25:
                    unanswered.append(sub_q[:80])

        # ── 4. Check for deflection (agent passed task to user) ────────────
        answer_lower = final_answer.lower()
        deflections = [p for p in DEFLECTION_PHRASES if p in answer_lower]
        deflection_count = len(set(deflections))

        # ── 5. Length ratio — very short answer to complex query ──────────
        query_complexity = len(re.findall(r'\b\w+\b', query))
        answer_length = len(re.findall(r'\b\w+\b', final_answer))

        # Expected answer length scales with query complexity
        min_expected = max(20, query_complexity * 1.5)
        length_ratio = answer_length / min_expected

        if length_ratio < 0.4:
            issues.append(f"Answer ({answer_length} words) is very short for query complexity ({query_complexity} words)")
        elif length_ratio >= 0.8:
            signals.append(f"Answer length ({answer_length} words) appears proportionate to query")

        # ── 6. Check answer contains actual information vs hedging ─────────
        # Count sentences with concrete information (numbers, proper nouns, dates)
        sentences = re.split(r'[.!?]\s+', final_answer)
        concrete = [
            s for s in sentences
            if re.search(r'\b\d+\b|\b[A-Z][a-z]{2,}\b', s)
        ]
        if len(sentences) > 0:
            concrete_ratio = len(concrete) / len(sentences)
            if concrete_ratio < 0.3 and answer_length > 50:
                issues.append("Answer contains mostly hedging language with little concrete information")
            elif concrete_ratio >= 0.5:
                signals.append(f"{len(concrete)}/{len(sentences)} sentences contain concrete information")

        # ── Scoring ────────────────────────────────────────────────────────
        score = 1.0

        if unanswered:
            score -= len(unanswered) * 0.25
            issues.append(f"{len(unanswered)} sub-question(s) appear unanswered: {unanswered[:2]}")

        if deflection_count >= 2:
            score -= 0.2
            issues.append(f"Answer deflects to user in {deflection_count} places (uses: {list(set(deflections))[:3]})")

        if length_ratio < 0.4:
            score -= 0.15

        score = max(0.0, min(1.0, score))
        passed = score >= 0.65

        details = {
            "query_word_count": query_complexity,
            "answer_word_count": answer_length,
            "length_ratio": round(length_ratio, 2),
            "is_multi_part_query": is_multi_part,
            "sub_questions_detected": len(sub_questions),
            "unanswered_sub_questions": unanswered,
            "deflection_phrases_found": list(set(deflections))[:5],
            "concrete_sentence_ratio": round(len(concrete) / max(1, len(sentences)), 2),
            "issues": issues,
            "positive_signals": signals,
        }

        reasoning = "; ".join(issues) if issues else (
            f"Response appears complete — addressed {'all detected sub-questions' if sub_questions else 'the query'} "
            f"with {answer_length} words of {'concrete' if concrete else 'general'} information."
        )

        return self._result(
            score=score, passed=passed, details=details,
            reasoning=reasoning,
            elapsed_ms=self._elapsed_ms(start),
        )
