package cli

import (
	"errors"
	"regexp"
	"strings"

	"github.com/pterm/pterm"
)

var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// SiloCLITable Generates terminal table UI.
type SiloCLITable struct {
	tableData     [][]string
	columnWidths  []int
	tableRenderer *pterm.TablePrinter
}

func NewdefaultTable() *SiloCLITable {
	return &SiloCLITable{
		tableData:     [][]string{},
		columnWidths:  []int{},
		tableRenderer: nil,
	}
}

// NewDefaultTableUI renders a table in the terminal with default settings, including column widths based on the terminal size.
func (r *SiloCLITable) WithData(tableData [][]string) *SiloCLITable {

	r.tableData = tableData

	return r
}

func (r *SiloCLITable) WithColourBoolColumn(tableData [][]string, colIndex int) *SiloCLITable {
	for row := range tableData {
		if row == 0 {
			continue // Skip header row
		}
		if colIndex < 0 || colIndex >= len(tableData[row]) {
			continue
		}
		value := strings.ToLower(strings.TrimSpace(tableData[row][colIndex]))
		if value == "true" {
			tableData[row][colIndex] = pterm.Green(tableData[row][colIndex])
		} else if value == "false" {
			tableData[row][colIndex] = pterm.Yellow(tableData[row][colIndex])
		}
	}
	return r
}

func (r *SiloCLITable) Render() error {
	if len(r.tableData) == 0 {
		return errors.New("table data is empty")
	}

	columnCount := len(r.tableData[0])
	if columnCount == 0 {
		return errors.New("table header is empty")
	}

	//grab the terminal width to calculate column widths.
	terminalWidth := pterm.GetTerminalWidth()

	// Account for table borders and separators.
	availableWidth := terminalWidth - 6
	if availableWidth < columnCount {
		availableWidth = columnCount
	}

	columnWidth := availableWidth / columnCount
	r.columnWidths = make([]int, columnCount)
	for i := range r.columnWidths {
		r.columnWidths[i] = columnWidth
	}

	// Centre all table cell contents.
	for row := range r.tableData {
		for col := range r.tableData[row] {
			if col < len(r.columnWidths) {
				r.tableData[row][col] = centerText(r.tableData[row][col], r.columnWidths[col])
			}
		}
	}

	r.tableRenderer = pterm.DefaultTable.
		WithHasHeader().
		WithColumnMinWidths(r.columnWidths...).
		WithData(r.tableData)

	return r.tableRenderer.Render()
}

func centerText(value string, width int) string {
	visibleLength := len(ansiEscapeRegex.ReplaceAllString(value, ""))
	padding := width - visibleLength
	if padding <= 0 {
		return value
	}

	left := padding / 2
	right := padding - left

	return strings.Repeat(" ", left) +
		value +
		strings.Repeat(" ", right)
}
