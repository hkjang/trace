# 아키텍처

```text
Browser / MCP client
        │
        ▼
Go HTTP server :8080
  ├─ React static bundle + SPA fallback
  ├─ Local session / dynamic Keycloak OIDC
  ├─ REST API / Streamable HTTP MCP
  ├─ Temporal state builder (version + known_at cutoff)
  ├─ Decision graph / review priority engine
  ├─ Semantic memory context ranker
  ├─ SSE AI proxy (Responses / Chat Completions)
  └─ RBAC + envelope key service
        │
        ▼
PostgreSQL
```

## 시간 모델

`created_at`은 Trace에 기록된 시각, `effective_at`은 사건이 실제로 유효한 시각, `known_at`은 사용자가 알게 된 시각입니다. Replay API는 `known_at <= replay_at`을 SQL에서 적용한 결과만 반환하고 같은 filtered object만 AI context에 전달합니다.

Decision 본문의 변경은 `decision_versions.valid_from/valid_to`에 보존됩니다. 확신, 전제, 반증 조건, 관계도 각각 이벤트 이력을 가지며 Replay Compare는 임의의 두 시점에서 이 상태들을 독립적으로 복원합니다. 관계 해제는 row 삭제 대신 `deleted_at`을 사용해 현재 Graph에서는 사라지고 과거 Graph에는 남습니다.

## Decision Intelligence

```text
Decision Versions ─┐
Confidence History ├─ Temporal Snapshot ─ Replay Compare
Assumption Events  ┤
Evidence known_at ─┘

Decision + Evidence + Outcome + Reflection
        │
        ├─ Offline local vectors (default)
        └─ OpenAI-compatible embeddings (optional)
                    │
                    ▼
 similarity + recency + category + relation + outcome + reliability
                    │
                    ▼
              Memory Context Builder
```

로컬 벡터는 PostgreSQL 확장 없이 동작하는 고정 차원 특성 벡터입니다. 관리자가 embedding model을 설정하면 호환 `/embeddings` batch API를 사용하고, 호출 실패 시 로컬 경로로 전환합니다. 따라서 Semantic Search는 외부 연결의 가용성과 무관하게 유지됩니다.

Review Priority는 결과/검토일 도래, 높은 확신, 새 Evidence, 위험 전제, 장기 미검토, 발동된 반증 조건을 설명 가능한 규칙으로 합산합니다. Decision Health는 결과의 좋고 나쁨과 판단 품질을 섞지 않고 현재 검토 필요도만 `HEALTHY`, `WATCH`, `NEEDS_REVIEW`, `CRITICAL`로 표시합니다.

## 운영 설정

네 개의 bootstrap 값만 환경변수이고 OIDC, AI, workflow, branding, 역할, 팀 설정은 관리자 API와 PostgreSQL에 저장됩니다. 민감 값은 root key로 암호화됩니다.

## 프론트엔드

- Desktop: Focus Graph + Detail + Time Slider
- Mobile: Graph를 숨기고 Decision Story/Timeline 우선
- 모든 메뉴 상태는 URL에 있으므로 서버가 어떤 앱 경로에도 `index.html`을 제공하고 React Router가 같은 화면을 복원합니다.
- 시스템 font stack만 사용해 외부 font 요청 없이 오프라인으로 동작합니다.
