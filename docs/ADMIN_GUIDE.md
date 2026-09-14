# Trace 관리자 가이드

관리자 → 서비스 관리 화면에서 바꿀 수 있는 설정 가운데 동작을 알고 써야 하는 항목을 설명합니다. 설치·백업·환경변수는 [운영 가이드](operations.md), 화면 사용법은 [사용자 가이드](guide.md)를 보십시오.

## Keycloak SSO

기본 연결 절차는 [운영 가이드 5장](operations.md#5-keycloak-연결)에 있습니다. 이 장은 그 위에 얹는 옵션을 다룹니다.

### 자동 로그인 (`auto_login`, Silent SSO)

Keycloak(또는 ReSSO)에 이미 로그인한 사람이 Trace를 열면 **로그인 화면 없이 바로 본 화면**으로 들어가게 하는 설정입니다. OIDC `prompt=none`(silent authentication)을 씁니다.

- 위치: 관리자 → 서비스 관리 → Keycloak SSO → *클레임과 Scope 고급 설정* → **자동 로그인 (Silent SSO)**
- 설정 키: `oidc.autoLogin`
- **기본값: 꺼짐.** 켜지 않은 설치에서는 아무것도 달라지지 않습니다.
- 전제: Keycloak SSO 활성화가 켜져 있어야 합니다. 꺼져 있으면 이 설정은 무시됩니다.

#### 동작

1. 브라우저가 세션 없이 Trace 페이지(예 `/decisions/42`)를 열면 `/api/v1/auth/oidc/start?prompt=none&returnTo=/decisions/42` 로 **최상위 이동**합니다. 숨은 iframe을 쓰지 않으므로 서드파티 쿠키가 막힌 브라우저에서도 동작하고 Keycloak의 프레임 정책과 무관합니다.
2. 서버는 `auto_login` 이 켜져 있을 때만 Keycloak 인가 요청에 `prompt=none` 을 붙입니다. 꺼져 있으면 주소에 `?prompt=none` 이 붙어 있어도 **조용히 평범한 로그인**으로 바꿉니다 — 누구든 주소를 고쳐 흐름을 바꿀 수 없습니다.
3. Keycloak 세션이 있으면 인가 코드가 바로 돌아오고 평소 로그인 흐름을 거쳐 원래 자리(`returnTo`)로 돌아갑니다.
4. Keycloak 세션이 없으면 `error=login_required` 로 돌아옵니다. 이것은 오류가 아니라 "세션 없음"이라는 평범한 대답입니다. 콜백은 이때 `/login?sso=none` 으로 보내 로그인 화면을 보여줍니다.

#### 무한 루프 방지

`prompt=none` 이 거절된 뒤 다시 시도하면 브라우저가 Keycloak과 Trace 사이를 끝없이 오갑니다. Trace는 세 겹으로 막습니다.

| 장치 | 내용 |
|------|------|
| 탭 세션당 한 번 | 시도 표시를 `sessionStorage`(`trace.sso.silentAttempted`)에 남깁니다. 거절 뒤 새로고침해도 다시 시도하지 않고, 새 탭을 열면 다시 한 번 시도합니다. |
| 로그아웃 억제 | 사용자가 직접 로그아웃하면 `trace.sso.signedOut` 표시를 남겨 자동 로그인을 멈춥니다. 다시 로그인해 세션이 생기면 풀립니다. |
| 주소 표시 | 콜백이 거절을 받으면 `/login?sso=none` 으로 보냅니다. 브라우저 저장소가 지워졌더라도 이 표시가 있으면 다시 시도하지 않습니다. |

`sessionStorage` 를 읽지 못하는 환경(사생활 보호 모드, 사이트 데이터 차단)에서는 **"이미 시도했다"** 로 간주하고 시도하지 않습니다. 막히는 쪽으로 실패합니다.

시도하지 않는 경로: `/login`, `/api/…`(콜백 포함), `/mcp`, `/healthz`. 브라우저 페이지 이동에만 해당합니다.

`returnTo` 는 `/` 로 시작하고 `//` 로 시작하지 않는 같은 출처 경로만 받습니다. 그 외 값은 `/` 로 바뀝니다.

#### 켜기 전 점검

- Keycloak client의 Valid redirect URI에 Trace 콜백(`<Public Base URL>/api/v1/auth/oidc/callback`)이 등록되어 있어야 합니다. 평범한 SSO와 같습니다.
- Keycloak에 로그인한 상태로 Trace를 열면 로그인 화면 없이 본 화면이 떠야 합니다.
- 로그인하지 않은 상태로 열면 로그인 화면이 한 번 뜨고, 새로고침을 반복해도 깜빡이거나 리다이렉트가 반복되지 않아야 합니다.
- 로그아웃한 뒤 다시 열어도 자동으로 로그인되지 않아야 합니다.

#### 문제 해결

- **로그인 화면이 계속 뜬다**: Keycloak 세션이 실제로 없거나(다른 realm·브라우저 프로필) `auto_login` 이 꺼져 있습니다. `/login?sso=none` 으로 돌아왔다면 Keycloak이 `login_required` 를 답한 것입니다.
- **`/login?error=…` 로 돌아온다**: 조용한 시도가 아닌 평범한 로그인에서 Keycloak이 오류를 낸 것입니다. 서버 로그의 `OIDC provider returned an error` 를 확인하십시오.
