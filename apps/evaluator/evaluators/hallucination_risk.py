"""Hallucination Risk Evaluator.

Scores the RISK of hallucination in the final answer by detecting
high-risk claim patterns — specific statistics, named URLs, exact quotes,
precise dates, and proper nouns not traceable to retrieved evidence.

The gap vs groundedness: groundedness checks if claims ARE grounded;
hallucination_risk detects claim patterns that ARE LIKELY to be hallucinated
even when they superficially look plausible. Research shows LLMs confidently
fabricate: exact percentages, specific URLs, precise dates from memory,
attributed quotes, and institutional policy numbers.

This is different from context_poisoning (which tracks stale-value drift)
and groundedness (which tracks keyword coverage). This evaluator scores
the structural riskiness of the answer itself.
"""

from __future__ import annotations

import re
from typing import Any

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


# Patterns that signal high hallucination risk when not supported by evidence
HIGH_RISK_PATTERNS = {
    "exact_percentage": r"\b\d+(?:\.\d+)?%",
    "exact_dollar_amount": r"\$\d[\d,]*(?:\.\d+)?(?:\s*(?:million|billion|thousand|M|B|K))?",
    "specific_url": r"https?://\S+",
    "exact_date": r"\b(?:January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2},?\s+\d{4}\b",
    "quoted_speech": r'"[^"]{10,150}"',
    "policy_number": r"\b(?:Section|Article|Clause|Rule|Policy|Act|Regulation)\s+\d+(?:\.\d+)*\b",
    "specific_count": r"\b(?:exactly|precisely|only|just)\s+\d+\b",
    "version_number": r"\bv\d+(?:\.\d+)+\b|\bversion\s+\d+(?:\.\d+)+\b",
}

CITATION_HEDGE_PATTERNS = [
    "according to", "based on", "per the", "as stated in", "the document says",
    "from the policy", "as documented", "the retrieved", "source indicates",
    "records show",
]


class HallucinationRiskEvaluator(BaseEvaluator):
    name = "hallucination_risk"
    version = "v1"
    category = "hallucination_risk"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        run = run_data.run
        retrieval = run_data.retrieval_candidates
        final_answer = run.get("final_answer") or ""

        if not final_answer:
            return self._result(
                score=1.0, passed=True,
                details={"reason": "no_answer"},
                reasoning="No answer to evaluate.",
                elapsed_ms=self._elapsed_ms(start),
            )

        # Build evidence corpus from selected documents
        selected_docs = [d for d in retrieval if d.get("selected")]
        evidence_text = " ".join(
            (d.get("content_preview") or d.get("title") or "")
            for d in selected_docs
        ).lower()

        high_risk_claims: list[dict[str, Any]] = []
        unhedged_specifics: list[dict[str, Any]] = []

        # Detect high-risk patterns
        for pattern_name, pattern_re in HIGH_RISK_PATTERNS.items():
            matches = re.findall(pattern_re, final_answer, re.IGNORECASE)
            for match in matches:
                match_lower = match.lower()
                # Check if the specific value appears in evidence
                in_evidence = match_lower in evidence_text or any(
                    word in evidence_text
                    for word in match_lower.split()
                    if len(word) > 3
                )
                # Check if claim is hedged with citation language
                # Find the sentence containing this match
                sentences = re.split(r'[.!?]\s+', final_answer)
                claim_sentence = ""
                for s in sentences:
                    if match.lower() in s.lower():
                        claim_sentence = s
                        break
                is_hedged = any(
                    hedge in claim_sentence.lower()
                    for hedge in CITATION_HEDGE_PATTERNS
                )

                risk_entry = {
                    "pattern": pattern_name,
                    "value": match[:100],
                    "in_evidence": in_evidence,
                    "is_hedged": is_hedged,
                    "sentence": claim_sentence[:120],
                }
                high_risk_claims.append(risk_entry)
                if not in_evidence and not is_hedged:
                    unhedged_specifics.append(risk_entry)

        # Confidence language amplifies hallucination risk
        confidence_amplifiers = [
            "definitely", "certainly", "always", "never", "exactly",
            "guaranteed", "100%", "absolutely", "for certain", "without doubt",
        ]
        overconfident_sentences = []
        for sent in re.split(r'[.!?]\s+', final_answer):
            if any(amp in sent.lower() for amp in confidence_amplifiers):
                overconfident_sentences.append(sent[:120])

        # Score: start at 1.0, penalise unhedged high-risk specifics
        total_claims = len(high_risk_claims)
        unhedged_count = len(unhedged_specifics)
        confident_count = len(overconfident_sentences)

        if total_claims == 0:
            # Generic, vague answer — low hallucination risk
            risk_score = 0.1
        else:
            # Fraction of high-risk claims that are unverified and unhedged
            unhedged_ratio = unhedged_count / max(1, total_claims)
            risk_score = unhedged_ratio * 0.7 + min(0.3, confident_count * 0.1)

        # Evaluator score = 1 - risk (high score = low risk)
        score = max(0.0, 1.0 - risk_score)
        passed = score >= 0.6

        details = {
            "total_high_risk_claims": total_claims,
            "unhedged_unverified_claims": unhedged_count,
            "overconfident_sentences": confident_count,
            "risk_score": round(risk_score, 3),
            "evidence_docs_count": len(selected_docs),
            "unhedged_specifics": unhedged_specifics[:5],
            "overconfident": overconfident_sentences[:3],
        }

        if unhedged_count > 0:
            reasoning = (
                f"{unhedged_count} high-risk claim(s) appear without evidence support or citation hedging: "
                + "; ".join(f"{c['pattern']} '{c['value'][:40]}'" for c in unhedged_specifics[:3])
            )
        elif confident_count > 0:
            reasoning = (
                f"Answer uses confident absolute language ({confident_count} instance(s)) "
                "which amplifies hallucination risk even when claims seem correct."
            )
        elif total_claims > 0:
            reasoning = (
                f"Answer contains {total_claims} specific claim(s), "
                "all appear supported by evidence or appropriately hedged."
            )
        else:
            reasoning = "Answer is appropriately general — low hallucination risk."

        return self._result(
            score=score, passed=passed, details=details,
            reasoning=reasoning,
            elapsed_ms=self._elapsed_ms(start),
        )
