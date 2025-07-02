package zip

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
	"time"
)

func TestNewZipWriter(t *testing.T) {
	var buf bytes.Buffer
	zw := NewZipWriter(&buf)

	if zw == nil {
		t.Fatal("NewZipWriter returned nil")
	}
	if zw.w != &buf {
		t.Error("Writer not set correctly")
	}
	if len(zw.files) != 0 {
		t.Error("Files slice should be empty initially")
	}
	if zw.offset != 0 {
		t.Error("Offset should be 0 initially")
	}
}

func TestAddSingleFile(t *testing.T) {
	var buf bytes.Buffer
	zw := NewZipWriter(&buf)

	// Add a simple file
	filename := "test.txt"
	content := []byte("Hello, World!")

	err := zw.AddFile(filename, content)
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	// Close to finalize the ZIP
	err = zw.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify the ZIP is valid by reading it back
	reader := bytes.NewReader(buf.Bytes())
	zipReader, err := zip.NewReader(reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to open ZIP for reading: %v", err)
	}

	// Check we have exactly one file
	if len(zipReader.File) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(zipReader.File))
	}

	// Check the file details
	file := zipReader.File[0]
	if file.Name != filename {
		t.Errorf("Expected filename %s, got %s", filename, file.Name)
	}
	if file.UncompressedSize64 != uint64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), file.UncompressedSize64)
	}

	// Check the file content
	rc, err := file.Open()
	if err != nil {
		t.Fatalf("Failed to open file in ZIP: %v", err)
	}
	defer rc.Close()

	readContent, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("Failed to read file content: %v", err)
	}

	if !bytes.Equal(readContent, content) {
		t.Errorf("Content mismatch. Expected %s, got %s", content, readContent)
	}
}

func TestAddMultipleFiles(t *testing.T) {
	var buf bytes.Buffer
	zw := NewZipWriter(&buf)

	// Test data
	files := map[string][]byte{
		"file1.txt":     []byte("First file content"),
		"file2.txt":     []byte("Second file content"),
		"dir/file3.txt": []byte("Third file in directory"),
	}

	// Add all files
	for name, content := range files {
		err := zw.AddFile(name, content)
		if err != nil {
			t.Fatalf("AddFile failed for %s: %v", name, err)
		}
	}

	// Close the ZIP
	err := zw.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify by reading back
	reader := bytes.NewReader(buf.Bytes())
	zipReader, err := zip.NewReader(reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to open ZIP for reading: %v", err)
	}

	// Check we have the right number of files
	if len(zipReader.File) != len(files) {
		t.Fatalf("Expected %d files, got %d", len(files), len(zipReader.File))
	}

	// Check each file
	for _, file := range zipReader.File {
		expectedContent, exists := files[file.Name]
		if !exists {
			t.Errorf("Unexpected file in ZIP: %s", file.Name)
			continue
		}

		// Check file content
		rc, err := file.Open()
		if err != nil {
			t.Errorf("Failed to open file %s: %v", file.Name, err)
			continue
		}

		readContent, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Errorf("Failed to read file %s: %v", file.Name, err)
			continue
		}

		if !bytes.Equal(readContent, expectedContent) {
			t.Errorf("Content mismatch for %s. Expected %s, got %s",
				file.Name, expectedContent, readContent)
		}
	}
}

func TestEmptyFile(t *testing.T) {
	var buf bytes.Buffer
	zw := NewZipWriter(&buf)

	err := zw.AddFile("empty.txt", []byte{})
	if err != nil {
		t.Fatalf("AddFile failed for empty file: %v", err)
	}

	err = zw.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify empty file
	reader := bytes.NewReader(buf.Bytes())
	zipReader, err := zip.NewReader(reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to open ZIP: %v", err)
	}

	if len(zipReader.File) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(zipReader.File))
	}

	file := zipReader.File[0]
	if file.UncompressedSize64 != 0 {
		t.Errorf("Expected empty file, got size %d", file.UncompressedSize64)
	}
}

