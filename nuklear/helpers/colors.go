package helpers

import (
	"image/color"

	nstyle "github.com/aarzilli/nucular/style"
)

var (
	DangerColor = color.RGBA{220, 53, 69, 255}

	whiteColor             = color.RGBA{255, 255, 255, 255}
	navBackgroundColor     = color.RGBA{9, 20, 64, 255}
	contentBackgroundColor = color.RGBA{240, 240, 250, 255}
)

var colorTable = nstyle.ColorTable{
	ColorText:                  color.RGBA{106, 106, 106, 255},
	ColorWindow:                contentBackgroundColor,
	ColorHeader:                color.RGBA{175, 175, 175, 255},
	ColorBorder:                color.RGBA{206, 212, 218, 255},
	ColorButton:                color.RGBA{9, 20, 64, 255},
	ColorButtonHover:           color.RGBA{255, 255, 255, 255},
	ColorButtonActive:          color.RGBA{0, 153, 204, 255},
	ColorToggle:                color.RGBA{150, 150, 150, 255},
	ColorToggleHover:           color.RGBA{120, 120, 120, 255},
	ColorToggleCursor:          color.RGBA{175, 175, 175, 255},
	ColorSelect:                color.RGBA{175, 175, 175, 255},
	ColorSelectActive:          color.RGBA{190, 190, 190, 255},
	ColorSlider:                color.RGBA{190, 190, 190, 255},
	ColorSliderCursor:          color.RGBA{80, 80, 80, 255},
	ColorSliderCursorHover:     color.RGBA{70, 70, 70, 255},
	ColorSliderCursorActive:    color.RGBA{60, 60, 60, 255},
	ColorProperty:              color.RGBA{175, 175, 175, 255},
	ColorEdit:                  color.RGBA{150, 150, 150, 255},
	ColorEditCursor:            color.RGBA{0, 0, 0, 255},
	ColorCombo:                 color.RGBA{175, 175, 175, 255},
	ColorChart:                 color.RGBA{160, 160, 160, 255},
	ColorChartColor:            color.RGBA{45, 45, 45, 255},
	ColorChartColorHighlight:   color.RGBA{255, 0, 0, 255},
	ColorScrollbar:             color.RGBA{180, 180, 180, 255},
	ColorScrollbarCursor:       color.RGBA{140, 140, 140, 255},
	ColorScrollbarCursorHover:  color.RGBA{150, 150, 150, 255},
	ColorScrollbarCursorActive: color.RGBA{160, 160, 160, 255},
	ColorTabHeader:             color.RGBA{0x89, 0x89, 0x89, 0xff},
}
