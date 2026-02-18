#!/bin/bash
# =============================================================================
# Wine Integration Test for Remote Desktop Agent
# Kører efter build for at verificere exe-filerne virker korrekt
#
# Usage: ./test-wine.sh [version]
#   version: f.eks. v2.72.1 (default: auto-detect fra builds/)
#
# Tests:
#   1. Binary validation (PE32+ format, version string)
#   2. Agent startup med authenticated JWT
#   3. Device registration i Supabase
#   4. TokenProvider + heartbeat
#   5. Screen capture init (GDI fallback)
#   6. OpenH264 encoder init
#   7. Session polling startet
#
# Exit codes: 0 = alle tests OK, 1 = fejl
# =============================================================================

set -uo pipefail
# Note: vi bruger IKKE set -e fordi check() håndterer fejl manuelt

# --- Config ---
BUILDS_DIR="$(cd "$(dirname "$0")" && pwd)/builds"
SUPABASE_HOST="192.168.1.92"
SUPABASE_PORT="8888"
SUPABASE_URL="http://${SUPABASE_HOST}:${SUPABASE_PORT}"
ANON_KEY="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE"
SERVICE_KEY="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJzZXJ2aWNlX3JvbGUiLAogICAgImlzcyI6ICJzdXBhYmFzZS1kZW1vIiwKICAgICJpYXQiOiAxNjQxNzY5MjAwLAogICAgImV4cCI6IDE3OTk1MzU2MDAKfQ.DaYlNEoUrrEn2Ig7tqibS-PHK5vgusbcbo7X36XVt4Q"
TEST_EMAIL="hansemand@gmail.com"
AGENT_TIMEOUT=15
CRED_FILE="${BUILDS_DIR}/.credentials"

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# --- Counters ---
PASS=0
FAIL=0
TOTAL=0

# --- Helpers ---
check() {
    local name="$1"
    local result="$2"  # 0 = pass, 1 = fail
    TOTAL=$((TOTAL + 1))
    if [ "$result" -eq 0 ]; then
        PASS=$((PASS + 1))
        echo -e "  ${GREEN}✅ PASS${NC}  $name"
    else
        FAIL=$((FAIL + 1))
        echo -e "  ${RED}❌ FAIL${NC}  $name"
    fi
}

cleanup() {
    # Fjern test credentials
    rm -f "$CRED_FILE"

    # Fjern test device fra databasen
    if [ -n "${TEST_DEVICE_ID:-}" ]; then
        ssh -o ConnectTimeout=5 dennis@${SUPABASE_HOST} \
            "docker exec supabase-db psql -U postgres -d postgres -q -c \
            \"DELETE FROM remote_devices WHERE device_id = '${TEST_DEVICE_ID}';\"" \
            2>/dev/null || true
    fi

    # Kill evt. baggrunds-wine processer
    pkill -f "remote-agent-console.*-console" 2>/dev/null || true
}

trap cleanup EXIT

# --- Detect version ---
if [ -n "${1:-}" ]; then
    VERSION="$1"
else
    # Auto-detect fra nyeste build
    LATEST=$(ls -t "$BUILDS_DIR"/remote-agent-console-v*.exe 2>/dev/null | head -1)
    if [ -z "$LATEST" ]; then
        echo -e "${RED}Ingen builds fundet i $BUILDS_DIR${NC}"
        exit 1
    fi
    VERSION=$(basename "$LATEST" | sed 's/remote-agent-console-\(v[0-9.]*\)\.exe/\1/')
fi

AGENT_EXE="${BUILDS_DIR}/remote-agent-console-${VERSION}.exe"
CONTROLLER_EXE="${BUILDS_DIR}/controller-${VERSION}.exe"
AGENT_GUI_EXE="${BUILDS_DIR}/remote-agent-${VERSION}.exe"

echo -e "${CYAN}============================================="
echo "  Wine Integration Test - ${VERSION}"
echo "=============================================${NC}"
echo ""

# =============================================================================
# TEST 1: Binary Validation
# =============================================================================
echo -e "${YELLOW}--- Binary Validation ---${NC}"

# Agent console exe
if [ -f "$AGENT_EXE" ]; then
    FILE_INFO=$(file "$AGENT_EXE")
    echo "$FILE_INFO" | grep -q "PE32+" 2>/dev/null
    check "Agent console: PE32+ format" $?

    AGENT_SIZE=$(stat -c%s "$AGENT_EXE")
    [ "$AGENT_SIZE" -gt 10000000 ]  # > 10MB
    check "Agent console: størrelse OK ($(numfmt --to=iec $AGENT_SIZE))" $?
else
    check "Agent console: fil findes" 1
fi

# Controller exe
if [ -f "$CONTROLLER_EXE" ]; then
    file "$CONTROLLER_EXE" | grep -q "PE32+" 2>/dev/null
    check "Controller: PE32+ format" $?

    CTRL_SIZE=$(stat -c%s "$CONTROLLER_EXE")
    [ "$CTRL_SIZE" -gt 10000000 ]
    check "Controller: størrelse OK ($(numfmt --to=iec $CTRL_SIZE))" $?
