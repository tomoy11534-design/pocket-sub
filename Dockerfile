# ビルド用ステージ
FROM golang:1.22 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /pocket-sub ./cmd/pocket-sub

FROM gcr.io/distroless/static
WORKDIR /app
COPY --from=build /pocket-sub /app/pocket-sub
# ↓静的ファイルをコンテナ内にコピーする行を追加
COPY --from=build /src/web /app/web

ENTRYPOINT ["/app/pocket-sub"]