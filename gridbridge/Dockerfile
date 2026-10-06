FROM golang:1.27 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gridbridge .

FROM gcr.io/distroless/static
COPY --from=build /out/gridbridge /gridbridge
ENTRYPOINT ["/gridbridge"]