func TestLargeFile(t *testing.T) {
	var buf bytes.Buffer
	zw := NewZipWriter(&buf)

	// Create a larger file (1MB)
	largeContent := make([]byte, 1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	err := zw.AddFile("large.bin", largeContent)
	if err != nil {
		t.Fatalf("AddFile failed for large file: %v", err)
	}

	err = zw.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify large file
	reader := bytes.NewReader(buf.Bytes())
	zipReader, err := zip.NewReader(reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to open ZIP: %v", err)
	}

	file := zipReader.File[0]
	if file.UncompressedSize64 != uint64(len(largeContent)) {
		t.Errorf("Size mismatch. Expected %d, got %d",
			len(largeContent), file.UncompressedSize64)
	}

	// Verify content (spot check)
	rc, err := file.Open()
	if err != nil {
		t.Fatalf("Failed to open large file: %v", err)
	}
	defer rc.Close()

	readContent, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("Failed to read large file: %v", err)
	}

	if !bytes.Equal(readContent, largeContent) {
		t.Error("Large file content mismatch")
	}
}

func TestUTF8Filenames(t *testing.T) {
	var buf bytes.Buffer
	zw := NewZipWriter(&buf)

	// Test various UTF-8 filenames
	testFiles := map[string][]byte{
		"hello.txt":              []byte("English"),
		"héllo.txt":              []byte("French"),
		"こんにちは.txt":         []byte("Japanese"),
		"🌟⭐✨.txt":                []byte("Emoji"),
		"test/深い/ファイル.txt": []byte("Deep path"),
	}

	for name, content := range testFiles {
		err := zw.AddFile(name, content)
		if err != nil {
			t.Fatalf("AddFile failed for %s: %v", name, err)
		}
	}

	err := zw.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify UTF-8 filenames
	reader := bytes.NewReader(buf.Bytes())
	zipReader, err := zip.NewReader(reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to open ZIP: %v", err)
	}

	for _, file := range zipReader.File {
		expectedContent, exists := testFiles[file.Name]
		if !exists {
			t.Errorf("Unexpected file: %s", file.Name)
			continue
		}

		rc, err := file.Open()
		if err != nil {
			t.Errorf("Failed to open UTF-8 file %s: %v", file.Name, err)
			continue
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Errorf("Failed to read UTF-8 file %s: %v", file.Name, err)
			continue
		}

		if !bytes.Equal(content, expectedContent) {
			t.Errorf("Content mismatch for UTF-8 file %s", file.Name)
		}
	}
}

func TestTimeToMSDos(t *testing.T) {
	// Test the time conversion function
	testTime := time.Date(2023, 12, 25, 14, 30, 45, 0, time.UTC)

	dosTime, dosDate := timeToMSDos(testTime)

	// Verify the bit packing
	expectedYear := 2023 - 1980 // 43
	expectedMonth := 12
	expectedDay := 25
	expectedDate := uint16(expectedYear<<9 | expectedMonth<<5 | expectedDay)

	expectedHour := 14
	expectedMinute := 30
	expectedSecond := 45 / 2 // DOS time stores seconds/2
	expectedTime := uint16(expectedHour<<11 | expectedMinute<<5 | expectedSecond)

	if dosDate != expectedDate {
		t.Errorf("Date conversion failed. Expected %d, got %d", expectedDate, dosDate)
	}
	if dosTime != expectedTime {
		t.Errorf("Time conversion failed. Expected %d, got %d", expectedTime, dosTime)
	}
}

func TestEmptyZip(t *testing.T) {
	var buf bytes.Buffer
	zw := NewZipWriter(&buf)

	// Close without adding any files
	err := zw.Close()
	if err != nil {
		t.Fatalf("Close failed on empty ZIP: %v", err)
	}

	// Verify empty ZIP is valid
	reader := bytes.NewReader(buf.Bytes())
	zipReader, err := zip.NewReader(reader, int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to open empty ZIP: %v", err)
	}

	if len(zipReader.File) != 0 {
		t.Errorf("Expected 0 files in empty ZIP, got %d", len(zipReader.File))
	}
}

// Benchmark tests
func BenchmarkAddSmallFile(b *testing.B) {
	content := []byte("Hello, World!")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		zw := NewZipWriter(&buf)
		zw.AddFile("test.txt", content)
		zw.Close()
	}
}

func BenchmarkAddLargeFile(b *testing.B) {
	content := make([]byte, 1024*1024) // 1MB
	for i := range content {
		content[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		zw := NewZipWriter(&buf)
		zw.AddFile("large.bin", content)
		zw.Close()
	}
}

// Example test
func ExampleZipWriter() {
	var buf bytes.Buffer
	zw := NewZipWriter(&buf)

	// Add some files
	zw.AddFile("hello.txt", []byte("Hello, World!"))
	zw.AddFile("data.txt", []byte("Some data here"))

	// Close to finalize
	zw.Close()

	// buf now contains a valid ZIP file
	println("ZIP file created with", buf.Len(), "bytes")
}