else
    check "Controller: fil findes" 1
fi

# Agent GUI exe
if [ -f "$AGENT_GUI_EXE" ]; then
    file "$AGENT_GUI_EXE" | grep -q "PE32+ executable (GUI)" 2>/dev/null
    check "Agent GUI: PE32+ GUI format" $?
else
    check "Agent GUI: fil findes" 1
fi

# Version string embedded (strings | grep kan give SIGPIPE, brug fil)
strings "$AGENT_EXE" 2>/dev/null > /tmp/agent_strings.txt
grep -q "$VERSION" /tmp/agent_strings.txt 2>/dev/null
check "Agent: version string '${VERSION}' indlejret" $?

strings "$CONTROLLER_EXE" 2>/dev/null > /tmp/ctrl_strings.txt
grep -q "$VERSION" /tmp/ctrl_strings.txt 2>/dev/null
check "Controller: version string '${VERSION}' indlejret" $?
rm -f /tmp/agent_strings.txt /tmp/ctrl_strings.txt

# =============================================================================
# TEST 2: Wine --help output
# =============================================================================
echo ""
echo -e "${YELLOW}--- Wine Startup Test ---${NC}"

# Check wine is available
if ! command -v wine-stable &>/dev/null && ! command -v wine &>/dev/null; then
    echo -e "  ${RED}⚠️  Wine ikke installeret - springer runtime tests over${NC}"
    echo ""
    echo -e "${YELLOW}--- Resultat ---${NC}"
    echo -e "  Tests: ${TOTAL}  Pass: ${GREEN}${PASS}${NC}  Fail: ${RED}${FAIL}${NC}"
    [ "$FAIL" -eq 0 ] && exit 0 || exit 1
fi

WINE_CMD=$(command -v wine-stable 2>/dev/null || command -v wine)

# Test --help output
HELP_OUTPUT=$(WINEDEBUG=-all timeout 10 "$WINE_CMD" "$AGENT_EXE" -help 2>&1 || true)
echo "$HELP_OUTPUT" | grep -q "Remote Desktop Agent"
check "Wine: agent starter og viser help" $?

echo "$HELP_OUTPUT" | grep -q "$VERSION"
check "Wine: korrekt version i help output" $?

# =============================================================================
# TEST 3: Supabase Connectivity
# =============================================================================
echo ""
echo -e "${YELLOW}--- Supabase Connectivity ---${NC}"

# Check Supabase is reachable
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 \
    "${SUPABASE_URL}/rest/v1/" \
    -H "apikey: ${ANON_KEY}" 2>/dev/null || echo "000")
[ "$HTTP_CODE" != "000" ]
check "Supabase API tilgængelig (HTTP $HTTP_CODE)" $?

if [ "$HTTP_CODE" = "000" ]; then
    echo -e "  ${RED}⚠️  Supabase ikke tilgængelig - springer auth tests over${NC}"
    echo ""
    echo -e "${YELLOW}--- Resultat ---${NC}"
    echo -e "  Tests: ${TOTAL}  Pass: ${GREEN}${PASS}${NC}  Fail: ${RED}${FAIL}${NC}"
    [ "$FAIL" -eq 0 ] && exit 0 || exit 1
fi

# Generate JWT token for test user
OTP=$(curl -s "${SUPABASE_URL}/auth/v1/admin/generate_link" \
    -H "apikey: ${SERVICE_KEY}" \
    -H "Authorization: Bearer ${SERVICE_KEY}" \
    -H "Content-Type: application/json" \
    -d "{\"type\":\"magiclink\",\"email\":\"${TEST_EMAIL}\"}" \
    | python3 -c "import sys,json; print(json.load(sys.stdin).get('email_otp',''))" 2>/dev/null)

[ -n "$OTP" ]
check "JWT: magiclink OTP genereret" $?

AUTH_RESULT=$(curl -s "${SUPABASE_URL}/auth/v1/verify" \
    -H "apikey: ${ANON_KEY}" \
    -H "Content-Type: application/json" \
    -d "{\"type\":\"magiclink\",\"token\":\"${OTP}\",\"email\":\"${TEST_EMAIL}\"}" 2>/dev/null)

ACCESS_TOKEN=$(echo "$AUTH_RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('access_token',''))" 2>/dev/null || echo "")
REFRESH_TOKEN=$(echo "$AUTH_RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('refresh_token',''))" 2>/dev/null || echo "")
EXPIRES_AT=$(echo "$AUTH_RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('expires_at',0))" 2>/dev/null || echo "0")
USER_ID=$(echo "$AUTH_RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('user',{}).get('id',''))" 2>/dev/null || echo "")

[ -n "$ACCESS_TOKEN" ] && [ "$ACCESS_TOKEN" != "" ]
check "JWT: access token modtaget" $?

# =============================================================================
# TEST 4: Agent Full Startup (Wine + Console Mode)
# =============================================================================
echo ""
echo -e "${YELLOW}--- Agent Integration Test ---${NC}"

