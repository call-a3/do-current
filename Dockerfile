# syntax=docker/dockerfile:1

FROM golang:1.18-bullseye AS build
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY *.go ./
RUN go build -o /current

FROM gcr.io/distroless/base-debian11
WORKDIR /
COPY --from=build /current /current
USER nonroot:nonroot
ENTRYPOINT [ "/current" ]
