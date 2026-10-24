# Build stage: compile a static binary with no C toolchain (pure-Go SQLite).
FROM golang:1.26 AS build

WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is off, so the result runs on a bare base image. Version info is injected
# with -ldflags, exactly as `make build` does.
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X 'Trove/internal/app.Version=${VERSION}' -X 'Trove/internal/app.Commit=${COMMIT}' -X 'Trove/internal/app.BuildDate=${BUILD_DATE}'" \
    -o /out/trove ./cmd/server

# An empty data directory with the nonroot user's ownership, copied into the
# runtime image so a fresh volume is writable.
RUN mkdir -p /out/data

# Runtime stage: no shell, no package manager, a single binary.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/trove /usr/local/bin/trove

# The data directory is a volume; content files live under it (spec storage).
# It is created in the image with the nonroot user's ownership so a fresh named
# or anonymous volume is writable. A bind mount must be chowned on the host
# (see the operations guide).
COPY --from=build --chown=nonroot:nonroot /out/data /data
VOLUME ["/data"]

EXPOSE 8080

# Run as the nonroot user provided by the base image.
USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/trove"]
CMD ["--listen", "0.0.0.0:8080", "--data-dir", "/data"]
