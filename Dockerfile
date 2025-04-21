FROM golang:1.24 AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /drand-poc ./cmd/server

FROM gcr.io/distroless/static
ENV BASE_DOMAIN=http://localhost
COPY --from=builder /drand-poc /drand-poc
COPY frontend /frontend
ENTRYPOINT ["/drand-poc"]
