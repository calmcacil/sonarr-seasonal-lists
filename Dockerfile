FROM golang:1.24-alpine AS builder
ARG VERSION=dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /usr/local/bin/anilistgen -ldflags="-s -w -X main.version=${VERSION}" ./cmd/anilistgen

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /usr/local/bin/anilistgen /usr/local/bin/anilistgen

ENTRYPOINT ["anilistgen"]
CMD ["daemon"]
