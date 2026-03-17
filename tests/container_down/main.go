package main

import (
	"zylo/internal/container"
)

func main() {
	downConf := container.ContainerDown{
		Hash: "1772832936-727109866772198263",
	}

	err := container.Down(downConf.Hash)
	if err != nil {
		panic(err)
	}
}
