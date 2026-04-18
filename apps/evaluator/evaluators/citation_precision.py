"""Citation Precision Evaluator.

Checks whether every citation in the answer actually supports the claim
it is cited for. This is a known gap in most eval tools — they check if
citations exist but not if they're *correct*.

Glean and similar enterprise search tools emphasise "referenceability" and
citation accuracy as a trust baseline. This evaluator enforces it.
"""

from __future__ import annotations

import json
import re

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


class CitationPrecisionEvaluator(BaseEvaluator):
    name = "citation_precision"
    version = "v1"
    category = "citation_precision"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        run = run_data.run
        retrieval = run_data.retrieval_candidates
        final_answer = run.get("final_answer") or ""

        # Parse citations from run
        citations_raw = run.get("citations") or []
        if isinstance(citations_raw, str):
            try:
                citations_raw = json.loads(citations_raw)
            except Exception:
                citations_raw = []

        # Normalise citation list (can be strings or dicts)
        cited_ids: list[str] = []
        for c in citations_raw:
            if isinstance(c, dict):
                cited_ids.append(c.get("doc_id") or c.get("id") or "")
            elif isinstance(c, str):
                cited_ids.append(c)
        cited_ids = [c for c in cited_ids if c]

        if not cited_ids:
            # No citations — check if answer makes factual claims
            factual_claims = _count_factual_claims(final_answer)
            if factual_claims > 3:
                return self._result(
                    score=0.4, passed=False,
                    details={
                        "cited_count": 0,
                        "factual_claims_estimated": factual_claims,
                        "issue": "answer_has_claims_but_no_citations",
                    },
                    reasoning=f"Answer makes ~{factual_claims} factual claims but provides no citations.",
                    elapsed_ms=self._elapsed_ms(start),
                )
            return self._result(
                score=1.0, passed=True,
                details={"cited_count": 0, "note": "No citations expected for this answer type"},
                reasoning="No citations present; answer type does not require them.",
                elapsed_ms=self._elapsed_ms(start),
            )

        # Build a lookup of doc_id → content_preview from retrieval candidates
        doc_lookup: dict[str, dict] = {}
        for doc in retrieval:
            did = doc.get("doc_id")
            if did:
                doc_lookup[did] = doc

        # Check each citation
        valid_citations: list[str] = []
        invalid_citations: list[dict] = []
        hallucinated_citations: list[str] = []

        for cid in cited_ids:
            if cid not in doc_lookup:
                hallucinated_citations.append(cid)
                continue

            doc = doc_lookup[cid]

            # Was this doc actually retrieved and selected?
            if not doc.get("selected"):
                invalid_citations.append({
                    "doc_id": cid,
                    "reason": "cited_but_not_selected",
                    "acl_passed": doc.get("acl_passed"),
                })
                continue

            # Was it ACL-blocked?
            if not doc.get("acl_passed", True):
                invalid_citations.append({
                    "doc_id": cid,
                    "reason": "cited_acl_blocked_doc",
                    "acl_reason": doc.get("acl_reason"),
                    "severity": "critical",
                })
                continue

            # Content support check: does answer share keywords with doc?
            preview = doc.get("content_preview") or doc.get("title") or ""
            if preview and final_answer:
                support_score = _content_support(final_answer, preview)
                if support_score < 0.15:
                    invalid_citations.append({
                        "doc_id": cid,
                        "reason": "low_content_support",
                        "support_score": round(support_score, 3),
                        "doc_title": doc.get("title"),
                    })
                    continue

            valid_citations.append(cid)

        # Scoring
        total = len(cited_ids)
        critical_violations = [c for c in invalid_citations if c.get("severity") == "critical"]

        if total == 0:
            precision = 1.0
        else:
            precision = len(valid_citations) / total

        hallucination_penalty = len(hallucinated_citations) * 0.3
        critical_penalty = len(critical_violations) * 0.4

        score = max(0.0, precision - hallucination_penalty - critical_penalty)
        passed = score >= 0.7 and len(critical_violations) == 0 and len(hallucinated_citations) == 0

        details = {
            "total_citations": total,
            "valid_citations": len(valid_citations),
            "invalid_citations": invalid_citations,
            "hallucinated_citations": hallucinated_citations,
            "precision": round(precision, 3),
        }

        reasoning_parts = []
        if hallucinated_citations:
            reasoning_parts.append(f"CRITICAL: {len(hallucinated_citations)} hallucinated citation(s) — doc IDs not in retrieved set")
        if critical_violations:
            reasoning_parts.append(f"CRITICAL: {len(critical_violations)} ACL-blocked doc(s) cited")
        if invalid_citations and not critical_violations and not hallucinated_citations:
            reasoning_parts.append(f"{len(invalid_citations)} citation(s) have weak content support")
        if valid_citations:
            reasoning_parts.append(f"{len(valid_citations)}/{total} citation(s) verified")
        if not reasoning_parts:
            reasoning_parts.append("All citations verified against retrieved documents")

        return self._result(
            score=score, passed=passed, details=details,
            reasoning="; ".join(reasoning_parts),
            elapsed_ms=self._elapsed_ms(start),
        )


def _count_factual_claims(text: str) -> int:
    """Rough heuristic for how many factual claims an answer makes."""
    # Count sentences with numbers, proper nouns, or policy-like language
    sentences = re.split(r'[.!?]\s+', text)
    factual = 0
    for s in sentences:
        if re.search(r'\b\d+\b', s) or re.search(r'\b[A-Z][a-z]+\s+[A-Z][a-z]+\b', s):
            factual += 1
    return factual


def _content_support(answer: str, doc_preview: str) -> float:
    """Fraction of doc keywords found in answer."""
    stop = {"the", "a", "an", "is", "are", "was", "were", "in", "of", "to", "for", "and", "or", "it", "this"}
    doc_words = {w.lower() for w in re.findall(r'\w+', doc_preview) if w.lower() not in stop and len(w) > 3}
    ans_words = {w.lower() for w in re.findall(r'\w+', answer)}
    if not doc_words:
        return 1.0
    return len(doc_words & ans_words) / len(doc_words)
