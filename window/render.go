package window

import (
	"gioui.org/layout"
	"gioui.org/widget/material"
)

type IFRender interface {
	Render(ctx *RenderContext) layout.Dimensions
}

type RenderContext struct {
	Gtx   layout.Context
	Theme *material.Theme
}
