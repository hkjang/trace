# 보안과 키 관리

## 암호화 계층

```text
ENCRYPTION_KEY (32-byte root, process only)
  ├─ system:oidc → Keycloak client secret
  ├─ system:ai   → AI provider API key
  └─ wrapped user data key vN
       └─ personal integration/API provider secrets
```

- AES-256-GCM nonce와 purpose별 Additional Authenticated Data를 사용합니다.
- 사용자별 데이터 키 회전은 새 키 생성, 모든 개인 비밀 재암호화, 이전 키 retire를 한 DB transaction에서 수행합니다.
- 개별 키 값 회전, 폐기, `permissions` 변경을 개인화 페이지와 API에서 지원합니다.
- 개인 API token은 `trc_` prefix의 256-bit random 값이며 원문을 한 번만 보여주고 SHA-256 hash만 저장합니다.

## 권한

기본 역할은 `administrator`, `team_manager`, `member`입니다. 관리자는 역할별 permission mapping을 화면에서 바꿀 수 있습니다. 관리자 역할의 모든 권한 제거는 잠금 방지를 위해 허용하지 않습니다.

MCP/REST token은 사용자 RBAC와 token scope를 모두 통과해야 합니다. 예를 들어 판단 생성은 `decisions.write` permission과 `decisions:write` scope가 동시에 필요합니다.

## Web 보안

- 세션 cookie: HttpOnly, SameSite=Lax, HTTPS에서 Secure
- cookie 기반 변경 요청: `X-Trace-Request: 1` 및 Origin 검증
- OIDC: state 일회용 DB 저장, nonce, PKCE S256, issuer/audience/signature 검증
- MCP: Bearer token 필수, Origin/Host 검증, protocol version 검증
- CSP, frame deny, no-sniff, referrer/permissions policy 헤더
- 비밀번호: bcrypt
- 비밀 값은 API 응답에서 placeholder로 마스킹

운영에서는 TLS reverse proxy, PostgreSQL TLS, 정기 DB 백업, 관리자 감사 로그 검토를 함께 적용하십시오.
