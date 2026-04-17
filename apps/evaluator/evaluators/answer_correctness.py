"""Answer correctness evaluator.

Evaluates whether the final answer is correct compared to expected answer.
Uses both exact matching heuristics and semantic similarity.
When an LLM API key is available, can also use LLM-as-judge.
"""

from __future__ import annotations

import json
import re

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


class AnswerCorrectnessEvaluator(BaseEvaluator):
    name = "answer_correctness"
    version = "v1"
    category = "answer_correctness"

    def __init__(self, openai_api_key: str = ""):
        self._openai_api_key = openai_api_key

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        run = run_data.run
        final_answer = run.get("final_answer") or ""
        expected_answer = run.get("expected_answer") or ""

        if not final_answer:
            return self._result(
                score=0.0,
                passed=False,
                details={"reason": "no_final_answer"},
                reasoning="Agent produced no final answer.",
                elapsed_ms=self._elapsed_ms(start),
            )

        if not expected_answer:
            # No ground truth — can only check basic quality signals
            return self._evaluate_without_ground_truth(run_data, start)

        # Heuristic scoring
        exact_match = final_answer.strip().lower() == expected_answer.strip().lower()
        containment = _containment_score(final_answer, expected_answer)
        keyword_overlap = _keyword_overlap(final_answer, expected_answer)
        length_ratio = min(len(final_answer), len(expected_answer)) / max(len(final_answer), len(expected_answer), 1)

        # LLM-as-judge if API key available
        llm_score = None
        llm_reasoning = None
        model_used = None
        eval_tokens = 0
        if self._openai_api_key:
            try:
                llm_score, llm_reasoning, eval_tokens = await self._llm_judge(
                    run.get("query", ""), expected_answer, final_answer
                )
                model_used = "gpt-4o-mini"
            except Exception as e:
                llm_reasoning = f"LLM judge failed: {e}"

        # Combined score
        if llm_score is not None:
            score = llm_score * 0.5 + containment * 0.2 + keyword_overlap * 0.2 + (1.0 if exact_match else 0.0) * 0.1
        else:
            score = containment * 0.35 + keyword_overlap * 0.35 + (1.0 if exact_match else 0.0) * 0.15 + length_ratio * 0.15

        passed = score >= 0.6

        details = {
            "exact_match": exact_match,
            "containment_score": round(containment, 3),
            "keyword_overlap": round(keyword_overlap, 3),
            "length_ratio": round(length_ratio, 3),
            "llm_score": round(llm_score, 3) if llm_score is not None else None,
            "answer_length": len(final_answer),
            "expected_length": len(expected_answer),
        }

        reasoning_parts = []
        if exact_match:
            reasoning_parts.append("Exact match with expected answer")
        elif containment > 0.8:
            reasoning_parts.append("Answer contains most of the expected content")
        elif keyword_overlap > 0.6:
            reasoning_parts.append("Good keyword overlap with expected answer")
        else:
            reasoning_parts.append("Answer diverges significantly from expected")
        if llm_reasoning:
            reasoning_parts.append(f"LLM judge: {llm_reasoning}")

        return self._result(
            score=score,
            passed=passed,
            details=details,
            reasoning="; ".join(reasoning_parts),
            model_used=model_used,
            eval_tokens=eval_tokens,
            elapsed_ms=self._elapsed_ms(start),
        )

    async def _evaluate_without_ground_truth(self, run_data: RunData, start: float) -> EvaluationResult:
        """When no expected answer exists, check basic quality signals."""
        run = run_data.run
        answer = run.get("final_answer") or ""
        status = run.get("status", "")

        quality_score = 0.5  # neutral baseline
        issues = []

        if status == "failed":
            quality_score = 0.0
            issues.append("Run status is 'failed'")
        elif status == "timeout":
            quality_score = 0.1
            issues.append("Run timed out")

        if len(answer) < 10:
            quality_score *= 0.5
            issues.append("Answer is very short")

        # Check if answer is a refusal/error
        refusal_patterns = ["i cannot", "i'm unable", "i don't have", "error:", "exception:"]
        if any(p in answer.lower() for p in refusal_patterns):
            quality_score *= 0.7
            issues.append("Answer appears to be a refusal or error")

        return self._result(
            score=quality_score,
            passed=quality_score >= 0.4,
            details={
                "has_ground_truth": False,
                "quality_score": round(quality_score, 3),
                "issues": issues,
            },
            reasoning="No expected answer provided. Basic quality check only. " + "; ".join(issues) if issues else "No expected answer provided.",
            elapsed_ms=self._elapsed_ms(start),
        )

    async def _llm_judge(self, query: str, expected: str, actual: str) -> tuple[float, str, int]:
        """Use LLM as judge for answer correctness."""
        import openai

        client = openai.AsyncOpenAI(api_key=self._openai_api_key)
        prompt = f"""You are an evaluation judge. Score how correct the actual answer is compared to the expected answer.

Query: {query}

Expected Answer: {expected}

Actual Answer: {actual}

Score from 0.0 to 1.0 where:
- 1.0 = perfectly correct, covers all key points
- 0.7 = mostly correct, minor omissions
- 0.5 = partially correct
- 0.3 = mostly wrong but has some relevant info
- 0.0 = completely wrong or irrelevant

Respond in JSON format: {{"score": <float>, "reasoning": "<brief explanation>"}}"""

        response = await client.chat.completions.create(
            model="gpt-4o-mini",
            messages=[{"role": "user", "content": prompt}],
            temperature=0,
            max_tokens=200,
        )

        content = response.choices[0].message.content or ""
        tokens = (response.usage.total_tokens if response.usage else 0)

        try:
            # Extract JSON from response
            match = re.search(r'\{[^}]+\}', content)
            if match:
                result = json.loads(match.group())
                return float(result.get("score", 0.5)), result.get("reasoning", ""), tokens
        except Exception:
            pass

        return 0.5, "Failed to parse LLM judge response", tokens


def _containment_score(answer: str, expected: str) -> float:
    """How much of the expected answer is contained in the actual answer."""
    if not expected:
        return 1.0
    expected_words = set(expected.lower().split())
    answer_words = set(answer.lower().split())
    if not expected_words:
        return 1.0
    overlap = expected_words & answer_words
    return len(overlap) / len(expected_words)


def _keyword_overlap(answer: str, expected: str) -> float:
    """Overlap of significant keywords between answer and expected."""
    stop_words = {"the", "a", "an", "is", "are", "was", "were", "be", "been", "being",
                  "have", "has", "had", "do", "does", "did", "will", "would", "could",
                  "should", "may", "might", "shall", "can", "to", "of", "in", "for",
                  "on", "with", "at", "by", "from", "it", "this", "that", "and", "or",
                  "but", "not", "no", "if", "as", "so"}

    def keywords(text: str) -> set[str]:
        words = re.findall(r'\w+', text.lower())
        return {w for w in words if w not in stop_words and len(w) > 2}

    kw_answer = keywords(answer)
    kw_expected = keywords(expected)
    if not kw_expected:
        return 1.0
    overlap = kw_answer & kw_expected
    return len(overlap) / len(kw_expected)
