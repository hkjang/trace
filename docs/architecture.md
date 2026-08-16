# 아키텍처

```text
Browser / MCP client
        │
        ▼
Go HTTP server :8080
  ├─ React static bundle + SPA fallback
  ├─ Local session / dynamic Keycloak OIDC
  ├─ REST API / Streamable HTTP MCP
  ├─ Replay context builder (known_at cutoff)
  ├─ SSE AI proxy (Responses / Chat Completions)
  └─ RBAC + envelope key service
        │
        ▼
PostgreSQL
```

## 시간 모델

`created_at`은 Trace에 기록된 시각, `effective_at`은 사건이 실제로 유효한 시각, `known_at`은 사용자가 알게 된 시각입니다. Replay API는 `known_at <= replay_at`을 SQL에서 적용한 결과만 반환하고 같은 filtered object만 AI context에 전달합니다.

## 운영 설정

네 개의 bootstrap 값만 환경변수이고 OIDC, AI, workflow, branding, 역할, 팀 설정은 관리자 API와 PostgreSQL에 저장됩니다. 민감 값은 root key로 암호화됩니다.

## 프론트엔드

- Desktop: Focus Graph + Detail + Time Slider
- Mobile: Graph를 숨기고 Decision Story/Timeline 우선
- 모든 메뉴 상태는 URL에 있으므로 서버가 어떤 앱 경로에도 `index.html`을 제공하고 React Router가 같은 화면을 복원합니다.
- 시스템 font stack만 사용해 외부 font 요청 없이 오프라인으로 동작합니다.
