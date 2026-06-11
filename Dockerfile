FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /netdiag ./cmd/netdiag

FROM alpine:3.22
RUN apk add --no-cache rsvg-convert
COPY --from=build /netdiag /netdiag
ENTRYPOINT ["/netdiag"]
