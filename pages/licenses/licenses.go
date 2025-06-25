package licenses

import (
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/ikafly144/sabalauncher/applayout"
	"github.com/ikafly144/sabalauncher/icon"
	"github.com/ikafly144/sabalauncher/pages"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type Page struct {
	widget.List
	*pages.Router
}

var _ pages.Page = (*Page)(nil)

func New(router *pages.Router) *Page {
	p := &Page{
		Router: router,
	}
	return p
}

// NavItem implements pages.Page.
func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "ライセンス",
		Icon: icon.LicenseIcon,
	}
}

// Actions implements pages.Page.
func (p *Page) Actions() []component.AppBarAction {
	return []component.AppBarAction{}
}

// Overflow implements pages.Page.
func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

//go:generate go tool go-licenses save . --save_path dist_licenses --force --ignore github.com/ikafly144/sabalauncher

//go:embed dist_licenses
var licenses embed.FS

var licensesMap map[string]string = make(map[string]string)
var licenseEntries []mapEntry[string, string]

type mapEntry[K any, V any] struct {
	key   K
	value V
}

func init() {
	var readDir func(path string, dirEntry fs.DirEntry) error
	readDir = func(path string, dirEntry fs.DirEntry) error {
		name := path + "/" + dirEntry.Name()
		if dirEntry.IsDir() {
			dirs, err := licenses.ReadDir(name)
			if err != nil {
				return fmt.Errorf("failed to read dir %s: %v", name, err)
			}
			for i := range dirs {
				if err := readDir(name, dirs[i]); err != nil {
					return fmt.Errorf("failed to read licenses: %v", err)
				}
			}
			return nil
		} else {
			f, err := licenses.ReadFile(name)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %v", name, err)
			}
			license := string(f)
			v, ok := licensesMap[path]
			if ok {
				license = v + "\n---\n\n" + license
			}
			licensesMap[path] = license
		}
		return nil
	}

	dirs, err := licenses.ReadDir("dist_licenses")
	if err != nil {
		panic(fmt.Sprintf("failed to read licenses: %v", err))
	}
	for i := range dirs {
		if err := readDir("dist_licenses", dirs[i]); err != nil {
			panic(fmt.Sprintf("failed to read licenses: %v", err))
		}
	}
	for k, v := range licensesMap {
		k = strings.TrimPrefix(k, "dist_licenses/")
		licenseEntries = append(licenseEntries, mapEntry[string, string]{key: k, value: v})
	}
	slices.SortStableFunc(licenseEntries, func(a, b mapEntry[string, string]) int {
		// Sort by key alphabetically
		c := strings.Compare(a.key, b.key)
		if c == 0 {
			return len(a.key) - len(b.key)
		}
		return c
	})
}

// Layout implements pages.Page.
// Subtle: this method shadows the method (*Router).Layout of Page.Router.
func (p *Page) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	p.List.Axis = layout.Vertical
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return p.List.Layout(gtx, len(licenseEntries), func(gtx C, i int) D {
				switch i {
				case 0:
					return applayout.DefaultInset.Layout(gtx, material.H3(th, "オープンソースライセンス").Layout)
				case 1:
					return layout.UniformInset(16).Layout(gtx, func(gtx C) D {
						return material.Body1(th, "本ソフトウェアが使用しているライブラリのライセンス情報は以下の通りです。").Layout(gtx)
					})
				}
				name := licenseEntries[i].key
				license := licenseEntries[i].value
				return layout.UniformInset(16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return applayout.DefaultInset.Layout(gtx, material.H6(th, name).Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.UniformInset(16).Layout(gtx, func(gtx C) D {
								return applayout.DefaultInset.Layout(gtx, material.Body1(th, license).Layout)
							})
						}),
					)
				})
			})
		}),
	)
}
