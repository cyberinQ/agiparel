-- Author: Sunil Jain, 2026-10-28.
-- F1.2: one-time OIDC authorization requests and opaque, revocable browser sessions.
-- No tenant membership or roles are inferred from session creation.
BEGIN;
CREATE TABLE workforce_oidc_requests_v16 (
  state_hash bytea PRIMARY KEY CHECK (octet_length(state_hash)=32),
  browser_hash bytea NOT NULL CHECK (octet_length(browser_hash)=32),
  nonce text NOT NULL CHECK (length(nonce) BETWEEN 32 AND 128),
  pkce_verifier text NOT NULL CHECK (length(pkce_verifier) BETWEEN 43 AND 128),
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL CHECK (expires_at>created_at)
);
CREATE INDEX workforce_oidc_requests_expire_v16 ON workforce_oidc_requests_v16(expires_at);
CREATE TABLE workforce_browser_sessions_v16 (
  session_hash bytea PRIMARY KEY CHECK (octet_length(session_hash)=32),
  csrf_hash bytea NOT NULL CHECK (octet_length(csrf_hash)=32),
  idp_issuer text NOT NULL CHECK (idp_issuer LIKE 'https://%'),
  idp_subject varchar(128) NOT NULL CHECK (length(idp_subject)>0),
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL CHECK (expires_at>created_at),
  revoked_at timestamptz
);
CREATE INDEX workforce_browser_sessions_expire_v16 ON workforce_browser_sessions_v16(expires_at);
CREATE INDEX workforce_browser_sessions_subject_v16 ON workforce_browser_sessions_v16(idp_issuer,idp_subject) WHERE revoked_at IS NULL;
ALTER TABLE workforce_oidc_requests_v16 ENABLE ROW LEVEL SECURITY;
ALTER TABLE workforce_oidc_requests_v16 FORCE ROW LEVEL SECURITY;
ALTER TABLE workforce_browser_sessions_v16 ENABLE ROW LEVEL SECURITY;
ALTER TABLE workforce_browser_sessions_v16 FORCE ROW LEVEL SECURITY;
REVOKE ALL ON workforce_oidc_requests_v16,workforce_browser_sessions_v16 FROM PUBLIC,agiparel_app;

-- Definer routines provide opaque, one-time lookups; the app cannot enumerate tables.
CREATE FUNCTION workforce_oidc_begin_v16(p_state bytea,p_browser bytea,p_nonce text,p_pkce text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 IF octet_length(p_state)<>32 OR octet_length(p_browser)<>32 OR length(p_nonce) NOT BETWEEN 32 AND 128 OR length(p_pkce) NOT BETWEEN 43 AND 128 THEN RAISE EXCEPTION 'invalid login input'; END IF;
 DELETE FROM public.workforce_oidc_requests_v16 WHERE expires_at<now();
 INSERT INTO public.workforce_oidc_requests_v16(state_hash,browser_hash,nonce,pkce_verifier,expires_at)
 VALUES(p_state,p_browser,p_nonce,p_pkce,now()+interval '5 minutes');
END $$;
CREATE FUNCTION workforce_oidc_consume_v16(p_state bytea,p_browser bytea)
RETURNS TABLE(nonce text,pkce_verifier text) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 RETURN QUERY DELETE FROM public.workforce_oidc_requests_v16 q
 WHERE q.state_hash=p_state AND q.browser_hash=p_browser AND q.expires_at>now()
 RETURNING q.nonce,q.pkce_verifier;
END $$;
CREATE FUNCTION workforce_session_create_v16(p_session bytea,p_csrf bytea,p_issuer text,p_subject text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 IF octet_length(p_session)<>32 OR octet_length(p_csrf)<>32 OR length(p_issuer) NOT BETWEEN 9 AND 512 OR p_issuer NOT LIKE 'https://%' OR length(p_subject) NOT BETWEEN 1 AND 128 THEN RAISE EXCEPTION 'invalid session input'; END IF;
 INSERT INTO public.workforce_browser_sessions_v16(session_hash,csrf_hash,idp_issuer,idp_subject,expires_at)
 VALUES(p_session,p_csrf,p_issuer,p_subject,now()+interval '8 hours');
END $$;
CREATE FUNCTION workforce_session_get_v16(p_session bytea)
RETURNS TABLE(idp_issuer text,idp_subject text,csrf_hash bytea) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 RETURN QUERY UPDATE public.workforce_browser_sessions_v16 s
 SET last_seen_at=now() WHERE s.session_hash=p_session AND s.revoked_at IS NULL
 AND s.expires_at>now() AND s.last_seen_at>now()-interval '20 minutes'
 RETURNING s.idp_issuer,s.idp_subject::text,s.csrf_hash;
END $$;
CREATE FUNCTION workforce_session_csrf_rotate_v16(p_session bytea,p_csrf bytea)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE changed integer;
BEGIN
 IF octet_length(p_session)<>32 OR octet_length(p_csrf)<>32 THEN RETURN false; END IF;
 UPDATE public.workforce_browser_sessions_v16 SET csrf_hash=p_csrf WHERE session_hash=p_session
 AND revoked_at IS NULL AND expires_at>now() AND last_seen_at>now()-interval '20 minutes';
 GET DIAGNOSTICS changed=ROW_COUNT;
 RETURN changed=1;
END $$;
CREATE FUNCTION workforce_session_revoke_v16(p_session bytea)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 UPDATE public.workforce_browser_sessions_v16 SET revoked_at=now() WHERE session_hash=p_session AND revoked_at IS NULL;
END $$;
REVOKE ALL ON FUNCTION workforce_oidc_begin_v16(bytea,bytea,text,text),
 workforce_oidc_consume_v16(bytea,bytea), workforce_session_create_v16(bytea,bytea,text,text),
 workforce_session_get_v16(bytea),workforce_session_csrf_rotate_v16(bytea,bytea),workforce_session_revoke_v16(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION workforce_oidc_begin_v16(bytea,bytea,text,text),
 workforce_oidc_consume_v16(bytea,bytea),workforce_session_create_v16(bytea,bytea,text,text),
 workforce_session_get_v16(bytea),workforce_session_csrf_rotate_v16(bytea,bytea),workforce_session_revoke_v16(bytea) TO agiparel_app;
COMMIT;
