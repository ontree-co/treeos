#!/usr/bin/env bash
set -euo pipefail

WEBUI_URL="${WEBUI_URL:-http://localhost:3001}"
ADMIN_EMAIL="${TREEOS_OPENWEBUI_ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${TREEOS_OPENWEBUI_ADMIN_PASSWORD:-password123}"
ADMIN_NAME="${TREEOS_OPENWEBUI_ADMIN_NAME:-TreeOS Admin}"
MODEL_NAME="${MODEL_NAME:-gemma3:270m}"

echo "OpenWebUI E2E"
echo "- WEBUI_URL: ${WEBUI_URL}"
echo "- MODEL_NAME: ${MODEL_NAME}"

if ! command -v curl >/dev/null 2>&1; then
  echo "FAIL: curl is required"
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "FAIL: python3 is required for JSON parsing"
  exit 1
fi

if [[ -z "${TREEOS_OPENWEBUI_ADMIN_EMAIL:-}" || -z "${TREEOS_OPENWEBUI_ADMIN_PASSWORD:-}" ]]; then
  echo "FAIL: TREEOS_OPENWEBUI_ADMIN_EMAIL and TREEOS_OPENWEBUI_ADMIN_PASSWORD must be set"
  exit 1
fi

signin_payload=$(cat <<JSON
{"email":"${ADMIN_EMAIL}","password":"${ADMIN_PASSWORD}"}
JSON
)

signin_resp=$(curl -fsSL -X POST "${WEBUI_URL}/api/v1/auths/signin" \
  -H "Content-Type: application/json" \
  -d "${signin_payload}")

token=$(python3 - <<'PY'
import json, sys
data = json.load(sys.stdin)
token = data.get("token")
if not token:
    raise SystemExit("missing token in signin response")
print(token)
PY
<<< "${signin_resp}")

echo "Authenticated as ${ADMIN_EMAIL}"

first_prompt="Say 'ready' and remember token X123."
first_payload=$(cat <<JSON
{"model":"${MODEL_NAME}","messages":[{"role":"user","content":"${first_prompt}"}]}
JSON
)

first_resp=$(curl -fsSL -X POST "${WEBUI_URL}/api/chat/completions" \
  -H "Authorization: Bearer ${token}" \
  -H "Content-Type: application/json" \
  -d "${first_payload}")

assistant_reply=$(python3 - <<'PY'
import json, sys
data = json.load(sys.stdin)
choices = data.get("choices") or []
message = choices[0].get("message", {}) if choices else {}
content = message.get("content", "")
print(content)
PY
<<< "${first_resp}")

if [[ -z "${assistant_reply}" ]]; then
  echo "FAIL: empty assistant reply"
  exit 1
fi

follow_prompt="What token did I ask you to remember?"
follow_payload=$(cat <<JSON
{"model":"${MODEL_NAME}","messages":[{"role":"user","content":"${first_prompt}"},{"role":"assistant","content":"${assistant_reply}"},{"role":"user","content":"${follow_prompt}"}]}
JSON
)

follow_resp=$(curl -fsSL -X POST "${WEBUI_URL}/api/chat/completions" \
  -H "Authorization: Bearer ${token}" \
  -H "Content-Type: application/json" \
  -d "${follow_payload}")

follow_reply=$(python3 - <<'PY'
import json, sys
data = json.load(sys.stdin)
choices = data.get("choices") or []
message = choices[0].get("message", {}) if choices else {}
content = message.get("content", "")
print(content)
PY
<<< "${follow_resp}")

if [[ "${follow_reply}" != *"X123"* ]]; then
  echo "FAIL: follow-up reply did not include expected token"
  exit 1
fi

chat_payload=$(cat <<JSON
{"chat":{"title":"TreeOS E2E","messages":[{"role":"user","content":"${first_prompt}"},{"role":"assistant","content":"${assistant_reply}"},{"role":"user","content":"${follow_prompt}"},{"role":"assistant","content":"${follow_reply}"}]}}
JSON
)

chat_resp=$(curl -fsSL -X POST "${WEBUI_URL}/api/v1/chats/new" \
  -H "Authorization: Bearer ${token}" \
  -H "Content-Type: application/json" \
  -d "${chat_payload}")

chat_id=$(python3 - <<'PY'
import json, sys
data = json.load(sys.stdin)
chat_id = data.get("id")
if not chat_id:
    raise SystemExit("missing chat id")
print(chat_id)
PY
<<< "${chat_resp}")

stored_chat=$(curl -fsSL -X GET "${WEBUI_URL}/api/v1/chats/${chat_id}" \
  -H "Authorization: Bearer ${token}")

python3 - <<'PY' "${stored_chat}"
import json, sys
data = json.loads(sys.argv[1])
chat = data.get("chat", {})
messages = chat.get("messages", [])
if not messages:
    raise SystemExit("chat messages missing")
contents = " ".join([m.get("content","") for m in messages])
if "X123" not in contents:
    raise SystemExit("expected token not found in stored chat")
PY

echo "OpenWebUI E2E succeeded"
