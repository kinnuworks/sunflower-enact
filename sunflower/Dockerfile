FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/sunflower .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/sunflower /sunflower
ENTRYPOINT ["/sunflower"]
