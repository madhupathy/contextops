# REST Adapter

Generic REST callback adapter for sending traces to ContextOps from any system.

## Usage

```bash
contextops adapter scaffold rest
```

Or POST traces directly:

```bash
curl -X POST http://localhost:8080/api/v1/runs \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: your-tenant-id" \
  -d @trace.json
```

See `contextops adapter scaffold rest` for a Python example.
