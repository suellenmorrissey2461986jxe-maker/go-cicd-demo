FROM golang:1.26.7-alpine AS builder

ARG VERSION=dev

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/app .

FROM scratch

COPY --from=builder /out/app /app

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/app"]
