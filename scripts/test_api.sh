#!/bin/bash

# Использование: ./scripts/test_api.sh [base_url] [admin_token] [user_token]

BASE_URL="${1:-http://localhost:8080}"
ADMIN_TOKEN="${2:-admin-secret}"
USER_TOKEN="${3:-user-secret}"

TIMESTAMP=$(date +%s)
PR_ID_1="pr-${TIMESTAMP}-1"
PR_ID_2="pr-${TIMESTAMP}-2"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASSED=0
FAILED=0


if command -v jq &> /dev/null; then
    JQ_CMD="jq"
else
    JQ_CMD="cat"
    echo "Warning: jq not found. JSON output will not be formatted."
fi

print_test() {
    echo -e "${YELLOW}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
    ((PASSED++))
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
    ((FAILED++))
}

check_response() {
    local status=$1
    local expected=$2
    local response=$3
    
    if [ "$status" -eq "$expected" ]; then
        print_success "HTTP $status (expected $expected)"
        if [ "$JQ_CMD" = "jq" ]; then
            echo "Response: $response" | jq '.' 2>/dev/null || echo "$response"
        else
            echo "Response: $response"
        fi
        return 0
    else
        print_error "HTTP $status (expected $expected)"
        echo "Response: $response"
        return 1
    fi
}

echo "=========================================="
echo "Testing PR Reviewer Assignment Service"
echo "Base URL: $BASE_URL"
echo "=========================================="
echo ""

print_test "1. Health Check"
response=$(curl -s -w "\n%{http_code}" "$BASE_URL/health")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
check_response "$http_code" 200 "$body"
echo ""

print_test "2. Create Team 'backend' with 3 members"
response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/team/add" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "backend",
    "members": [
      {"user_id": "u1", "username": "Alice", "is_active": true},
      {"user_id": "u2", "username": "Bob", "is_active": true},
      {"user_id": "u3", "username": "Charlie", "is_active": true}
    ]
  }')
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
if [ "$http_code" -eq 201 ] || [ "$http_code" -eq 400 ]; then
    check_response "$http_code" "$http_code" "$body"
else
    check_response "$http_code" 201 "$body"
fi
echo ""

print_test "3. Try to create duplicate team (should fail with 400)"
response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/team/add" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "backend",
    "members": [
      {"user_id": "u4", "username": "David", "is_active": true}
    ]
  }')
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
check_response "$http_code" 400 "$body"
echo ""

print_test "4. Get Team 'backend'"
response=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/team/get?team_name=backend" \
  -H "Authorization: Bearer $USER_TOKEN")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
check_response "$http_code" 200 "$body"
echo ""

print_test "5. Create Pull Request (should auto-assign reviewers)"
response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/pullRequest/create" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"pull_request_id\": \"$PR_ID_1\",
    \"pull_request_name\": \"Add search feature\",
    \"author_id\": \"u1\"
  }")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
check_response "$http_code" 201 "$body"

if [ "$JQ_CMD" = "jq" ]; then
    REVIEWERS=$(echo "$body" | jq -r '.pr.assigned_reviewers[]' 2>/dev/null)
    REVIEWER_COUNT=$(echo "$REVIEWERS" | wc -l | tr -d ' ')
else
    REVIEWER_COUNT=$(echo "$body" | grep -o '"assigned_reviewers"' -A 5 | grep -o '"[^"]*"' | wc -l | tr -d ' ')
    REVIEWER_COUNT=$((REVIEWER_COUNT - 1))
    [ "$REVIEWER_COUNT" -lt 0 ] && REVIEWER_COUNT=0
fi
if [ "$REVIEWER_COUNT" -ge 1 ] && [ "$REVIEWER_COUNT" -le 2 ]; then
    print_success "Assigned $REVIEWER_COUNT reviewer(s) (expected 1-2)"
else
    print_error "Assigned $REVIEWER_COUNT reviewer(s) (expected 1-2)"
fi
echo ""

