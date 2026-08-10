-- SEC-3: Bind accept_invitation() to the invitee email.
--
-- accept_invitation (20241205_admin_roles.sql:124) found an invitation by token
-- only, then inserted a user_approvals row with user_id = auth.uid() without
-- ever comparing the invitation's email to the caller's email. If an invitation
-- token leaked (shared URL, referrer, logs, MITM during the 7-day window), any
-- registered user could redeem it and escalate to the invited role (e.g. admin).
--
-- Fix: require the invitation email to match the caller's auth.users.email when
-- the invitation carries an email (it is NOT NULL per schema, but we guard
-- defensively). Body reproduced from 20241205_admin_roles.sql with the email
-- check added.

CREATE OR REPLACE FUNCTION accept_invitation(p_token TEXT)
RETURNS BOOLEAN AS $$
DECLARE
    v_invitation RECORD;
    v_caller_email TEXT;
BEGIN
    -- Find valid invitation
    SELECT * INTO v_invitation
    FROM user_invitations
    WHERE token = p_token
      AND expires_at > NOW()
      AND accepted_at IS NULL;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Invalid or expired invitation';
    END IF;

    -- Bind invitation to the caller's email (defence-in-depth against token leaks)
    SELECT email INTO v_caller_email FROM auth.users WHERE id = auth.uid();
    IF v_caller_email IS NULL THEN
        RAISE EXCEPTION 'Could not resolve caller email';
    END IF;
    IF v_invitation.email IS NOT NULL AND v_invitation.email <> v_caller_email THEN
        RAISE EXCEPTION 'Invitation is for a different email address';
    END IF;

    -- Mark as accepted
    UPDATE user_invitations
    SET accepted_at = NOW()
    WHERE token = p_token;

    -- Auto-approve user with role
    INSERT INTO user_approvals (user_id, approved, role, approved_by)
    VALUES (auth.uid(), true, v_invitation.role, v_invitation.invited_by)
    ON CONFLICT (user_id) DO UPDATE
    SET approved = true, role = v_invitation.role;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
