package pdf

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/ledongthuc/pdf"
)

type Reader struct {
	pdfPath string
	reader  *pdf.Reader
	file    *os.File
}

func NewReader(pdfPath string) (*Reader, error) {
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("PDF file does not exist: %w", err)
	}

	f, err := os.Open(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF file: %w", err)
	}

	fileInfo, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	reader, err := pdf.NewReader(f, fileInfo.Size())
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to create PDF reader: %w", err)
	}

	return &Reader{
		pdfPath: pdfPath,
		reader:  reader,
		file:    f,
	}, nil
}

// NewReaderFromBytes creates a Reader from in-memory PDF bytes, with no disk I/O.
func NewReaderFromBytes(data []byte) (*Reader, error) {
	r := bytes.NewReader(data)
	reader, err := pdf.NewReader(r, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF reader: %w", err)
	}
	return &Reader{reader: reader}, nil
}

func (r *Reader) GetNumPages() int {
	return r.reader.NumPage()
}

func (r *Reader) ExtractTextFromPage(pageNum int) (string, error) {
	if pageNum < 1 || pageNum > r.reader.NumPage() {
		return "", fmt.Errorf("invalid page number: %d", pageNum)
	}

	page := r.reader.Page(pageNum)
	if page.V.IsNull() {
		return "", fmt.Errorf("page %d is null", pageNum)
	}

	text, err := page.GetPlainText(nil)
	if err != nil {
		return "", fmt.Errorf("failed to extract text from page %d: %w", pageNum, err)
	}

	return text, nil
}

// ExtractCellsFromPage returns the page's text with a single space inserted
// between every table cell. Some statements' content streams place each
// table cell's text at explicit coordinates with no whitespace glyph between
// adjacent cells — ExtractTextFromPage concatenates those cells with nothing
// at all (e.g. "11233.000.00" for two amount columns), which is ambiguous to
// re-split. This walks the page's text grouped by row/column position
// instead of raw stream order, so cell boundaries are preserved.
func (r *Reader) ExtractCellsFromPage(pageNum int) (string, error) {
	if pageNum < 1 || pageNum > r.reader.NumPage() {
		return "", fmt.Errorf("invalid page number: %d", pageNum)
	}

	page := r.reader.Page(pageNum)
	if page.V.IsNull() {
		return "", fmt.Errorf("page %d is null", pageNum)
	}

	rows, err := page.GetTextByRow()
	if err != nil {
		return "", fmt.Errorf("failed to extract cell text from page %d: %w", pageNum, err)
	}

	var sb strings.Builder
	for _, row := range rows {
		for _, t := range row.Content {
			s := strings.TrimSpace(t.S)
			if s == "" {
				continue
			}
			sb.WriteString(s)
			sb.WriteString(" ")
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

func (r *Reader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}
