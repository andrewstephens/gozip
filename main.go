package main

import (
	"GoZIp/zip"
	"os"
)

func main() {
	// open file
	//file, err := os.Open("example.zip")
	//if err != nil {
	//	panic(err)
	//}
	//
	//defer file.Close()
	//
	//zip.ReadZip(file)

	// Write to zip
	file, err := os.Create("output.zip")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Create a zip writer
	zw := zip.NewZipWriter(file)

	// Add some files
	err = zw.AddFile("hello.txt", []byte("Hello, World!"))
	if err != nil {
		panic(err)
	}

	err = zw.AddFile("readme.txt", []byte("This is a readme file."))
	if err != nil {
		panic(err)
	}

	// Close the zip writer
	err = zw.Close()
	if err != nil {
		panic(err)
	}

	// Print success message
	println("Zip file created successfully: output.zip")
}
