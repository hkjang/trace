-- Silent SSO (prompt=none): the callback must know whether a refusal such as
-- login_required came from a silent attempt, so the browser can be told not
-- to retry.

ALTER TABLE oauth_states ADD COLUMN silent boolean NOT NULL DEFAULT false;
