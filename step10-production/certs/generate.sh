#!/usr/bin/env bash
# TLS テスト用の自己署名証明書を生成する
# openssl が必要
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt \
  -days 365 -nodes \
  -subj "/C=JP/ST=Tokyo/L=Tokyo/O=gRPC-Learn/CN=localhost"
echo "証明書を生成しました: server.crt, server.key"
