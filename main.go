package main

import (
	"GoZIp/zip"
	_ "GoZIp/zip"
	"os"
)

func main() {
	// open file
	file, err := os.Open("Holding Out for a Hero.zip")
	if err != nil {
		panic(err)
	}

	defer file.Close()

	zip.ReadZip(file)
}
