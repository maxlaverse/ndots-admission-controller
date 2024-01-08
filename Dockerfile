FROM golang:1.21 AS builder
WORKDIR /build/
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./ ./
RUN CGO_ENABLED=0 go build -mod=readonly -o ndots-admission-controller

FROM scratch
ENTRYPOINT ["/ndots-admission-controller"]
COPY --from=builder /build/ndots-admission-controller /ndots-admission-controller