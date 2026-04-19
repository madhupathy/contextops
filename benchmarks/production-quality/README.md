# Production Quality Benchmark

Tests that catch issues only visible at production scale:
- Hallucinated statistics and pricing in sales/support contexts
- Multi-part queries where agents answer partially
- Response completeness for complex enterprise questions
- Performance regression vs recorded baselines

## Cases
- `hallucinated-pricing.json` — agent fabricates specific pricing not in docs
- `multi-part-incomplete.json` — agent answers first sub-question, ignores second
- `completeness-complex-policy.json` — complex policy question needs full coverage
