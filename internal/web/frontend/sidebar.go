// sidebar.go provides a reusable two-column sidebar layout component for pages
// with categorized content. Renders a left navigation with HTMX-powered panel
// switching and a right content area. Used by the Settings and Backup pages.
package frontend

import (
	"fmt"
	"io"
)

// SidebarNavItem defines a single navigation entry in the sidebar.
type SidebarNavItem struct {
	Label string
	URL   string
}

// SidebarConfig defines the sidebar layout configuration.
type SidebarConfig struct {
	PanelID    string
	DefaultURL string
	NavItems   []SidebarNavItem
}

// SidebarRenderer writes the two-column sidebar layout HTML.
type SidebarRenderer struct {
	cfg SidebarConfig
	w   io.Writer
}

// NewSidebarRenderer creates a renderer for the given config.
func NewSidebarRenderer(w io.Writer, cfg SidebarConfig) *SidebarRenderer {
	return &SidebarRenderer{cfg: cfg, w: w}
}

// Render writes the complete sidebar layout including nav and panel container.
func (sr *SidebarRenderer) Render() {
	fmt.Fprint(sr.w, `<div class="flex gap-6 max-w-5xl">`)

	// Left nav
	fmt.Fprint(sr.w, `<nav class="w-48 shrink-0"><ul class="space-y-1 text-sm">`)
	for i, item := range sr.cfg.NavItems {
		active := ""
		if item.URL == sr.cfg.DefaultURL {
			active = " active"
		}
		fmt.Fprintf(sr.w,
			`<li><a href="#" class="sidebar-nav-item%s" data-idx="%d" hx-get="%s" hx-target="#%s" hx-swap="innerHTML">%s</a></li>`,
			active, i, item.URL, sr.cfg.PanelID, item.Label,
		)
	}
	fmt.Fprint(sr.w, `</ul></nav>`)

	// Right panel
	fmt.Fprintf(sr.w,
		`<div id="%s" class="flex-1 card" hx-get="%s" hx-trigger="load" hx-swap="innerHTML"><p class="text-gray-500 dark:text-gray-400 text-sm">Loading...</p></div>`,
		sr.cfg.PanelID, sr.cfg.DefaultURL,
	)

	fmt.Fprint(sr.w, `</div>`)

	// Active state JS
	fmt.Fprintf(sr.w, `<script>
document.addEventListener('htmx:afterRequest', function(evt) {
  if (evt.detail.target && evt.detail.target.id === '%s') {
    document.querySelectorAll('.sidebar-nav-item').forEach(function(el) { el.classList.remove('active'); });
    if (evt.detail.elt && evt.detail.elt.classList.contains('sidebar-nav-item')) {
      evt.detail.elt.classList.add('active');
    }
  }
});
</script>`, sr.cfg.PanelID)
}
