#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo_root"

load_project=${QUIZGO_LOAD_COMPOSE_PROJECT:-quizgo-load-$$}
export DOMAIN=${DOMAIN:-localhost}
export POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-load-postgres-password}
export QUIZ_GRPC_SERVICE_TOKEN=${QUIZ_GRPC_SERVICE_TOKEN:-load-service-token-0123456789abcdef}
export JWT_SECRET=${JWT_SECRET:-load-jwt-secret-0123456789abcdef0123456789abcdef}
export FRONTEND_PORT=${QUIZGO_LOAD_FRONTEND_PORT:-13001}
export GAME_PORT=${QUIZGO_LOAD_GAME_PORT:-18082}
export GAME_INSTANCE_ID=${GAME_INSTANCE_ID:-game-load}
export GAME_INTERNAL_URL=${GAME_INTERNAL_URL:-http://game:8081}

compose=(docker compose --project-name "$load_project" --file "$repo_root/compose.yaml")
cleanup() {
  if [[ "${QUIZGO_LOAD_KEEP_STACK:-0}" != "1" ]]; then
    "${compose[@]}" down --volumes --remove-orphans
  fi
}
trap cleanup EXIT
trap 'exit 130' INT TERM

"${compose[@]}" up --detach --build --wait postgres redis quiz game frontend

go run ./services/game/cmd/ws-load \
  -http-url "http://127.0.0.1:${FRONTEND_PORT}" \
  -ws-url "ws://127.0.0.1:${GAME_PORT}/ws" \
  "$@"