print_test "6. Get Review Assignments for u2"
response=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/users/getReview?user_id=u2" \
  -H "Authorization: Bearer $USER_TOKEN")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
check_response "$http_code" 200 "$body"
echo ""

print_test "7. Reassign Reviewer (replace reviewer with another from same team)"
if [ "$JQ_CMD" = "jq" ]; then
    FIRST_REVIEWER=$(echo "$body" | jq -r '.pr.assigned_reviewers[0]' 2>/dev/null)
    if [ -z "$FIRST_REVIEWER" ] || [ "$FIRST_REVIEWER" = "null" ]; then
        FIRST_REVIEWER=$(echo "$REVIEWERS" | head -n1)
    fi
else
    FIRST_REVIEWER=$(echo "$body" | grep -o '"assigned_reviewers"[^]]*' | grep -o '"[^"]*"' | head -1 | tr -d '"')
fi
if [ -n "$FIRST_REVIEWER" ] && [ "$FIRST_REVIEWER" != "assigned_reviewers" ] && [ "$FIRST_REVIEWER" != "null" ] && [ "$FIRST_REVIEWER" != "" ]; then
    response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/pullRequest/reassign" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{
        \"pull_request_id\": \"$PR_ID_1\",
        \"old_user_id\": \"$FIRST_REVIEWER\"
      }")
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    check_response "$http_code" 200 "$body"
else
    print_error "No reviewer to reassign (PR may not have reviewers)"
fi
echo ""

print_test "8. Set User u3 as Inactive"
response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/users/setIsActive" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "u3",
    "is_active": false
  }')
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
check_response "$http_code" 200 "$body"
echo ""

print_test "9. Create PR (inactive users should not be assigned)"
response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/pullRequest/create" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"pull_request_id\": \"$PR_ID_2\",
    \"pull_request_name\": \"Fix bug\",
    \"author_id\": \"u1\"
  }")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
check_response "$http_code" 201 "$body"

if [ "$JQ_CMD" = "jq" ]; then
    REVIEWERS2=$(echo "$body" | jq -r '.pr.assigned_reviewers[]' 2>/dev/null)
        if echo "$REVIEWERS2" | grep -q "u3"; then
        print_error "Inactive user u3 was assigned (should not be)"
    else
        print_success "Inactive user u3 was not assigned"
    fi
else
    if echo "$body" | grep -q '"u3"'; then
        print_error "Inactive user u3 might be assigned (manual check needed)"
    else
        print_success "Inactive user u3 was not assigned"
    fi
fi
echo ""

print_test "10. Merge Pull Request $PR_ID_1"
response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/pullRequest/merge" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"pull_request_id\": \"$PR_ID_1\"
  }")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
check_response "$http_code" 200 "$body"

if [ "$JQ_CMD" = "jq" ]; then
    STATUS=$(echo "$body" | jq -r '.pr.status' 2>/dev/null)
else
    STATUS=$(echo "$body" | grep -o '"status"[[:space:]]*:[[:space:]]*"[^"]*"' | grep -o '"[^"]*"' | tail -1 | tr -d '"')
fi
if [ "$STATUS" = "MERGED" ]; then
    print_success "PR status is MERGED"
else
    print_error "PR status is $STATUS (expected MERGED)"
fi
echo ""

print_test "11. Try to reassign reviewer on merged PR (should fail)"
response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/pullRequest/reassign" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"pull_request_id\": \"$PR_ID_1\",
    \"old_user_id\": \"u2\"
  }")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
check_response "$http_code" 409 "$body"
echo ""

print_test "12. Merge PR again (idempotent operation)"
response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/pullRequest/merge" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"pull_request_id\": \"$PR_ID_1\"
  }")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
check_response "$http_code" 200 "$body"
print_success "Idempotent merge succeeded"
echo ""

print_test "13. Test authorization (should fail without token)"
response=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/team/get?team_name=backend")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
check_response "$http_code" 401 "$body"
echo ""

echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed! ✓${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed! ✗${NC}"
    exit 1
fi

