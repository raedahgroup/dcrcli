package pages

import (
	"fmt"

	"github.com/rivo/tview"
	"github.com/gdamore/tcell"
)

func SendPage(setFocus func(p tview.Primitive) *tview.Application, clearFocus func()) tview.Primitive {
	//Form for Sending
	body := tview.NewForm().
		AddDropDown("Account", []string{"Dafault", "..."}, 0, nil).
		AddInputField("Amount", "", 20, nil, nil).
		AddInputField("Destination Address", "", 20, nil, nil).
		AddButton("Send", func() {
			fmt.Println("Next")
		})
	body.AddButton("Cancel", func() {
		clearFocus()
	})
	body.SetBackgroundColor(tcell.NewRGBColor(255, 255, 255))
	body.SetLabelColor(tcell.NewRGBColor(0, 0, 0))
	body.SetFieldTextColor(tcell.NewRGBColor(0, 0, 0))

	setFocus(body)

	return body
}
