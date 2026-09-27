#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ ! -d services/quiz/frontend/node_modules ]]; then npm ci --prefix services/quiz/frontend; fi
if [[ -z "${JWT_SECRET:-}" ]]; then export JWT_SECRET="$(openssl rand -hex 32)"; fi
if [[ -z "${QUIZ_GRPC_SERVICE_TOKEN:-}" ]]; then export QUIZ_GRPC_SERVICE_TOKEN="$(openssl rand -hex 32)"; fi
export QUIZ_API_PROXY="http://127.0.0.1:${QUIZ_HTTP_PORT:-8080}"
quizgo_api_pid=''
quizgo_frontend_pid=''
cleanup() {
  if [[ -n "$quizgo_api_pid" ]]; then kill "$quizgo_api_pid" 2>/dev/null || true; wait "$quizgo_api_pid" 2>/dev/null || true; fi
  if [[ -n "$quizgo_frontend_pid" ]]; then kill "$quizgo_frontend_pid" 2>/dev/null || true; wait "$quizgo_frontend_pid" 2>/dev/null || true; fi
  if [[ -n "${quizgo_bin_dir:-}" ]]; then
    if [[ -f "$quizgo_bin_dir/quiz-service" ]]; then rm -- "$quizgo_bin_dir/quiz-service"; fi
    rmdir -- "$quizgo_bin_dir"
  fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM
quizgo_bin_dir=$(mktemp -d)
go build -o "$quizgo_bin_dir/quiz-service" ./services/quiz/cmd/quiz-service
"$quizgo_bin_dir/quiz-service" &
quizgo_api_pid=$!
(cd services/quiz/frontend && exec node node_modules/vite/bin/vite.js --host 127.0.0.1) &
quizgo_frontend_pid=$!
while kill -0 "$quizgo_api_pid" 2>/dev/null && kill -0 "$quizgo_frontend_pid" 2>/dev/null; do sleep 1; done
exit 1
