# QuizGo

QuizGo is an online realtime quiz platform.

## Architecture
- Go microservices
- game-service owns realtime game execution
- CRUD service provides quiz data over gRPC
- Redis stores game state
- WebSocket is used for host/player realtime communication
- each active room is managed by an actor loop

## Game service rules
- Preserve actor-loop architecture.
- Do not introduce shared mutable room state outside the actor.
- Persist state required for reconnect/recovery in Redis.
- Keep WebSocket protocol backward-compatible unless explicitly requested.
- Add tests for new game-state transitions.
- Avoid unrelated refactoring.

## Workflow
Before implementing:
1. Inspect relevant existing code.
2. Reuse existing abstractions.
3. Make the smallest reasonable change.

After implementing:
1. Run relevant tests.
2. Report what changed.
3. Report unresolved issues.

Do not implement functionality outside the requested task.