# Backlog

## BL-001: Serve fake data from `/api/edu`
- Status: `Todo`
- Priority: `High`
- Type: `Feature`

### Description
Add a fake-data mode for the current `GET /api/edu` endpoint so local/dev testing does not require live NSC vendor calls.

### Scope
- Add a config flag (for example: `EDU_USE_FAKE_DATA`) to enable fake responses.
- When enabled, return a deterministic `education.Response` payload from `GET /api/edu`.
- Preserve existing auth and route-level circuit breaker behavior.
- Avoid outbound NSC calls when fake mode is enabled.

### Acceptance Criteria
- With fake mode enabled, `GET /api/edu` returns `200` with a valid `education.Response` JSON payload and no vendor dependency.
- With fake mode disabled, existing behavior remains unchanged (current NSC integration path).
- Unit tests cover both fake and real-mode branching.
- Endpoint contract (response shape/fields) remains stable.
