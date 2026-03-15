# Stage 1: Build frontend
FROM oven/bun:1 AS ui-builder
WORKDIR /app/ui
COPY ui/package.json ui/bun.lock ./
RUN bun install --frozen-lockfile --ignore-scripts
COPY ui/ ./
RUN bunx quasar prepare && bunx quasar build

# Stage 2: Build Go binary with embedded frontend
FROM golang:1.25 AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui-builder /app/ui/dist/spa/ ./internal/web/html/
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X 'github.com/mxcd/rabbithole/internal/util.Version=${VERSION}'" -o /server ./cmd/server

# Stage 3: Minimal runtime
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go-builder /server /server
ENTRYPOINT ["/server"]
