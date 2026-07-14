package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ProcessPDFs(inputDir, outputDir string, verbose bool) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	files, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || strings.ToLower(filepath.Ext(file.Name())) != ".pdf" {
			continue
		}

		inputPath := filepath.Join(inputDir, file.Name())
		outputPath := filepath.Join(outputDir, file.Name()+".txt")

		if err := ExtractText(inputPath, outputPath, verbose); err != nil {
			return fmt.Errorf("failed to process %s: %w", file.Name(), err)
		}

		if verbose {
			fmt.Printf("Extracted: %s\n", file.Name())
		}
	}

	return nil
}

// ExtractTextFromBytes extracts all page text from in-memory PDF bytes.
func ExtractTextFromBytes(data []byte) (string, error) {
	reader, err := NewReaderFromBytes(data)
	if err != nil {
		return "", fmt.Errorf("failed to create PDF reader: %w", err)
	}

	var sb strings.Builder
	for i := 1; i <= reader.GetNumPages(); i++ {
		text, err := reader.ExtractTextFromPage(i)
		if err != nil {
			return "", fmt.Errorf("failed to extract text from page %d: %w", i, err)
		}
		sb.WriteString(text)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// ExtractCellsFromBytes is like ExtractTextFromBytes but preserves table
// cell boundaries with a space — see Reader.ExtractCellsFromPage.
func ExtractCellsFromBytes(data []byte) (string, error) {
	reader, err := NewReaderFromBytes(data)
	if err != nil {
		return "", fmt.Errorf("failed to create PDF reader: %w", err)
	}

	var sb strings.Builder
	for i := 1; i <= reader.GetNumPages(); i++ {
		text, err := reader.ExtractCellsFromPage(i)
		if err != nil {
			return "", fmt.Errorf("failed to extract cells from page %d: %w", i, err)
		}
		sb.WriteString(text)
	}
	return sb.String(), nil
}

func ExtractText(inputPath, outputPath string, verbose bool) error {
	reader, err := NewReader(inputPath)
	if err != nil {
		return fmt.Errorf("failed to create PDF reader: %w", err)
	}
	defer reader.Close()

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	for i := 1; i <= reader.GetNumPages(); i++ {
		text, err := reader.ExtractTextFromPage(i)
		if err != nil {
			return fmt.Errorf("failed to extract text from page %d: %w", i, err)
		}
		if _, err := outFile.WriteString(text + "\n"); err != nil {
			return fmt.Errorf("failed to write text: %w", err)
		}
	}

	return nil
}
