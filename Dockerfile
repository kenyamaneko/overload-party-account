FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY . .
RUN --mount=type=secret,id=COMMON_GO_MODULES_FETCH \
    git config --global url."https://x-access-token:$(cat /run/secrets/COMMON_GO_MODULES_FETCH)@github.com/kenyamaneko/".insteadOf "https://github.com/kenyamaneko/" && \
    GOPRIVATE=github.com/kenyamaneko/* go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /account ./cmd/server

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /account /app/account
EXPOSE 9005
ENTRYPOINT ["/app/account"]
