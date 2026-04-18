# Dockerfile — Seam Core operator (distroless).
#
# Seam Core is a long-running Deployment in seam-system. It owns the
# InfrastructureLineageIndex CRD and the DSNSReconciler for the Domain Semantic
# Name Service. Distroless: zero attack surface. INV-022.
# seam-core-schema.md §6.

FROM golang:1.25 AS builder
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /bin/seam-core \
    ./cmd/seam-core

FROM gcr.io/distroless/base:nonroot
COPY --from=builder /bin/seam-core /usr/local/bin/seam-core

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/seam-core"]
