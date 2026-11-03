package cli

import (
	"strings"

	"github.com/pterm/pterm"
)

// SiloCLITableRenderer Generates terminal table UI.
type SiloCLITableRenderer struct{}

// NewSiloCLITableRenderer creates a new instance of SiloCLITableRenderer.
func NewSiloCLITableRenderer() *SiloCLITableRenderer {
	return &SiloCLITableRenderer{}
}

// NewDefaultTableUI renders a table in the terminal with default settings, including column widths based on the terminal size.
func (r *SiloCLITableRenderer) NewDefaultTableUI(tableData [][]string) error {

	if len(tableData) == 0 {
		return nil
	}

	terminalWidth := pterm.GetTerminalWidth()

	// Account for table borders and separators.
	availableWidth := terminalWidth - 6

	columnCount := len(tableData[0])
	columnWidth := availableWidth / columnCount

	columnWidths := make([]int, columnCount)
	for i := range columnWidths {
		columnWidths[i] = columnWidth
	}

	// Centre all table cell contents.
	for row := range tableData {
		for col := range tableData[row] {
			tableData[row][col] = centerText(tableData[row][col], columnWidths[col])
		}
	}

	err := pterm.DefaultTable.
		WithHasHeader().
		WithColumnMinWidths(columnWidths...).
		WithData(tableData).
		Render()

	if err != nil {
		return err
	}

	return nil
}

func centerText(value string, width int) string {
	padding := width - len(value)
	if padding <= 0 {
		return value
	}

	left := padding / 2
	right := padding - left

	return strings.Repeat(" ", left) +
		value +
		strings.Repeat(" ", right)
}
