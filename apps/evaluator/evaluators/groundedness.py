"""Groundedness evaluator.

Evaluates whether the agent's answer is grounded in the retrieved evidence.
Detects hallucinations — claims in the answer not supported by any retrieved document.
"""

from __future__ import annotations

import re

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


class GroundednessEvaluator(BaseEvaluator):
    name = "groundedness"
    version = "v1"
    category = "groundedness"

    def __init__(self, openai_api_key: str = ""):
        self._openai_api_key = openai_api_key

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        run = run_data.run
        candidates = run_data.retrieval_candidates
        final_answer = run.get("final_answer") or ""

        if not final_answer:
            return self._result(
                score=0.0, passed=False,
                details={"reason": "no_final_answer"},
                reasoning="No answer to evaluate groundedness against.",
                elapsed_ms=self._elapsed_ms(start),
            )

        selected = [c for c in candidates if c.get("selected")]

        if not selected:
            # No evidence selected — answer is entirely ungrounded
            return self._result(
                score=0.0, passed=False,
                details={"reason": "no_selected_evidence", "answer_length": len(final_answer)},
                reasoning="Answer exists but no evidence was selected. Entire answer is ungrounded.",
                elapsed_ms=self._elapsed_ms(start),
            )

        # Build evidence corpus from selected docs
        evidence_text = ""
        for doc in selected:
            preview = doc.get("content_preview") or doc.get("title") or ""
            evidence_text += " " + preview

        # Heuristic: keyword coverage
        answer_keywords = _extract_keywords(final_answer)
        evidence_keywords = _extract_keywords(evidence_text)

        if not answer_keywords:
            coverage = 1.0
        else:
            grounded_keywords = answer_keywords & evidence_keywords
            coverage = len(grounded_keywords) / len(answer_keywords)

        # Heuristic: sentence-level grounding
        answer_sentences = _split_sentences(final_answer)
        grounded_sentences = 0
        ungrounded = []

        for sent in answer_sentences:
            sent_keywords = _extract_keywords(sent)
            if not sent_keywords:
                grounded_sentences += 1
                continue
            sent_overlap = sent_keywords & evidence_keywords
            if len(sent_overlap) / len(sent_keywords) >= 0.3:
                grounded_sentences += 1
            else:
                ungrounded.append(sent[:120])

        sentence_grounding = grounded_sentences / max(1, len(answer_sentences))

        # Check for numeric claims not in evidence
        answer_numbers = set(re.findall(r'\b\d+\.?\d*\b', final_answer))
        evidence_numbers = set(re.findall(r'\b\d+\.?\d*\b', evidence_text))
        unsupported_numbers = answer_numbers - evidence_numbers
        number_grounding = 1.0
        if answer_numbers:
            number_grounding = 1.0 - (len(unsupported_numbers) / len(answer_numbers))

        # LLM-as-judge if available
        llm_score = None
        llm_reasoning = None
        model_used = None
        eval_tokens = 0
        if self._openai_api_key and len(evidence_text.strip()) > 50:
            try:
                llm_score, llm_reasoning, eval_tokens = await self._llm_groundedness(
                    final_answer, evidence_text[:3000]
                )
                model_used = "gpt-4o-mini"
            except Exception as e:
                llm_reasoning = f"LLM check failed: {e}"

        # Combined score
        if llm_score is not None:
            score = llm_score * 0.4 + coverage * 0.2 + sentence_grounding * 0.25 + number_grounding * 0.15
        else:
            score = coverage * 0.35 + sentence_grounding * 0.40 + number_grounding * 0.25

        passed = score >= 0.6

        details = {
            "evidence_docs_count": len(selected),
            "keyword_coverage": round(coverage, 3),
            "sentence_grounding": round(sentence_grounding, 3),
            "number_grounding": round(number_grounding, 3),
            "total_sentences": len(answer_sentences),
            "ungrounded_sentences": ungrounded[:5],
            "unsupported_numbers": list(unsupported_numbers)[:10],
            "llm_score": round(llm_score, 3) if llm_score is not None else None,
        }

        reasoning_parts = []
        if ungrounded:
            reasoning_parts.append(f"{len(ungrounded)} sentence(s) appear ungrounded")
        if unsupported_numbers:
            reasoning_parts.append(f"Numbers not found in evidence: {list(unsupported_numbers)[:5]}")
        if llm_reasoning:
            reasoning_parts.append(f"LLM: {llm_reasoning}")
        if not reasoning_parts:
            reasoning_parts.append("Answer appears well-grounded in retrieved evidence")

        return self._result(
            score=score, passed=passed, details=details,
            reasoning="; ".join(reasoning_parts),
            model_used=model_used, eval_tokens=eval_tokens,
            elapsed_ms=self._elapsed_ms(start),
        )

    async def _llm_groundedness(self, answer: str, evidence: str) -> tuple[float, str, int]:
        """Use LLM to check if answer is grounded in evidence."""
        import json
        import openai

        client = openai.AsyncOpenAI(api_key=self._openai_api_key)
        prompt = f"""You are a groundedness evaluator. Check if the answer is fully supported by the evidence.

Evidence:
{evidence}

Answer:
{answer}

Score from 0.0 to 1.0:
- 1.0 = every claim in the answer is supported by the evidence
- 0.5 = some claims are supported, some are not
- 0.0 = answer is entirely unsupported / hallucinated

Respond in JSON: {{"score": <float>, "reasoning": "<brief explanation>", "hallucinated_claims": [<list of unsupported claims>]}}"""

        response = await client.chat.completions.create(
            model="gpt-4o-mini",
            messages=[{"role": "user", "content": prompt}],
            temperature=0, max_tokens=300,
        )
        content = response.choices[0].message.content or ""
        tokens = response.usage.total_tokens if response.usage else 0

        try:
            match = re.search(r'\{[^}]+\}', content, re.DOTALL)
            if match:
                result = json.loads(match.group())
                return float(result.get("score", 0.5)), result.get("reasoning", ""), tokens
        except Exception:
            pass
        return 0.5, "Failed to parse LLM response", tokens


def _extract_keywords(text: str) -> set[str]:
    stop_words = {"the", "a", "an", "is", "are", "was", "were", "be", "been", "being",
                  "have", "has", "had", "do", "does", "did", "will", "would", "could",
                  "should", "may", "might", "shall", "can", "to", "of", "in", "for",
                  "on", "with", "at", "by", "from", "it", "this", "that", "and", "or",
                  "but", "not", "no", "if", "as", "so", "its", "also", "such"}
    words = re.findall(r'\w+', text.lower())
    return {w for w in words if w not in stop_words and len(w) > 2}


def _split_sentences(text: str) -> list[str]:
    sentences = re.split(r'[.!?]\s+', text)
    return [s.strip() for s in sentences if s.strip() and len(s.strip()) > 10]
