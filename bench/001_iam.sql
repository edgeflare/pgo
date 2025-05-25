CREATE SCHEMA IF NOT EXISTS iam;

CREATE OR REPLACE FUNCTION iam.generate_dex_sub(p_id text, p_connector_id text)
RETURNS text AS $$
DECLARE
    binary_data bytea;
BEGIN
    binary_data := E'\\x0a'::bytea || 
                   set_byte(E'\\x00'::bytea, 0, length(p_id)) ||
                   convert_to(p_id, 'UTF8') ||
                   E'\\x12'::bytea || 
                   set_byte(E'\\x00'::bytea, 0, length(p_connector_id)) ||
                   convert_to(p_connector_id, 'UTF8');
    
    -- Base64 encode, make URL-safe, and remove padding
    RETURN replace(
             replace(
               rtrim(encode(binary_data, 'base64'), '='), 
               '+', '-'), 
             '/', '_');
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- users table
CREATE TABLE IF NOT EXISTS iam.users (
    id TEXT NOT NULL,
    name TEXT,
    email TEXT,
    preferred_username TEXT,
    groups BYTEA,
    connector_id TEXT NOT NULL,
    sub TEXT PRIMARY KEY GENERATED ALWAYS AS (iam.generate_dex_sub(id, connector_id)) STORED,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX users_id_connector_id_idx ON iam.users (id, connector_id);
CREATE INDEX users_email_idx ON iam.users (email);

CREATE OR REPLACE FUNCTION iam.user_jwt_sub() RETURNS TEXT AS $$
DECLARE
    jwt_claim_sub TEXT;
BEGIN
    SELECT (current_setting('request.jwt.claims', true)::json->>'sub')::TEXT INTO jwt_claim_sub;
    RETURN jwt_claim_sub;
END;
$$ LANGUAGE plpgsql STABLE;
