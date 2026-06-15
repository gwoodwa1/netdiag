FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /netdiag ./cmd/netdiag

FROM alpine:3.22
RUN apk add --no-cache font-dejavu rsvg-convert \
    && addgroup -S netdiag \
    && adduser -S -G netdiag netdiag \
    && mkdir /work \
    && chown netdiag:netdiag /work
COPY --from=build --chown=netdiag:netdiag /netdiag /netdiag
WORKDIR /work
USER netdiag
ENTRYPOINT ["/netdiag"]
