# Examples

- `basic-http/` — ALB2 + Frontend + two Ingress-origin Rules covering vhost, CORS, and redirect.
- `l4-tcp/` — ALB2 + TCP Frontend with `sip-hash`.

Run the regression checks with:

```bash
python3 tests/test_high_fixes.py
```

The examples use reserved `example.com` values and fictional names only.
