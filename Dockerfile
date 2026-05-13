FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/system-agent-rag .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/system-agent-rag /usr/local/bin/system-agent-rag
ENTRYPOINT ["system-agent-rag"]
CMD ["-config", "/etc/system-agent-rag/config.yaml"]
