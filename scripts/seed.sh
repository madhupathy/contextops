#!/usr/bin/env bash
set -euo pipefail

API_URL="${CONTEXTOPS_API_URL:-http://localhost:8080}"
TENANT_ID="${CONTEXTOPS_TENANT:-00000000-0000-0000-0000-000000000001}"

echo "ContextOps Seed Script"
echo "API: $API_URL"
echo "Tenant: $TENANT_ID"
echo ""

# Wait for API to be ready
echo "Waiting for API..."
for i in $(seq 1 30); do
  if curl -sf "$API_URL/health" > /dev/null 2>&1; then
    echo "API is ready."
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ERROR: API not available after 30 seconds."
    exit 1
  fi
  sleep 1
done

echo ""

# Ingest example traces
EXAMPLES_DIR="$(dirname "$0")/../examples/traces"

if [ -d "$EXAMPLES_DIR" ]; then
  for f in "$EXAMPLES_DIR"/*.json; do
    [ -f "$f" ] || continue
    name=$(basename "$f")
    echo -n "Ingesting $name... "
    status=$(curl -sf -o /dev/null -w "%{http_code}" \
      -X POST "$API_URL/api/v1/runs" \
      -H "Content-Type: application/json" \
      -H "X-Tenant-ID: $TENANT_ID" \
      -d @"$f" 2>/dev/null || echo "000")

    if [ "$status" -ge 200 ] && [ "$status" -lt 300 ]; then
      echo "OK ($status)"
    else
      echo "FAIL ($status)"
    fi
  done
else
  echo "No examples directory found at $EXAMPLES_DIR"
fi

echo ""
echo "Seed complete."
echo ""
echo "Next steps:"
echo "  curl $API_URL/api/v1/runs | python3 -m json.tool"
echo "  contextops trace list"
echo "  contextops eval run <run-id>"
