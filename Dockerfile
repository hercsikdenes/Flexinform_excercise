# Shared source stage for containerized tests and the production build.
FROM golang:1.27.1-alpine AS test

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

CMD ["go", "test", "./..."]

# Production application builder.
FROM test AS builder

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

# The final image contains only runtime assets, the compiled application, and a
# non-root user; the Go toolchain remains in the builder/test stages.
FROM alpine:3.20 AS runtime

RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=builder --chown=app:app /out/api /usr/local/bin/api
COPY --chown=app:app migrations /app/migrations
COPY --chown=app:app data /app/data

USER app
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/api"]
