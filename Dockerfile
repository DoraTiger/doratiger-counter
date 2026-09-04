FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
ARG GOPROXY=https://proxy.golang.org,direct
RUN GOPROXY=${GOPROXY} go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -mod=readonly -trimpath \
    -ldflags "-s -w -X github.com/DoraTiger/doratiger-counter/internal/version.BuildVersion=${VERSION} -X github.com/DoraTiger/doratiger-counter/internal/version.BuildRepo=https://github.com/DoraTiger/doratiger-counter" \
    -o /counter ./cmd/doratiger-counter

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S counter \
    && adduser -S -G counter -h /app counter \
    && install -d -o counter -g counter /app/data
WORKDIR /app
COPY --from=builder /counter /usr/local/bin/doratiger-counter
COPY --chown=counter:counter config.toml /app/config.toml
USER counter
VOLUME ["/app/data"]
EXPOSE 8080
ENTRYPOINT ["doratiger-counter"]
CMD ["serve", "--config", "/app/config.toml"]
