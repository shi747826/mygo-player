package main

import "github.com/egoist/mygo/ui"

func (a *app) pluginFilters(c *ui.Context) {
	for _, m := range a.pluginManager.Manifests() {
		if m.ID != a.source {
			continue
		}
		for _, filter := range m.Filters {
			f := filter
			current := a.channel
			if f.Value == "sort" {
				current = a.hgSort
			}
			labels := make([]string, len(f.Options))
			selected := 0
			for i, o := range f.Options {
				labels[i] = o.Label
				if o.Value == current {
					selected = i
				}
			}
			if ui.Segmented(c.Key("plugin-filter-"+f.Value), &selected, labels...).Label(f.Label).Changed() && selected >= 0 && selected < len(f.Options) {
				if f.Value == "sort" {
					a.hgSort = f.Options[selected].Value
					a.hgSortSel = selected
				} else {
					a.channel = f.Options[selected].Value
					a.channelSel = selected
				}
				a.loadPage("推荐", false)
			}
		}
	}
}
