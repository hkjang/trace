CREATE TABLE IF NOT EXISTS schema_migrations (
    version text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL,
    username text NOT NULL,
    display_name text NOT NULL,
    password_hash text,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    locale text NOT NULL DEFAULT 'ko-KR',
    timezone text NOT NULL DEFAULT 'Asia/Seoul',
    last_login_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_uidx ON users (lower(email));
CREATE UNIQUE INDEX users_username_lower_uidx ON users (lower(username));

CREATE TABLE roles (
    id uuid PRIMARY KEY,
    name text NOT NULL UNIQUE,
    description text NOT NULL DEFAULT '',
    is_system boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
    code text PRIMARY KEY,
    description text NOT NULL
);

CREATE TABLE role_permissions (
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_code text NOT NULL REFERENCES permissions(code) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_code)
);

CREATE TABLE user_roles (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_by uuid REFERENCES users(id) ON DELETE SET NULL,
    assigned_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE system_settings (
    key text PRIMARY KEY,
    value jsonb NOT NULL DEFAULT '{}'::jsonb,
    encrypted_value text,
    is_secret boolean NOT NULL DEFAULT false,
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_data_keys (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    encrypted_key text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'retired')),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    retired_at timestamptz,
    PRIMARY KEY (user_id, version)
);
CREATE UNIQUE INDEX user_data_keys_one_active_idx ON user_data_keys (user_id) WHERE status = 'active';

CREATE TABLE personal_keys (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('ai_provider', 'integration', 'signing', 'other')),
    encrypted_value text NOT NULL,
    data_key_version integer NOT NULL,
    permissions jsonb NOT NULL DEFAULT '["use"]'::jsonb,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    expires_at timestamptz,
    last_rotated_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, name),
    FOREIGN KEY (user_id, data_key_version) REFERENCES user_data_keys(user_id, version)
);

CREATE TABLE api_tokens (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    token_prefix text NOT NULL,
    token_hash text NOT NULL UNIQUE,
    scopes text[] NOT NULL DEFAULT ARRAY[]::text[],
    last_used_at timestamptz,
    expires_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    auth_method text NOT NULL CHECK (auth_method IN ('local', 'oidc')),
    user_agent text NOT NULL DEFAULT '',
    ip_address text NOT NULL DEFAULT '',
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id, expires_at DESC);

CREATE TABLE oidc_identities (
    issuer text NOT NULL,
    subject text NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    claims jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (issuer, subject)
);

CREATE TABLE oauth_states (
    state_hash text PRIMARY KEY,
    nonce text NOT NULL,
    code_verifier text NOT NULL,
    return_to text NOT NULL DEFAULT '/',
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE teams (
    id uuid PRIMARY KEY,
    name text NOT NULL UNIQUE,
    description text NOT NULL DEFAULT '',
    manager_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE team_members (
    team_id uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    member_role text NOT NULL DEFAULT 'member' CHECK (member_role IN ('member', 'manager')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);

CREATE TABLE decisions (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id uuid REFERENCES teams(id) ON DELETE SET NULL,
    title text NOT NULL,
    category text NOT NULL DEFAULT 'other',
    decision text NOT NULL,
    reason text NOT NULL DEFAULT '',
    assumptions text NOT NULL DEFAULT '',
    invalidation_conditions text NOT NULL DEFAULT '',
    confidence integer NOT NULL CHECK (confidence BETWEEN 0 AND 100),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'closed', 'archived')),
    workflow_state text NOT NULL DEFAULT 'not_required' CHECK (workflow_state IN ('not_required', 'draft', 'pending', 'approved', 'rejected')),
    decided_at timestamptz NOT NULL,
    review_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version integer NOT NULL DEFAULT 1
);
CREATE INDEX decisions_owner_updated_idx ON decisions (owner_id, updated_at DESC);
CREATE INDEX decisions_team_updated_idx ON decisions (team_id, updated_at DESC) WHERE team_id IS NOT NULL;
CREATE INDEX decisions_review_idx ON decisions (review_at) WHERE review_at IS NOT NULL AND status = 'active';

CREATE TABLE decision_alternatives (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE decision_evidence (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    title text NOT NULL,
    evidence_type text NOT NULL DEFAULT 'note' CHECK (evidence_type IN ('url', 'file', 'note', 'data')),
    source text NOT NULL DEFAULT '',
    content text NOT NULL DEFAULT '',
    snapshot text NOT NULL DEFAULT '',
    summary text NOT NULL DEFAULT '',
    reliability integer CHECK (reliability BETWEEN 0 AND 100),
    stance text NOT NULL DEFAULT 'neutral' CHECK (stance IN ('support', 'neutral', 'against')),
    published_at timestamptz,
    known_at timestamptz NOT NULL,
    captured_at timestamptz NOT NULL DEFAULT now(),
    added_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX decision_evidence_replay_idx ON decision_evidence (decision_id, known_at, created_at);

CREATE TABLE decision_expectations (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    expectation text NOT NULL,
    success_criteria text NOT NULL DEFAULT '',
    expected_at timestamptz,
    probability integer CHECK (probability BETWEEN 0 AND 100),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE decision_outcomes (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    result text NOT NULL,
    outcome_score integer NOT NULL CHECK (outcome_score BETWEEN -2 AND 2),
    decision_quality integer CHECK (decision_quality BETWEEN -2 AND 2),
    outcome_at timestamptz NOT NULL,
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE decision_reflections (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    reflection text NOT NULL,
    learning text NOT NULL DEFAULT '',
    reasoning_still_sound boolean,
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE decision_ai_insights (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    insight_type text NOT NULL,
    content jsonb NOT NULL,
    model text NOT NULL,
    prompt_version text NOT NULL,
    replay_at timestamptz,
    input_snapshot_hash text NOT NULL,
    created_by uuid NOT NULL REFERENCES users(id),
    generated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE decision_events (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    effective_at timestamptz NOT NULL,
    known_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX decision_events_replay_idx ON decision_events (decision_id, known_at, effective_at);

CREATE TABLE decision_links (
    source_decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    target_decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    relation text NOT NULL CHECK (relation IN ('caused_by', 'depends_on', 'replaced', 'follow_up', 'related', 'conflicts_with')),
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (source_decision_id, target_decision_id, relation),
    CHECK (source_decision_id <> target_decision_id)
);

CREATE TABLE approval_requests (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    requester_id uuid NOT NULL REFERENCES users(id),
    reviewer_id uuid REFERENCES users(id) ON DELETE SET NULL,
    state text NOT NULL CHECK (state IN ('pending', 'approved', 'rejected', 'cancelled')),
    request_note text NOT NULL DEFAULT '',
    response_note text NOT NULL DEFAULT '',
    requested_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz
);
CREATE UNIQUE INDEX approval_one_pending_idx ON approval_requests (decision_id) WHERE state = 'pending';

CREATE TABLE approval_events (
    id uuid PRIMARY KEY,
    approval_request_id uuid NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    actor_id uuid NOT NULL REFERENCES users(id),
    action text NOT NULL CHECK (action IN ('requested', 'approved', 'rejected', 'cancelled')),
    note text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
    id uuid PRIMARY KEY,
    actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    ip_address text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_created_idx ON audit_logs (created_at DESC);
