package output

import (
	"fmt"
	"io"
	"strings"
)

// Table renders tabular data for text output.
type Table struct {
	headers   []string
	rows      [][]string
	noHeader  bool
	separator string
}

// NewTable creates a new table with the given headers.
func NewTable(headers ...string) *Table {
	return &Table{
		headers:   headers,
		rows:      [][]string{},
		separator: "  ",
	}
}

// AddRow adds a row to the table.
func (t *Table) AddRow(cells ...string) {
	t.rows = append(t.rows, cells)
}

// SetNoHeader suppresses the header row.
func (t *Table) SetNoHeader(noHeader bool) {
	t.noHeader = noHeader
}

// SetSeparator sets the column separator.
func (t *Table) SetSeparator(sep string) {
	t.separator = sep
}

// Render renders the table to the writer.
//
//nolint:gocognit // Table rendering logic is clear and readable at complexity 11
func (t *Table) Render(w io.Writer) error {
	if len(t.headers) == 0 && len(t.rows) == 0 {
		return nil
	}

	// Calculate column widths
	widths := t.calculateWidths()

	// Print header
	if !t.noHeader && len(t.headers) > 0 {
		if err := t.renderRow(w, t.headers, widths); err != nil {
			return err
		}
		if err := t.renderSeparatorLine(w, widths); err != nil {
			return err
		}
	}

	// Print rows
	for _, row := range t.rows {
		if err := t.renderRow(w, row, widths); err != nil {
			return err
		}
	}

	return nil
}

// String returns the table as a string.
func (t *Table) String() string {
	var sb strings.Builder
	_ = t.Render(&sb)
	return sb.String()
}

// calculateWidths calculates the maximum width for each column.
//
//nolint:gocognit // Width calculation logic is clear and readable at complexity 13
func (t *Table) calculateWidths() []int {
	numCols := len(t.headers)
	for _, row := range t.rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}

	widths := make([]int, numCols)

	// Account for headers
	for i, h := range t.headers {
		if len(h) > widths[i] {
			widths[i] = len(h)
		}
	}

	// Account for rows
	for _, row := range t.rows {
		for i, cell := range row {
			if i < numCols && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	return widths
}

// renderRow renders a single row.
func (t *Table) renderRow(w io.Writer, cells []string, widths []int) error {
	parts := make([]string, len(widths))
	for i := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		parts[i] = fmt.Sprintf("%-*s", widths[i], cell)
	}
	_, err := fmt.Fprintln(w, strings.Join(parts, t.separator))
	return err
}

// renderSeparatorLine renders a separator line under the header.
func (t *Table) renderSeparatorLine(w io.Writer, widths []int) error {
	parts := make([]string, len(widths))
	for i, width := range widths {
		parts[i] = strings.Repeat("-", width)
	}
	_, err := fmt.Fprintln(w, strings.Join(parts, t.separator))
	return err
}
