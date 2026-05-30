# ── builder ───────────────────────────────────────────────────────────
# VERSION is consumed by -ldflags to bake the semver tag into the binary.
ARG VERSION=dev
FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /usr/local/bin/anilistgen \
    -ldflags="-s -w -X main.version=${VERSION}" ./cmd/anilistgen

# ── runtime ───────────────────────────────────────────────────────────
# Distroless base image — no shell, no package manager, non-root user.
# Includes ca-certificates for HTTPS calls to AniList & MDBList.
FROM gcr.io/distroless/base-debian12:nonroot

LABEL org.opencontainers.image.source="https://github.com/calmcacil/anilistgen"
LABEL org.opencontainers.image.description="Seasonal anime list generator for MDBList from AniList"

COPY --from=builder /usr/local/bin/anilistgen /usr/local/bin/anilistgen

# Default search path for config file when mounted as a volume.
# Users can bind-mount their config at this path and pass -config /etc/anilistgen/anilistgen.yaml,
# or rely on env vars alone (no config file needed).
# The nonroot user (uid 65534) can read this path.
ENTRYPOINT ["/usr/local/bin/anilistgen"]
CMD ["daemon"]
