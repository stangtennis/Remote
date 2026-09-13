// Unit tests for the pure purpose/role authorization rules used by the
// device-enrollment Edge Function. Run with:
//   deno test supabase/functions/device-enrollment/purpose_auth.test.ts

import { assertEquals } from 'https://deno.land/std@0.168.0/testing/asserts.ts'
import {
  evaluatePurposeAuthorization,
  isAdminRole,
  type PurposeAuthDecision,
} from './purpose_auth.ts'

function decisionOk(d: PurposeAuthDecision): d is { ok: true; purpose: 'agent' | 'ai_support' } {
  return d.ok
}

Deno.test('missing/null purpose defaults to agent for any approved role', () => {
  for (const purpose of [undefined, null]) {
    assertEquals(evaluatePurposeAuthorization(purpose, 'user'), { ok: true, purpose: 'agent' })
    assertEquals(evaluatePurposeAuthorization(purpose, null), { ok: true, purpose: 'agent' })
  }
})

Deno.test('purpose=agent stays allowed for approved non-admins', () => {
  const d = evaluatePurposeAuthorization('agent', 'user')
  assertEquals(decisionOk(d) && d.purpose === 'agent', true)
})

Deno.test('purpose=ai_support is rejected (403) for approved non-admins', () => {
  for (const role of ['user', 'some_other_role', '', null, undefined, 42, {}]) {
    const d = evaluatePurposeAuthorization('ai_support', role)
    assertEquals(
      decisionOk(d),
      false,
      `role ${JSON.stringify(role)} must not be allowed to mint ai_support tokens`,
    )
    assertEquals(d.ok === false && d.status, 403)
  }
})

Deno.test('purpose=ai_support is allowed for admin and super_admin', () => {
  for (const role of ['admin', 'super_admin']) {
    const d = evaluatePurposeAuthorization('ai_support', role)
    assertEquals(decisionOk(d) && d.purpose === 'ai_support', true)
  }
})

Deno.test('unknown or non-string purposes are rejected with 400', () => {
  for (const purpose of ['bogus', '', 'AGENT', 'Agent', 1, {}, ['agent'], true]) {
    const d = evaluatePurposeAuthorization(purpose, 'admin')
    assertEquals(d.ok === false && d.status, 400, `purpose ${JSON.stringify(purpose)} must be 400`)
  }
})

Deno.test('isAdminRole accepts only exact admin/super_admin strings', () => {
  assertEquals(isAdminRole('admin'), true)
  assertEquals(isAdminRole('super_admin'), true)
  assertEquals(isAdminRole('Admin'), false)
  assertEquals(isAdminRole('user'), false)
  assertEquals(isAdminRole(null), false)
  assertEquals(isAdminRole(undefined), false)
  assertEquals(isAdminRole(123), false)
})
