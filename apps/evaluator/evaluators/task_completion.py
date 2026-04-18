"""Task Completion Evaluator.

Evaluates whether the agent actually completed the user's intent,
not just whether the final answer looks correct.

This distinction matters at enterprise scale:
- A correct fact delivered without action = incomplete (if action was needed)
- An answer that addresses a different intent = failed completion
- A multi-step task with steps left undone = partial completion

Inspired by Galileo's "Session Success Tracking" and the broader finding
(KPMG survey, 65% of C-suite cite agentic complexity as blocker) that
task completion — not answer quality — is the real production gate.
"""

from __future__ import annotations

import re

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator

# Intent signals mapped to expected completion indicators
INTENT_PATTERNS = {
    "lookup": ["what is", "what are", "who is", "define", "tell me about", "what does"],
    "action": ["create", "update", "delete", "send", "schedule", "book", "submit", "file", "set up"],
    "comparison": ["compare", "difference between", "vs", "which is better", "pros and cons"],
    "list": ["list all", "show me all", "give me a list", "enumerate", "what are the"],
    "summary": ["summarize", "summarise", "tldr", "brief", "overview of"],
    "troubleshoot": ["fix", "debug", "why is", "not working", "error", "failed", "issue with"],
    "calculate": ["how many", "how much", "calculate", "compute", "total", "count"],
}

ACTION_COMPLETION_SIGNALS = [
    "has been", "have been", "was created", "was updated", "was sent",
    "successfully", "completed", "done", "scheduled", "confirmed",
    "created", "updated", "deleted",
]

ACTION_FAILURE_SIGNALS = [
    "i cannot", "i'm unable", "i don't have", "i can't", "not authorized",
    "you'll need to", "please", "you should", "you can",
]


class TaskCompletionEvaluator(BaseEvaluator):
    name = "task_completion"
    version = "v1"
    category = "task_completion"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        run = run_data.run
        tool_calls = run_data.tool_calls
        final_answer = run.get("final_answer") or ""
        query = run.get("query") or ""
        status = run.get("status", "")

        if not query:
            return self._result(
                score=1.0, passed=True,
                details={"reason": "no_query"},
                reasoning="No query to evaluate completion against.",
                elapsed_ms=self._elapsed_ms(start),
            )

        # Detect intent type
        intent = _detect_intent(query)

        # Hard fail states
        if status in ("failed", "timeout"):
            return self._result(
                score=0.0 if status == "failed" else 0.1,
                passed=False,
                details={"intent": intent, "status": status, "query": query[:200]},
                reasoning=f"Run {status} — task not completed.",
                elapsed_ms=self._elapsed_ms(start),
            )

        if not final_answer:
            return self._result(
                score=0.0, passed=False,
                details={"intent": intent, "issue": "no_final_answer"},
                reasoning="No final answer produced.",
                elapsed_ms=self._elapsed_ms(start),
            )

        scores: dict[str, float] = {}
        issues: list[str] = []

        # ── Intent-specific completion checks ──────────────────────────────
        if intent == "action":
            # Action tasks need tool calls AND completion language
            if not tool_calls:
                scores["action_tool_calls"] = 0.3
                issues.append("Action intent detected but no tool calls made — task likely not executed")
            else:
                successful_tools = [t for t in tool_calls if t.get("status") == "success"]
                scores["action_tool_calls"] = len(successful_tools) / len(tool_calls)

            has_completion_language = any(
                sig in final_answer.lower() for sig in ACTION_COMPLETION_SIGNALS
            )
            has_failure_language = any(
                sig in final_answer.lower() for sig in ACTION_FAILURE_SIGNALS
            )

            if has_failure_language and not has_completion_language:
                scores["action_completed"] = 0.2
                issues.append("Answer deflects action to user — agent failed to complete the task")
            elif has_completion_language:
                scores["action_completed"] = 1.0
            else:
                scores["action_completed"] = 0.6

        elif intent == "lookup":
            # Lookup tasks need an answer of reasonable length
            word_count = len(final_answer.split())
            if word_count < 10:
                scores["lookup_depth"] = 0.3
                issues.append(f"Lookup answer is very short ({word_count} words)")
            elif word_count > 20:
                scores["lookup_depth"] = 1.0
            else:
                scores["lookup_depth"] = 0.7

            # Answer should contain relevant terms from query
            query_keywords = {w.lower() for w in re.findall(r'\w+', query) if len(w) > 3}
            answer_keywords = {w.lower() for w in re.findall(r'\w+', final_answer)}
            overlap = query_keywords & answer_keywords
            if query_keywords:
                scores["lookup_relevance"] = min(1.0, len(overlap) / len(query_keywords))
            else:
                scores["lookup_relevance"] = 1.0

        elif intent == "list":
            has_list = (
                "\n-" in final_answer
                or "\n•" in final_answer
                or "\n*" in final_answer
                or bool(re.search(r'\n\d+\.', final_answer))
            )
            scores["list_format"] = 1.0 if has_list else 0.4
            if not has_list:
                issues.append("List intent but no list structure in answer")

        elif intent == "comparison":
            has_comparison = (
                "vs" in final_answer.lower()
                or "compared" in final_answer.lower()
                or "whereas" in final_answer.lower()
                or "while" in final_answer.lower()
                or re.search(r'\b(better|worse|higher|lower|more|less)\b', final_answer, re.IGNORECASE) is not None
            )
            scores["comparison_content"] = 1.0 if has_comparison else 0.4
            if not has_comparison:
                issues.append("Comparison intent but no comparative language in answer")

        elif intent == "calculate":
            has_number = bool(re.search(r'\b\d+\b', final_answer))
            scores["calculate_result"] = 1.0 if has_number else 0.2
            if not has_number:
                issues.append("Calculation intent but answer contains no numeric result")

        else:
            # Generic: basic completion
            word_count = len(final_answer.split())
            scores["generic_completion"] = min(1.0, word_count / 30)

        # ── Expected tools check ───────────────────────────────────────────
        expected_tools_raw = run.get("expected_tools")
        if expected_tools_raw:
            import json
            try:
                expected = json.loads(expected_tools_raw) if isinstance(expected_tools_raw, str) else expected_tools_raw
                if isinstance(expected, list) and expected:
                    actual_names = {t.get("tool_name") for t in tool_calls}
                    expected_names = {e.get("tool_name") if isinstance(e, dict) else e for e in expected}
                    coverage = len(expected_names & actual_names) / len(expected_names)
                    scores["expected_tool_coverage"] = coverage
                    if coverage < 1.0:
                        missing = expected_names - actual_names
                        issues.append(f"Expected tools not called: {list(missing)}")
            except Exception:
                pass

        # ── Final score ────────────────────────────────────────────────────
        if scores:
            score = sum(scores.values()) / len(scores)
        else:
            score = 0.7
        passed = score >= 0.6 and len([i for i in issues if "failed" in i.lower() or "not executed" in i.lower()]) == 0

        details = {
            "intent": intent,
            "query": query[:200],
            "answer_word_count": len(final_answer.split()),
            "tool_calls_made": len(tool_calls),
            "sub_scores": {k: round(v, 3) for k, v in scores.items()},
            "issues": issues,
        }

        reasoning_parts = issues or [f"Task ({intent}) appears completed (score: {score:.2f})"]
        return self._result(
            score=score, passed=passed, details=details,
            reasoning="; ".join(reasoning_parts),
            elapsed_ms=self._elapsed_ms(start),
        )


def _detect_intent(query: str) -> str:
    q = query.lower()
    for intent, patterns in INTENT_PATTERNS.items():
        if any(p in q for p in patterns):
            return intent
    return "lookup"
