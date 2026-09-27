# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY gen/ ./gen/
COPY services/quiz/cmd/ ./services/quiz/cmd/
COPY services/quiz/internal/ ./services/quiz/internal/
COPY services/quiz/docs/ ./services/quiz/docs/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/quiz-service ./services/quiz/cmd/quiz-service

FROM alpine:3.23
RUN apk add --no-cache ca-certificates \
    && addgroup -S quizgo && adduser -S -G quizgo quizgo
COPY --from=build /out/quiz-service /usr/local/bin/quiz-service
USER quizgo
ENV QUIZ_HTTP_PORT=8080
EXPOSE 8080 9090
ENTRYPOINT ["quiz-service"]
