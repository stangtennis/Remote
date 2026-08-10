# TURN Secret Rotation Runbook

## Situation

The static TURN shared secret `REDACTED_TURN_SECRET` is present in git
history (6 commits introduced or touched it). It is **no longer used by
active code** (current agent/dashboard read `TURN_SECRET` from the
environment / Supabase secrets), but anyone with read access to the repo can
recover it from history and, while it is still configured on the running
coturn/TURN infra, mint valid TURN credentials.

Confirmed exposure:

```bash
git log --all --oneline -S 'REDACTED_TURN_SECRET'
# 6 commits
```

Two actions are required, in this order:

1. **Rotate the secret on the infrastructure** (mandatory, low risk).
2. **Decide on git-history rewrite** (optional, high risk, irreversible — see
   the explicit gate at the end).

---

## Part 1 — Rotate the running TURN secret (do this first)

### 1.1 Generate a new secret

```bash
# Strong, random shared secret (URL-safe, 32 bytes).
NEW_TURN_SECRET="$(openssl rand -base64 32 | tr -d '/+=' | cut -c1-32)"
echo "$NEW_TURN_SECRET"  # copy for the next steps
```

### 1.2 Update coturn (self-hosted fallback)

The coturn container reads its config from the compose/env. Example for the
Docker stack on the Ubuntu host:

```bash
# On linux1 (192.168.1.92) — adjust paths to your coturn stack
nano ~/coturn/turnserver.conf      # set: static-auth-secret=<NEW_TURN_SECRET>
# or set TURN_SECRET in the compose env, then:
cd ~/coturn && docker compose up -d --force-recreate coturn
docker logs --tail 50 coturn        # confirm it bound and uses the new secret
```

### 1.3 Update Supabase edge-function secrets

`support-signal` (action `turn`) and `turn-credentials` read `TURN_SECRET`
from the function env:

```bash
# From the Supabase dashboard or CLI:
supabase secrets set TURN_SECRET=<NEW_TURN_SECRET>
# Redeploy affected functions so they pick up the new value:
supabase functions deploy support-signal
supabase functions deploy turn-credentials
```

### 1.4 Update the agent update flow (if TURN_SECRET is baked anywhere)

Confirm no tracked file references the old secret:

```bash
git grep -n 'REDACTED_TURN_SECRET'      # should be empty in active code
```

### 1.5 Verify end-to-end

- Open the dashboard → connect to a device that requires relay.
- Confirm a `relay` ICE candidate is generated and the connection establishes.
- `docs/turn-test.html` → "Test TURN Server" shows RELAY candidates.

### 1.6 Rotate again if anything else leaked

If `ULTIMATE_GUIDE_LOCAL_PASSWORDS.md` or `.env` leaked other material, rotate
those too. This runbook only covers the TURN shared secret.

---

## Part 2 — Git history rewrite (OPTIONAL, IRREVERSIBLE — requires explicit confirmation)

Rewriting history force-pushes a new `main` and invalidates every existing
clone and fork. It is **not** required once the secret is rotated on the infra
(the exposed value no longer grants access). It only removes the embarrassing
historical copy. Do this only if you accept the collateral damage.

**Gate: do NOT run any of the following without an explicit "yes, rewrite
history" from the repo owner.** This document describing the procedure is not
that consent.

Recommended tool: `git-filter-repo` (replaces the old `git filter-branch`).

```bash
# 0. Make a full backup first.
git clone --mirror <repo> remote-backup.git

# 1. Install git-filter-repo.
pip install git-filter-repo

# 2. In a fresh clone, replace the old secret everywhere in history.
git clone <repo> rd-rewrite && cd rd-rewrite
echo 'REDACTED_TURN_SECRET==>REDACTED_TURN_SECRET' > replacements.txt
git filter-repo --replace-text replacements.txt

# 3. Inspect the result.
git log --all -S 'REDACTED_TURN_SECRET'   # must be empty
git log --all -S 'REDACTED_TURN_SECRET'      # shows where it was

# 4. Force-push (IRREVERSIBLE — coordinates with any collaborators first).
git push --force origin --all
git push --force origin --tags

# 5. Bump the repo's GC so the old blobs become unreachable sooner.
# (GitHub: also consider "Repository settings → danger zone" or contact
# support to purge cached objects if the secret is sensitive enough.)
```

After a rewrite, every contributor must re-clone (the old history is dead).
GitHub Actions, release tags, and any pinned SHAs referencing old commits
break and must be repinned.

---

## Decision record

- Rotate infra secret: **REQUIRED** — do Part 1 now.
- Rewrite history: **DEFERRED** — pending explicit owner confirmation.
  Rationale: once Part 1 is done, the leaked secret is inert; the rewrite's
  cost (broken clones/tags/releases) outweighs the benefit.
