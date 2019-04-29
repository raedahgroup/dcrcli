package widgets

import (
	"fyne.io/fyne"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
)

type TableCell fyne.CanvasObject

type TableRow struct {
	Cells []TableCell
}

type Table struct {
	Rows []*TableRow
}

func NewTable() *Table {
	return &Table{}
}

func (table *Table) AddRow(rowObjects ...TableCell) {
	table.Rows = append(table.Rows, &TableRow{rowObjects})
}

func (table *Table) AddRowSimple(texts ...string) {
	tableCells := make([]TableCell, len(texts))
	for i, text := range texts {
		tableCells[i] = widget.NewLabel(text)
	}
	table.AddRow(tableCells...)
}

func (table *Table) Clear() {
	table.Rows = []*TableRow{}
}

// DefaultTable returns a table that grows beyond the minimum size to cover all available space
func (table *Table) DefaultTable() fyne.CanvasObject {
	defaultTable := fyne.NewContainerWithLayout(layout.NewGridLayout(1))
	for _, row := range table.Rows {
		rowObject := fyne.NewContainerWithLayout(layout.NewGridLayout(len(row.Cells)))
		for _, cell := range row.Cells {
			rowObject.AddObject(cell)
		}
		defaultTable.AddObject(rowObject)
	}
	return defaultTable
}

// CondensedTable returns a table that does not grow beyond the minimum size required to display the longest row
func (table *Table) CondensedTable() fyne.CanvasObject {
	condensedTable := widget.NewVBox()
	columnCellSizes := table.calculateColumnCellSizes()

	for _, row := range table.Rows {
		cellContainers := make([]fyne.CanvasObject, len(row.Cells))
		for i, cell := range row.Cells {
			columnSize := columnCellSizes[i]
			cellContainers[i] = fyne.NewContainerWithLayout(layout.NewFixedGridLayout(columnSize), cell)
		}
		condensedTable.Append(widget.NewHBox(cellContainers...))
	}

	return condensedTable
}

// calculateColumnCellSizes checks all cells in all rows to determine the widest cell in each column
func (table *Table) calculateColumnCellSizes() (columnCellSizes []fyne.Size) {
	for _, row := range table.Rows {
		for i, cell := range row.Cells {
			if len(columnCellSizes) > i {
				columnCellSizes[i] = cell.MinSize().Union(columnCellSizes[i])
			} else {
				columnCellSizes = append(columnCellSizes, cell.MinSize())
			}
		}
	}
	return
}
