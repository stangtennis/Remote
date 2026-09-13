// Pure authorization rules for device-enrollment token creation.
//
// Extracted from index.ts so the rules can be unit tested with `deno test`
// without starting the HTTP server. This module handles no secrets and logs
// nothing.

export type EnrollmentPurpose = 'agent' | 'ai_support'

export type PurposeAuthDecision =
  | { ok: true; purpose: EnrollmentPurpose }
  | { ok: false; status: 400 | 403; error: string }

const ADMIN_ROLES = new Set(['admin', 'super_admin'])

/** True only for the roles allowed to mint ai_support enrollment tokens. */
export function isAdminRole(role: unknown): boolean {
  return typeof role === 'string' && ADMIN_ROLES.has(role)
}

/**
 * Decide whether the authenticated user's role may create an enrollment
 * token for the requested purpose.
 *
 * - purpose defaults to 'agent' when missing/null (backwards compatible).
 * - Unknown purposes are rejected with 400.
 * - purpose='ai_support' additionally requires an admin/super_admin role
 *   from the existing user_approvals row; approved non-admins get 403.
 */
export function evaluatePurposeAuthorization(
  requestedPurpose: unknown,
  role: unknown,
): PurposeAuthDecision {
  const purpose =
    requestedPurpose === undefined || requestedPurpose === null
      ? 'agent'
      : requestedPurpose

  if (purpose !== 'agent' && purpose !== 'ai_support') {
    return { ok: false, status: 400, error: 'Invalid purpose' }
  }

  if (purpose === 'ai_support' && !isAdminRole(role)) {
    return {
      ok: false,
      status: 403,
      error: 'Admin access required for ai_support enrollment',
    }
  }

  return { ok: true, purpose }
}
