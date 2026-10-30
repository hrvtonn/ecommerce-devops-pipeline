FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ecommerce-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/ecommerce-api /app/ecommerce-api
ENV APP_ENV=production \
    PORT=3000
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD ["/app/ecommerce-api", "healthcheck"]
USER nonroot:nonroot
ENTRYPOINT ["/app/ecommerce-api"]
CMD ["serve"]
