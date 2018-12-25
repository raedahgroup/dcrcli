package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

func tabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 1, ' ', tabwriter.AlignRight|tabwriter.Debug)
}

// PrintResult formats and prints the content of `res` to `w`
func PrintResult(w *tabwriter.Writer, res *Response) {
	header := ""
	spaceRow := ""
	columnLength := len(res.Columns)

	for i := range res.Columns {
		tab := " \t "
		if columnLength == i+1 {
			tab = " "
		}
		header += res.Columns[i] + tab
		spaceRow += " " + tab
	}

	fmt.Fprintln(w, header)
	fmt.Fprintln(w, spaceRow)
	for _, row := range res.Result {
		rowStr := ""
		for range row {
			rowStr += "%v \t "
		}

		rowStr = strings.TrimSuffix(rowStr, "\t ")
		fmt.Fprintln(w, fmt.Sprintf(rowStr, row...))
	}

	w.Flush()
}
