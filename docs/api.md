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

분석 mode는 `review`, `counter`, `counterfactual`, `assumption`, `replay`, `clarify`를 지원합니다. 모든 실행은 `ai_analysis_runs`에 model, prompt/context version, replay 시점, 입력 hash, 결과 또는 오류 상태로 추적됩니다. 개인 패턴 분석은 `POST /api/v1/ai/patterns/stream`을 사용합니다.

## Decision Intelligence API

| 영역 | 주요 경로 | 설명 |
| --- | --- | --- |
| Versioning | `GET /decisions/{id}/versions` | 덮어쓰기 없는 판단 변경 이력 |
| Replay Compare | `GET /decisions/{id}/replay/compare?from=&to=` | 두 시점의 버전·확신·근거·전제 차이 |
| Network | `GET /decisions/{id}/graph`, `GET /graph` | 1/2-hop Focus Graph와 전체 Graph |
| Relations | `POST /decisions/{id}/links`, `DELETE /decision-links/{id}` | 관계 생성과 시간축을 보존하는 연결 해제 |
| Confidence | `POST /decisions/{id}/confidence` | 확신 변화와 이유 기록 |
| Assumption | `POST /decisions/{id}/assumptions`, `PATCH /assumptions/{id}` | 전제 상태 전이 |
| Invalidation | `POST /decisions/{id}/invalidation-conditions`, `PATCH /invalidation-conditions/{id}` | 반증 조건 감지·해소 이력 |
| Semantic Memory | `POST /search/semantic`, `GET /decisions/{id}/similar` | 자연어 및 유사 판단 검색 |
| Review | `GET /reviews`, `POST /decisions/{id}/review` | 우선순위 검토 Inbox와 다음 검토 일정 |
| Intelligence | `GET /analytics/{calibration,biases,patterns,profile}` | 확신 보정, 편향·패턴·개인 프로필 |

Semantic Memory는 관리자 AI 설정의 `embeddingModel`이 있으면 OpenAI-compatible `/embeddings`를 사용합니다. 설정이 없거나 provider가 실패하면 `trace-local-v1` 로컬 벡터로 즉시 전환하므로 폐쇄망에서도 검색이 유지됩니다. 응답의 `model`과 `fallback`으로 어느 경로가 사용됐는지 확인할 수 있습니다.

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
| `trace.compare_replay` | `decisions:read` | 두 시점의 정확한 판단 상태 비교 |
| `trace.get_decision_graph` | `decisions:read` | 1/2-hop 판단 관계 탐색 |
| `trace.search_memory` | `decisions:read` | 자연어로 판단 메모리 검색 |
| `trace.create_decision` | `decisions:write` | 승인 설정에 따라 Active 또는 Draft 생성 |

초기화 예:

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"client","version":"1.0"}}}
```

Trace의 stateless MCP는 server-to-client notification stream을 열지 않으므로 GET은 405를 반환하고 모든 request를 개별 POST로 처리합니다.
