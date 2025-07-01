package main

import (
	"GoZIp/zip"
	"os"
)

func main() {
	// open file
	file, err := os.Open("example.zip")
	if err != nil {
		panic(err)
	}

	defer file.Close()

	zip.ReadZip(file)
}
