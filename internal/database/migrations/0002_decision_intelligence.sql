-- Trace phase 2: temporal decision network and personal intelligence.

ALTER TABLE decision_links RENAME COLUMN relation TO relation_type;
ALTER TABLE decision_links DROP CONSTRAINT IF EXISTS decision_links_relation_check;
ALTER TABLE decision_links DROP CONSTRAINT IF EXISTS decision_links_pkey;
ALTER TABLE decision_links ADD COLUMN id uuid;
ALTER TABLE decision_links ADD COLUMN description text NOT NULL DEFAULT '';
ALTER TABLE decision_links ADD COLUMN effective_at timestamptz;
UPDATE decision_links
SET id = md5(source_decision_id::text || target_decision_id::text || relation_type)::uuid,
    relation_type = CASE relation_type
        WHEN 'caused_by' THEN 'CAUSED_BY'
        WHEN 'depends_on' THEN 'DEPENDS_ON'
        WHEN 'replaced' THEN 'REPLACES'
        WHEN 'follow_up' THEN 'FOLLOW_UP'
        WHEN 'related' THEN 'RELATED_TO'
        WHEN 'conflicts_with' THEN 'CONFLICTS_WITH'
        ELSE upper(relation_type)
    END,
    effective_at = created_at;
ALTER TABLE decision_links ALTER COLUMN id SET NOT NULL;
ALTER TABLE decision_links ALTER COLUMN effective_at SET NOT NULL;
ALTER TABLE decision_links ALTER COLUMN effective_at SET DEFAULT now();
ALTER TABLE decision_links ADD PRIMARY KEY (id);
ALTER TABLE decision_links ADD CONSTRAINT decision_links_relation_type_check
    CHECK (relation_type IN ('DEPENDS_ON','CAUSED_BY','FOLLOW_UP','REPLACES','SUPPORTS','CONFLICTS_WITH','RELATED_TO'));
CREATE UNIQUE INDEX decision_links_unique_edge_idx
    ON decision_links(source_decision_id,target_decision_id,relation_type);
CREATE INDEX decision_links_source_time_idx ON decision_links(source_decision_id,effective_at);
CREATE INDEX decision_links_target_time_idx ON decision_links(target_decision_id,effective_at);

CREATE TABLE decision_versions (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    title text NOT NULL,
    category text NOT NULL,
    decision text NOT NULL,
    reason text NOT NULL DEFAULT '',
    assumptions text NOT NULL DEFAULT '',
    invalidation_conditions text NOT NULL DEFAULT '',
    confidence integer NOT NULL CHECK (confidence BETWEEN 0 AND 100),
    status text NOT NULL,
    workflow_state text NOT NULL,
    decided_at timestamptz NOT NULL,
    review_at timestamptz,
    change_reason text NOT NULL DEFAULT '',
    changed_by uuid REFERENCES users(id) ON DELETE SET NULL,
    valid_from timestamptz NOT NULL,
    valid_to timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(decision_id,version),
    CHECK (valid_to IS NULL OR valid_to >= valid_from)
);
CREATE INDEX decision_versions_replay_idx ON decision_versions(decision_id,valid_from DESC);
INSERT INTO decision_versions(
    id,decision_id,version,title,category,decision,reason,assumptions,
    invalidation_conditions,confidence,status,workflow_state,decided_at,review_at,
    change_reason,changed_by,valid_from,created_at
)
SELECT md5(id::text || ':version:1')::uuid,id,version,title,category,decision,reason,assumptions,
       invalidation_conditions,confidence,status,workflow_state,decided_at,review_at,
       'Initial decision',owner_id,decided_at,created_at
FROM decisions;