if [ -z "$ACCESS_TOKEN" ] || [ "$ACCESS_TOKEN" = "" ]; then
    echo -e "  ${RED}⚠️  Ingen JWT token - springer integration test over${NC}"
else
    # Write credentials file
    cat > "$CRED_FILE" << CREDEOF
{
  "email": "${TEST_EMAIL}",
  "access_token": "${ACCESS_TOKEN}",
  "refresh_token": "${REFRESH_TOKEN}",
  "user_id": "${USER_ID}",
  "expires_at": ${EXPIRES_AT}
}
CREDEOF

    # Run agent with Wine in console mode
    AGENT_OUTPUT=$(WINEDEBUG=-all timeout "$AGENT_TIMEOUT" \
        "$WINE_CMD" "$AGENT_EXE" -console 2>&1 || true)

    # Parse expected log lines
    echo "$AGENT_OUTPUT" | grep -q "Logget ind som: ${TEST_EMAIL}"
    check "Auth: logget ind som ${TEST_EMAIL}" $?

    echo "$AGENT_OUTPUT" | grep -q "TokenProvider oprettet"
    check "Auth: TokenProvider oprettet (authenticated API)" $?

    echo "$AGENT_OUTPUT" | grep -q "Device registered"
    check "Device: registreret i Supabase" $?

    # Extract device ID from output
    TEST_DEVICE_ID=$(echo "$AGENT_OUTPUT" | grep -oP "Device ID: \K[a-z0-9_]+" | head -1 || echo "")

    echo "$AGENT_OUTPUT" | grep -q "Screen capturer initialized"
    check "Capture: screen capturer initialiseret" $?

    echo "$AGENT_OUTPUT" | grep -q "OpenH264.*initialized"
    check "Encoder: OpenH264 loaded og initialiseret" $?

    echo "$AGENT_OUTPUT" | grep -q "Session polling started"
    check "Signaling: session polling startet" $?

    echo "$AGENT_OUTPUT" | grep -q "Agent kører"
    check "Startup: agent kører og venter på forbindelser" $?

    # Verify device in database
    if [ -n "$TEST_DEVICE_ID" ]; then
        DB_RESULT=$(ssh -o ConnectTimeout=5 dennis@${SUPABASE_HOST} \
            "docker exec supabase-db psql -U postgres -d postgres -t -c \
            \"SELECT is_online, owner_id FROM remote_devices WHERE device_id = '${TEST_DEVICE_ID}';\"" \
            2>/dev/null || echo "")

        echo "$DB_RESULT" | grep -q "t.*${USER_ID}"
        check "Database: device online med korrekt owner_id" $?
    else
        check "Database: device ID fundet i output" 1
    fi
fi

# =============================================================================
# TEST 5: RLS Verification (anon blocked)
# =============================================================================
echo ""
echo -e "${YELLOW}--- RLS Policy Test ---${NC}"

# Anon should get empty results
ANON_DEVICES=$(curl -s "${SUPABASE_URL}/rest/v1/remote_devices?select=device_id" \
    -H "apikey: ${ANON_KEY}" \
    -H "Authorization: Bearer ${ANON_KEY}" 2>/dev/null)
[ "$ANON_DEVICES" = "[]" ]
check "RLS: anon BLOKERET fra remote_devices" $?

ANON_SESSIONS=$(curl -s "${SUPABASE_URL}/rest/v1/webrtc_sessions?select=session_id" \
    -H "apikey: ${ANON_KEY}" \
    -H "Authorization: Bearer ${ANON_KEY}" 2>/dev/null)
[ "$ANON_SESSIONS" = "[]" ]
check "RLS: anon BLOKERET fra webrtc_sessions" $?

ANON_SIGNALING=$(curl -s "${SUPABASE_URL}/rest/v1/session_signaling?select=id" \
    -H "apikey: ${ANON_KEY}" \
    -H "Authorization: Bearer ${ANON_KEY}" 2>/dev/null)
[ "$ANON_SIGNALING" = "[]" ]
check "RLS: anon BLOKERET fra session_signaling" $?

# Quick Support should still work for anon
ANON_SUPPORT=$(curl -s "${SUPABASE_URL}/rest/v1/support_sessions?select=id&limit=1" \
    -H "apikey: ${ANON_KEY}" \
    -H "Authorization: Bearer ${ANON_KEY}" 2>/dev/null)
echo "$ANON_SUPPORT" | grep -qv "error"
check "RLS: anon har adgang til Quick Support" $?

# =============================================================================
# Results
# =============================================================================
echo ""
echo -e "${CYAN}============================================="
echo "  RESULTATER"
echo "=============================================${NC}"
echo -e "  Tests:  ${TOTAL}"
echo -e "  Pass:   ${GREEN}${PASS}${NC}"
echo -e "  Fail:   ${RED}${FAIL}${NC}"
echo ""

if [ "$FAIL" -eq 0 ]; then
    echo -e "  ${GREEN}✅ ALLE TESTS BESTÅET${NC}"
    exit 0
else
    echo -e "  ${RED}❌ ${FAIL} TEST(S) FEJLET${NC}"
    exit 1
fi
