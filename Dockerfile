# Build frontend
FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

# Build Go binary
FROM golang:1.25-alpine AS go
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN rm -rf cmd/vibecoding/dist && mkdir -p cmd/vibecoding/dist && cp -R web/dist/. cmd/vibecoding/dist/
RUN CGO_ENABLED=0 go build -tags server -o /vibecoding ./cmd/vibecoding

FROM alpine:3.20
RUN apk add --no-cache git ca-certificates
COPY --from=go /vibecoding /usr/local/bin/vibecoding
EXPOSE 8090
ENTRYPOINT ["vibecoding"]
CMD ["--addr", "0.0.0.0:8090", "--data-dir", "/data"]
