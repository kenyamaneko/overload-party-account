FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY . .
RUN --mount=type=secret,id=COMMON_GO_MODULES_FETCH \
    --mount=type=secret,id=BATTLE_GO_MODULES_FETCH \
    --mount=type=secret,id=SHOP_GO_MODULES_FETCH \
    --mount=type=secret,id=SCENARIO_GO_MODULES_FETCH \
    git config --global url."https://x-access-token:$(cat /run/secrets/COMMON_GO_MODULES_FETCH)@github.com/kenyamaneko/overload-party-common".insteadOf "https://github.com/kenyamaneko/overload-party-common" && \
    git config --global url."https://x-access-token:$(cat /run/secrets/BATTLE_GO_MODULES_FETCH)@github.com/kenyamaneko/overload-party-battle".insteadOf "https://github.com/kenyamaneko/overload-party-battle" && \
    git config --global url."https://x-access-token:$(cat /run/secrets/SHOP_GO_MODULES_FETCH)@github.com/kenyamaneko/overload-party-shop".insteadOf "https://github.com/kenyamaneko/overload-party-shop" && \
    git config --global url."https://x-access-token:$(cat /run/secrets/SCENARIO_GO_MODULES_FETCH)@github.com/kenyamaneko/overload-party-scenario".insteadOf "https://github.com/kenyamaneko/overload-party-scenario" && \
    GOPRIVATE=github.com/kenyamaneko/* go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /account ./cmd/server

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /account /app/account
EXPOSE 9005
ENTRYPOINT ["/app/account"]
