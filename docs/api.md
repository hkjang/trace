# REST API와 MCP

## 인증

브라우저는 `/api/v1/auth/login` 또는 Keycloak OIDC 후 HttpOnly session cookie를 사용합니다. 외부 API/MCP client는 개인화 → 개인 액세스 토큰에서 token을 발급하고 다음 헤더를 보냅니다.

```http
Authorization: Bearer trc_...
```

주요 REST 경로는 [OpenAPI 문서](../api/openapi.yaml)에 정의되어 있습니다.

## AI SSE

```http
POST /api/v1/decisions/{id}/ai/stream
Content-Type: application/json

{"mode":"replay","replayAt":"2026-08-16T00:00:00Z"}
```

응답 event는 `meta`, `delta`, `done`, `error`입니다. Replay mode에서는 endpoint 호출 전에 PostgreSQL cutoff가 적용됩니다.

## MCP

Endpoint는 `/mcp`이며 Streamable HTTP JSON-RPC를 사용합니다. 현재 server protocol은 `2025-11-25`이고 `2025-03-26`, `2025-06-18` client도 협상합니다. POST 요청의 `Accept`는 두 content type을 모두 포함해야 합니다.

```http
Accept: application/json, text/event-stream
MCP-Protocol-Version: 2025-11-25
Authorization: Bearer trc_...
```

도구는 token scope에 따라 결정론적 순서로 노출됩니다.

| 도구 | scope | 동작 |
| --- | --- | --- |
| `trace.list_decisions` | `decisions:read` | 보이는 판단 목록 |
| `trace.get_decision` | `decisions:read` | 판단 전체 context |
| `trace.replay_decision` | `decisions:read` | 지정 시점까지의 정보만 반환 |
| `trace.create_decision` | `decisions:write` | 승인 설정에 따라 Active 또는 Draft 생성 |

초기화 예:

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"client","version":"1.0"}}}
```

Trace의 stateless MCP는 server-to-client notification stream을 열지 않으므로 GET은 405를 반환하고 모든 request를 개별 POST로 처리합니다.