CREATE TABLE decision_confidence_history (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    confidence integer NOT NULL CHECK (confidence BETWEEN 0 AND 100),
    reason text NOT NULL DEFAULT '',
    recorded_by uuid REFERENCES users(id) ON DELETE SET NULL,
    recorded_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX decision_confidence_history_idx ON decision_confidence_history(decision_id,recorded_at);
INSERT INTO decision_confidence_history(id,decision_id,confidence,reason,recorded_by,recorded_at,created_at)
SELECT md5(id::text || ':confidence:1')::uuid,id,confidence,'Initial confidence',owner_id,decided_at,created_at
FROM decisions;

CREATE TABLE decision_assumptions (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    assumption text NOT NULL,
    status text NOT NULL DEFAULT 'UNKNOWN'
        CHECK (status IN ('UNKNOWN','ACTIVE','STRENGTHENED','WEAKENING','BROKEN')),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    known_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX decision_assumptions_replay_idx ON decision_assumptions(decision_id,known_at);
INSERT INTO decision_assumptions(id,decision_id,assumption,status,created_by,known_at,created_at,updated_at)
SELECT md5(id::text || ':assumption:1')::uuid,id,assumptions,'ACTIVE',owner_id,decided_at,created_at,updated_at
FROM decisions WHERE btrim(assumptions) <> '';

CREATE TABLE assumption_events (
    id uuid PRIMARY KEY,
    assumption_id uuid NOT NULL REFERENCES decision_assumptions(id) ON DELETE CASCADE,
    previous_status text,
    status text NOT NULL CHECK (status IN ('UNKNOWN','ACTIVE','STRENGTHENED','WEAKENING','BROKEN')),
    reason text NOT NULL DEFAULT '',
    evidence_id uuid REFERENCES decision_evidence(id) ON DELETE SET NULL,
    changed_by uuid REFERENCES users(id) ON DELETE SET NULL,
    known_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX assumption_events_replay_idx ON assumption_events(assumption_id,known_at);
INSERT INTO assumption_events(id,assumption_id,status,reason,changed_by,known_at,created_at)
SELECT md5(id::text || ':event:1')::uuid,id,status,'Initial assumption',created_by,known_at,created_at
FROM decision_assumptions;

CREATE TABLE invalidation_conditions (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    condition text NOT NULL,
    status text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','TRIGGERED','RESOLVED')),
    evidence_id uuid REFERENCES decision_evidence(id) ON DELETE SET NULL,
    detection_note text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    known_at timestamptz NOT NULL,
    triggered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invalidation_conditions_review_idx ON invalidation_conditions(decision_id,status,known_at);
INSERT INTO invalidation_conditions(id,decision_id,condition,status,created_by,known_at,created_at,updated_at)
SELECT md5(id::text || ':invalidation:1')::uuid,id,invalidation_conditions,'ACTIVE',owner_id,decided_at,created_at,updated_at
FROM decisions WHERE btrim(invalidation_conditions) <> '';

CREATE TABLE evidence_snapshots (
    id uuid PRIMARY KEY,
    evidence_id uuid NOT NULL REFERENCES decision_evidence(id) ON DELETE CASCADE,
    content text NOT NULL,
    content_hash text NOT NULL,
    captured_at timestamptz NOT NULL,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE(evidence_id,content_hash)
);

CREATE TABLE decision_embeddings (
    decision_id uuid PRIMARY KEY REFERENCES decisions(id) ON DELETE CASCADE,
    model text NOT NULL,
    dimensions integer NOT NULL CHECK (dimensions > 0),
    embedding double precision[] NOT NULL,
    input_hash text NOT NULL,
    generated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE evidence_embeddings (
    evidence_id uuid PRIMARY KEY REFERENCES decision_evidence(id) ON DELETE CASCADE,
    model text NOT NULL,
    dimensions integer NOT NULL CHECK (dimensions > 0),
    embedding double precision[] NOT NULL,
    input_hash text NOT NULL,
    generated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE review_schedules (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    next_review_at timestamptz NOT NULL,
    cadence_days integer CHECK (cadence_days IS NULL OR cadence_days > 0),
    enabled boolean NOT NULL DEFAULT true,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(decision_id)
);
CREATE INDEX review_schedules_due_idx ON review_schedules(next_review_at) WHERE enabled;
INSERT INTO review_schedules(id,decision_id,next_review_at,created_by,created_at,updated_at)
SELECT md5(id::text || ':review')::uuid,id,review_at,owner_id,created_at,updated_at
FROM decisions WHERE review_at IS NOT NULL;

CREATE TABLE review_events (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    reviewer_id uuid REFERENCES users(id) ON DELETE SET NULL,
    event_type text NOT NULL CHECK (event_type IN ('REVIEWED','SNOOZED','DISMISSED','AUTO_FLAGGED')),
    note text NOT NULL DEFAULT '',
    confidence integer CHECK (confidence BETWEEN 0 AND 100),
    reviewed_at timestamptz NOT NULL DEFAULT now(),
    next_review_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX review_events_decision_idx ON review_events(decision_id,reviewed_at DESC);

CREATE TABLE ai_analysis_runs (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    decision_id uuid REFERENCES decisions(id) ON DELETE CASCADE,
    analysis_type text NOT NULL,
    model text NOT NULL,
    prompt_version text NOT NULL,
    context_version text NOT NULL,
    replay_at timestamptz,
    input_hash text NOT NULL,
    output_json jsonb,
    status text NOT NULL DEFAULT 'RUNNING' CHECK (status IN ('RUNNING','COMPLETED','FAILED')),
    error_message text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX ai_analysis_runs_user_idx ON ai_analysis_runs(user_id,started_at DESC);
CREATE INDEX ai_analysis_runs_decision_idx ON ai_analysis_runs(decision_id,started_at DESC) WHERE decision_id IS NOT NULL;

CREATE TABLE decision_scores (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    analysis_run_id uuid REFERENCES ai_analysis_runs(id) ON DELETE SET NULL,
    evidence_quality integer CHECK (evidence_quality BETWEEN 0 AND 100),
    logic_quality integer CHECK (logic_quality BETWEEN 0 AND 100),
    alternative_consideration integer CHECK (alternative_consideration BETWEEN 0 AND 100),
    risk_awareness integer CHECK (risk_awareness BETWEEN 0 AND 100),
    assumption_quality integer CHECK (assumption_quality BETWEEN 0 AND 100),
    calibration integer CHECK (calibration BETWEEN 0 AND 100),
    counter_evidence integer CHECK (counter_evidence BETWEEN 0 AND 100),
    overall integer CHECK (overall BETWEEN 0 AND 100),
    estimated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX decision_scores_latest_idx ON decision_scores(decision_id,estimated_at DESC);

CREATE TABLE bias_detections (
    id uuid PRIMARY KEY,
    decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    analysis_run_id uuid REFERENCES ai_analysis_runs(id) ON DELETE SET NULL,
    bias_type text NOT NULL,
    severity text NOT NULL CHECK (severity IN ('LOW','MEDIUM','HIGH')),
    confidence integer CHECK (confidence BETWEEN 0 AND 100),
    evidence text NOT NULL DEFAULT '',
    detected_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX bias_detections_profile_idx ON bias_detections(decision_id,bias_type,detected_at DESC);
