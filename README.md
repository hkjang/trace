# <img src="frontend/public/logo.svg" width="36" height="36" align="center" style="vertical-align: middle; margin-right: 8px;" /> Trace

<div align="center">
  <a href="docs/index.html#video-demo">
    <img src="docs/images/08_decision_detail_graph.png" alt="Trace Decision Flow Graph & Video Demo" width="900" style="border-radius: 12px; box-shadow: 0 10px 30px rgba(0,0,0,0.5);" />
  </a>

  <p align="center">
    <strong>Remember why you decided.</strong><br />
    결과가 아니라, 당시 알고 있던 정보와 판단의 품질을 시간축에 남겨 다시 보는 <strong>Decision Intelligence Platform</strong>
  </p>

  <p align="center">
    <a href="docs/index.html"><strong>🌐 웹 홍보 및 소개 페이지</strong></a> •
    <a href="docs/trace-demo.mp4"><strong>🎬 3분 제품 시연 비디오 (MP4)</strong></a> •
    <a href="docs/guide.md"><strong>📖 사용자 가이드</strong></a> •
    <a href="docs/cru-manual.md"><strong>⚙️ CRU 매뉴얼</strong></a> •
    <a href="docs/architecture.md"><strong>🏛️ 아키텍처</strong></a> •
    <a href="docs/security.md"><strong>🔐 보안 & 암호화</strong></a> •
    <a href="docs/api.md"><strong>🔌 REST & MCP API</strong></a>
  </p>
</div>

---

## 🌟 핵심 특징 (Key Highlights)

- **결정 당시 앎의 시점(`known_at`) 보존**: 문서 발행일이 아닌 '내가 이 정보를 알게 된 시점'을 기준으로 근거(Evidence) 바인딩
- **Decision Replay (과거 시점 복원)**: 사후 확증 편향(Hindsight Bias)을 방지하기 위해 미래에 발생한 결과/증거를 수학적으로 제거하고 당시 시각으로 롤백
- **인터랙티브 디시전 그래프**: React Flow 기반으로 `Decision → Evidence → Alternatives → Expectation → Outcome` 관계망 직관화
- **Skill vs Luck 판단 매트릭스**: 좋은 판단과 좋은 결과를 독립적으로 교차 분석하여 조직의 의사결정 역량을 자산화
- **Zero-Trust & Envelope Encryption**: 사용자별 AES-256-GCM 봉투 암호화, Keycloak OIDC SSO, 무중단 키 회전
- **AI Streaming Co-Pilot & Streamable HTTP MCP**: 과거 시점 컨텍스트 기반 LLM 객관성 검토 및 `/mcp` 엔드포인트 지원

---

## 🎬 3분 제품 시연 비디오 (Demo Video)

서비스의 전체 라이프사이클(로그인 → 5단계 판단 입력 → 대화형 노드 그래프 → 사후 증거/결과/회고 등록 → 과거 시점 Decision Replay → 인사이트 & 관리자 콘솔)을 담은 데모 비디오를 제공합니다:

- **동영상 파일**: [`docs/trace-demo.mp4`](docs/trace-demo.mp4) (1440×900 HD, H.264, 3.2MB)
- **온라인 시청**: [`docs/index.html#video-demo`](docs/index.html#video-demo) (웹 비디오 플레이어)

---

## 📸 주요 화면 (Product Tour)

| 01. 로그인 & OIDC SSO | 02. 시간의 흐름 (홈 대시보드) |
| :---: | :---: |
| <img src="docs/images/01_login_page.png" width="440" /> | <img src="docs/images/02_home_dashboard.png" width="440" /> |

| 03. 5단계 판단 입력 (CRU Create) | 04. 디시전 인터랙티브 그래프 |
| :---: | :---: |
| <img src="docs/images/03_new_decision_step1.png" width="440" /> | <img src="docs/images/08_decision_detail_graph.png" width="440" /> |

| 05. 과거 시점 복원 (Time Replay) | 06. 판단 패턴 & 인사이트 분석 |
| :---: | :---: |
| <img src="docs/images/12_decision_time_replay.png" width="440" /> | <img src="docs/images/14_insights_analytics.png" width="440" /> |

---

## 🚀 빠른 시작 (Quick Start)

필수 도구: **Go 1.26+**, **Node.js 24+**, **PostgreSQL 15+**

```bash
# 1. PostgreSQL 컨테이너 기동
docker run -d --name trace-db -p 5432:5432 \
  -e POSTGRES_USER=trace \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=trace postgres:16-alpine

# 2. 프론트엔드 빌드 및 Go 바이너리 컴파일
cd frontend && npm ci && npm run build && cd ..
mkdir -p internal/web/dist && cp -a frontend/dist/. internal/web/dist/
go build -o trace ./cmd/trace

# 3. 환경변수 설정 및 실행 (정확히 4개)
export POSTGRES_DSN='postgres://trace:password@127.0.0.1:5432/trace?sslmode=disable'
export BOOTSTRAP_ADMIN='admin@example.internal'
export BOOTSTRAP_ADMIN_PASSWORD='a-long-bootstrap-password'
export ENCRYPTION_KEY="$(openssl rand -base64 32)"

./trace
```

브라우저에서 `http://localhost:8080`에 접속하여 로그인합니다.

---

## 📚 상세 문서 목록

- [🌐 홍보 및 쇼케이스 웹페이지 (`docs/index.html`)](docs/index.html)
- [🎬 3분 제품 시연 동영상 (`docs/trace-demo.mp4`)](docs/trace-demo.mp4)
- [📖 사용자 가이드 (`docs/guide.md`)](docs/guide.md)
- [⚙️ CRU (Create, Read, Update/Replay) 실무 매뉴얼 (`docs/cru-manual.md`)](docs/cru-manual.md)
- [🏛️ 아키텍처 및 시스템 설계 (`docs/architecture.md`)](docs/architecture.md)
- [🔐 보안 명세 및 봉투 암호화 (`docs/security.md`)](docs/security.md)
- [🔌 REST API 및 MCP 엔드포인트 명세 (`docs/api.md`)](docs/api.md)
- [📋 OpenAPI 3.1 명세 (`api/openapi.yaml`)](api/openapi.yaml)
- [🚢 오프라인 배포 및 운영 가이드 (`docs/operations.md`)](docs/operations.md)

---

## 📄 라이선스

Copyright © 2026 Trace Contributors. All rights reserved.
