package main

import (
	"fmt"
	"os"

	"alauda.io/alb2/utils/dirhash"
)

func main() {
	out, err := dirhash.HashDir(os.Args[1], "", dirhash.DefaultHash)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", out)
}
