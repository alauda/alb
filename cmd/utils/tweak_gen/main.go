package main

import (
	"fmt"
	"os"
	"path"

	"alauda.io/alb2/pkg/operator/controllers/depl/resources/configmap"
)

// generate configmap used for nginx test
func main() {
	outDir := os.Args[1]
	_ = os.MkdirAll(outDir, 0o700)
	fmt.Printf("%v", configmap.HTTP)
	fmt.Printf("%v", outDir)
	toFile(configmap.HTTP, path.Join(outDir, "http.conf"))
	toFile(configmap.HTTPSERVER, path.Join(outDir, "http_server.conf"))
	toFile(configmap.UPSTREAM, path.Join(outDir, "upstream.conf"))
	toFile(configmap.STREAM_COMMON, path.Join(outDir, "stream-common.conf"))
	toFile(configmap.STREAM_TCP, path.Join(outDir, "stream-tcp.conf"))
	toFile(configmap.STREAM_UDP, path.Join(outDir, "stream-udp.conf"))
	toFile(configmap.GRPCSERVER, path.Join(outDir, "grpc_server.conf"))
}

func toFile(content string, path string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	_, err = f.WriteString(content)
	if err != nil {
		panic(err)
	}
}
