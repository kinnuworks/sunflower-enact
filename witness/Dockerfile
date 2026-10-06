FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/witness .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/witness /witness
ENTRYPOINT ["/witness"]
