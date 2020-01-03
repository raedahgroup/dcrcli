module github.com/raedahgroup/godcr

go 1.12

replace (
	github.com/raedahgroup/godcr/cmd/godcr-fyne => ./cmd/godcr-fyne
	github.com/raedahgroup/godcr/cmd/godcr-terminal => ./cmd/godcr-terminal
	github.com/raedahgroup/godcr/fyne => ./fyne
	github.com/raedahgroup/godcr/terminal => ./terminal
)

require github.com/raedahgroup/godcr/cmd/godcr-fyne v0.0.0-00010101000000-000000000000 // indirect
