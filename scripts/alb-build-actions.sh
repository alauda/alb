#!/bin/bash

function alb-static-build() {
  set -x
  rm ./bin/alb
  rm ./bin/operator
  CC=/usr/bin/musl-gcc CGO_ENABLED=1 go build -v -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now,-static' -v -o ./bin/operator alauda.io/alb2/cmd/operator
  CC=/usr/bin/musl-gcc CGO_ENABLED=1 go build -v -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now,-static' -v -o ./bin/alb alauda.io/alb2/cmd/alb
  CC=/usr/bin/musl-gcc CGO_ENABLED=1 go build -v -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now,-static' -v -o ./bin/migrate/init-port-info alauda.io/alb2/migrate/init-port-info
  md5sum ./bin/alb
  md5sum ./bin/operator
}
