FROM golang:1.26-alpine AS build
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
    -o /netdiag ./cmd/netdiag

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
