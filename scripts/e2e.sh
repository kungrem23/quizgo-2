#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo_root"

e2e_project=${QUIZGO_E2E_COMPOSE_PROJECT:-quizgo-e2e-$$}
export DOMAIN=${DOMAIN:-localhost}
export POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-e2e-postgres-password}
export QUIZ_GRPC_SERVICE_TOKEN=${QUIZ_GRPC_SERVICE_TOKEN:-e2e-service-token-0123456789abcdef}
export JWT_SECRET=${JWT_SECRET:-e2e-jwt-secret-0123456789abcdef0123456789abcdef}
export FRONTEND_PORT=${QUIZGO_E2E_FRONTEND_PORT:-13000}
export GAME_PORT=${QUIZGO_E2E_GAME_PORT:-18081}
export GAME_INSTANCE_ID=${GAME_INSTANCE_ID:-game-e2e}
export GAME_INTERNAL_URL=${GAME_INTERNAL_URL:-http://game:8081}

compose=(docker compose --project-name "$e2e_project" --file "$repo_root/compose.yaml")
cleanup() {
  if [[ "${QUIZGO_E2E_KEEP_STACK:-0}" != "1" ]]; then
    "${compose[@]}" down --volumes --remove-orphans
  fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM

"${compose[@]}" up --detach --build --wait postgres redis quiz game frontend

export QUIZGO_E2E_HTTP_URL="http://127.0.0.1:${FRONTEND_PORT}"
export QUIZGO_E2E_GAME_URL="http://127.0.0.1:${GAME_PORT}"
export QUIZGO_E2E_RESTART_GAME=1
export QUIZGO_E2E_COMPOSE_PROJECT="$e2e_project"
export QUIZGO_E2E_COMPOSE_FILE="$repo_root/compose.yaml"

go test -count=1 -timeout=2m -v ./services/game/tests/e2e
