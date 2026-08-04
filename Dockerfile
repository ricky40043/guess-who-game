FROM golang:1.23-alpine AS builder
WORKDIR /src
ARG BUILD_VERSION=dev
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go test ./... && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags="-s -w -X main.buildVersion=${BUILD_VERSION}" \
      -o /out/guess-who-game .

FROM alpine:3.21
RUN adduser -D -H -u 10001 app
USER app
COPY --from=builder /out/guess-who-game /app/guess-who-game
EXPOSE 8080
ENV PORT=8080
ENTRYPOINT ["/app/guess-who-game"]
