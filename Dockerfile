# ビルド用ステージ
FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /pocket-sub ./cmd/pocket-sub

# 実行用ステージ
FROM gcr.io/distroless/static
# 静的ファイルは相対パス（web/static）で配信するため、作業ディレクトリ配下に置く
WORKDIR /app
COPY --from=build /pocket-sub /app/pocket-sub
COPY --from=build /src/web /app/web
# PORTはホスティング側が環境変数で渡す
ENTRYPOINT ["/app/pocket-sub"]
