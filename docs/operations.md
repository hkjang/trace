# 오프라인 배포와 운영

## 1. 연결망에서 릴리스 파일 받기

GitHub Release에는 Trace 서비스 Docker 이미지 하나를 담은 `trace-v버전.tar.gz`만 게시됩니다. PostgreSQL과 Keycloak은 대상 조직의 기존 내부 인프라를 사용합니다.

## 2. 폐쇄망으로 반입하고 이미지 적재

```bash
gzip -dc trace-v0.1.1.tar.gz | docker image load
docker image inspect trace:v0.1.1
```

이미지 이름은 `trace:v0.1.1`, 파일 이름은 `trace-v0.1.1.tar.gz`입니다.

## 3. 네 개의 환경변수 준비

Trace 애플리케이션은 다음 값 외의 런타임 환경변수를 읽지 않습니다.

| 이름 | 목적 |
| --- | --- |
| `POSTGRES_DSN` | PostgreSQL 연결 문자열 |
| `BOOTSTRAP_ADMIN` | 최초 로컬 관리자 이메일 또는 아이디 |
| `BOOTSTRAP_ADMIN_PASSWORD` | 최초 관리자 비밀번호, 최소 12자 |
| `ENCRYPTION_KEY` | 32바이트 루트 키의 Base64/Hex/Raw 표현 |

키 생성 예:

```bash
openssl rand -base64 32
```

`ENCRYPTION_KEY`는 반드시 별도 비밀 저장소에 백업하고 모든 재시작에서 같은 값을 사용해야 합니다. 잃으면 저장된 OIDC/AI 비밀과 개인 데이터 키를 복호화할 수 없습니다.

## 4. 컨테이너 실행

`deploy/.env.example`을 `.env`로 복사해 값을 채우고 다음을 실행합니다.

```bash
docker compose --env-file .env -f deploy/compose.yml up -d
curl --fail http://127.0.0.1:8080/healthz
```

Compose 예제는 read-only root filesystem, 모든 Linux capability 제거, no-new-privileges를 적용합니다. TLS는 조직의 reverse proxy에서 종료하고 `X-Forwarded-Proto: https`를 전달하십시오.

## 5. Keycloak 연결

관리자 → 서비스 관리 → Keycloak SSO에서 다음만 입력합니다.

1. Realm Issuer URL: `https://keycloak.internal/realms/<realm>`
2. Public Base URL: 사용자가 접근할 Trace 주소
3. Client ID와 Client Secret
4. 화면에 계산된 Redirect URI를 Keycloak client의 Valid redirect URI에 등록
5. 활성화 후 저장

Trace는 OIDC Discovery, authorization code flow, PKCE S256, state/nonce 검증을 자동 적용합니다. 기본 claim은 `preferred_username`, `email`, `name`이며 관리 화면에서 바꿀 수 있습니다.

## 6. AI 연결

관리자 → AI 연동에서 내부 AI gateway 또는 외부 OpenAI endpoint를 설정합니다. 폐쇄망에서는 `Base URL`을 망 내부 OpenAI 호환 endpoint로 지정합니다. API key는 데이터베이스에 AES-256-GCM으로 암호화되며 환경변수로 받지 않습니다.

Responses 프로토콜은 `/responses`, Chat Completions 호환 프로토콜은 `/chat/completions`에 연결합니다. 모든 호출은 기본 SSE 스트리밍이고 최대 출력/컨텍스트 설정은 각각 262,144를 넘을 수 없습니다.

## 7. 백업과 복구

- PostgreSQL 전체 백업과 `ENCRYPTION_KEY` 백업은 한 쌍으로 관리합니다.
- 데이터베이스에는 스키마 마이그레이션 이력, 세션, 감사 로그, 키 버전, AI 입력 snapshot hash가 포함됩니다.
- 복구 후 같은 버전 이미지와 같은 `ENCRYPTION_KEY`로 시작하고 `/healthz`, 로그인, 관리자 설정 복호화를 확인합니다.
- 이미지 rollback은 이전 `trace:v버전`을 실행합니다. DB migration은 forward-only이므로 운영 전 DB snapshot을 생성하십시오.
