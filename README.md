# Trace

Trace는 결정 당시 알고 있던 정보와 나중에 확인된 결과를 분리해 기록하고, 과거 시점으로 돌아가 판단 품질을 다시 보는 Decision Intelligence 서비스입니다.

> **Trace — Remember why you decided.**

## 구현 범위

- Decision → Evidence (`known_at`) → Expectation → Outcome → Reflection
- 백엔드에서 미래 정보를 제거하는 Decision Replay
- Focus Node, Time Slider, Then vs Now, Purple AI Layer 기반 반응형 UI
- Go 단일 바이너리 + React 정적 번들 + PostgreSQL
- 로컬 Bootstrap 관리자 및 Keycloak OIDC Discovery/PKCE SSO
- 관리자 설정 기반 선택적 팀장 검토·승인·반려
- 변경 가능한 RBAC, 사용자별 Envelope Encryption, 개인 키/데이터 키 회전
- OpenAI Responses 및 OpenAI 호환 Chat Completions SSE 스트리밍
- 최대 262,144(256K) 토큰 설정 검증
- REST API와 인증된 Streamable HTTP MCP endpoint (`/mcp`)
- 로그인 화면과 프로필 메뉴의 서비스 버전 표시
- BrowserRouter SPA fallback을 통한 새로고침 경로 복원
- 오프라인 반입용 `trace:v버전` 이미지와 `trace-v버전.tar.gz` GitHub Release

## UI 기술 선택

Trace는 일반적인 컴포넌트 라이브러리의 SaaS 모양을 그대로 쓰지 않습니다. React 위에 Radix UI 접근성 프리미티브, 자체 디자인 토큰, React Flow를 조합했습니다. 기본 글자 크기는 16px이고 모바일에서는 그래프 대신 시간 순서의 Story/Detail 경험을 우선합니다.

## 빠른 개발 실행

필수 도구는 Go 1.26, Node.js 24, PostgreSQL 15 이상입니다.

```bash
cd frontend
npm ci
npm run build
cd ..
cp -a frontend/dist/. internal/web/dist/
go build ./cmd/trace
```

Trace 프로세스가 읽는 환경변수는 정확히 네 개입니다.

```bash
export POSTGRES_DSN='postgres://trace:password@127.0.0.1:5432/trace?sslmode=disable'
export BOOTSTRAP_ADMIN='admin@example.internal'
export BOOTSTRAP_ADMIN_PASSWORD='a-long-bootstrap-password'
export ENCRYPTION_KEY="$(openssl rand -base64 32)"
./trace
```

`http://localhost:8080`에서 로그인합니다. 마이그레이션과 최초 관리자 생성은 시작 시 자동 수행됩니다. Bootstrap 관리자가 이미 존재하면 환경변수 비밀번호로 기존 비밀번호를 덮어쓰지 않습니다.

## 배포와 운영

- [오프라인 배포 및 운영](docs/operations.md)
- [보안과 키 관리](docs/security.md)
- [REST API와 MCP](docs/api.md)
- [OpenAPI 3.1 명세](api/openapi.yaml)
- [아키텍처](docs/architecture.md)

로컬 릴리스 이미지는 다음 명령으로 만듭니다.

```bash
./scripts/release-image.sh v0.1.0
```

산출물은 이미지 `trace:v0.1.0`, 파일 `dist/trace-v0.1.0.tar.gz`입니다. `v*` 태그를 `https://github.com/hkjang/trace`에 push하면 GitHub Actions가 동일한 이름으로 이미지만 압축해 Release에 첨부합니다.

## 검증

```bash
go test ./...
cd frontend && npm run build
docker build -t trace:v0.1.0 .
```

## 라이선스

Copyright © Trace contributors. 저장소 배포 정책에 따라 라이선스를 선택해 추가할 수 있습니다.
