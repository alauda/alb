package main

// copy from https://github.com/gzuidhof/tygo.git

import (
	"log"
	"os"
	"strings"

	t "tylua/tylua"
)

func main() {
	t := t.New(&t.Config{
		Packages: []*t.PackageConfig{
			{
				Path:       os.Args[1],
				OutputPath: os.Args[2],
			},
		},
	})
	err := t.Generate(strings.Split(os.Args[1], ","), os.Args[2], os.Args[3])
	if err != nil {
		log.Fatalf("Tylua failed: %v", err)
	}
}
