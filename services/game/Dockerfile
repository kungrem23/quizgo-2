# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY gen/ ./gen/
COPY services/game/cmd/ ./services/game/cmd/
COPY services/game/internal/ ./services/game/internal/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/game-service ./services/game/cmd/game-service

FROM alpine:3.23
RUN apk add --no-cache ca-certificates \
    && addgroup -S quizgo && adduser -S -G quizgo quizgo
COPY --from=build /out/game-service /usr/local/bin/game-service
USER quizgo
ENV GAME_HTTP_PORT=8081
EXPOSE 8081
ENTRYPOINT ["game-service"]
