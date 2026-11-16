# syntax=docker/dockerfile:1

##
## Build
##
FROM golang:1.27-trixie AS build

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY . ./

ARG BUILD_VERSION=development
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X 'github.com/Mario-F/cloudflare-dyndns/cmd.Version=${BUILD_VERSION}'" -o /cloudflare-dyndns

##
## Deploy
##
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /

COPY --from=build /cloudflare-dyndns /cloudflare-dyndns

USER nonroot:nonroot

ENTRYPOINT ["/cloudflare-dyndns"]
