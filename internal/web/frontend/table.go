// table.go provides a reusable HTML table renderer for HTMX-powered partial
// responses. Define a TableConfig with columns and a partial URL, parse the
// request into a TableState, then call RenderHeader/RenderFooter around your
// row output. Handles sortable column headers, search/filter input, pagination
// controls, and page size selection. Used by all list partials (users, groups,
// SSH certs, FIDO2 keys, service accounts, dashboard activity).
package frontend

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
)

const (
	defaultPageSize = 25
	maxPageSize     = 100
	filterDelay     = "300ms"
	arrowUp         = " &#9650;"
	arrowDown       = " &#9660;"
	emptyMessage    = "No results"
)

var defaultPageSizes = []int{10, 25, 50, 100}

// Column defines a table column.
type Column struct {
	Key      string
	Label    string
	Sortable bool
}

// TableConfig defines the table's static configuration.
type TableConfig struct {
	Columns     []Column
	PartialURL  string
	Filterable  bool
	Exportable  bool
	Selectable  bool
	BulkActions []BulkAction
	PageSizes   []int
}

// BulkAction defines an available bulk operation.
type BulkAction struct {
	Label       string
	URL         string
	Class       string // CSS class for the button (e.g., "btn-danger")
	Confirm     bool   // Requires "yesiagree" confirmation
}

// TableState holds the current request state for a table.
type TableState struct {
	Sort   string
	Order  string
	Offset int
	Limit  int
	Query  string
	Total  int
}

// ParseTableState extracts table state from query parameters.
func ParseTableState(r *http.Request, defaultSort string) TableState {
	s := TableState{
		Sort:  r.URL.Query().Get("sort"),
		Order: r.URL.Query().Get("order"),
		Query: r.URL.Query().Get("q"),
	}
	s.Offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	s.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))

	if s.Sort == "" {
		s.Sort = defaultSort
	}
	if s.Order != "asc" && s.Order != "desc" {
		s.Order = "desc"
	}
	if s.Limit <= 0 || s.Limit > maxPageSize {
		s.Limit = defaultPageSize
	}
	if s.Offset < 0 {
		s.Offset = 0
	}
	return s
}

// TableRenderer writes standardized table HTML.
type TableRenderer struct {
	cfg   TableConfig
	state TableState
	w     io.Writer
}

// NewTableRenderer creates a renderer for the given config and state.
func NewTableRenderer(w io.Writer, cfg TableConfig, state TableState) *TableRenderer {
	if len(cfg.PageSizes) == 0 {
		cfg.PageSizes = defaultPageSizes
	}
	return &TableRenderer{cfg: cfg, state: state, w: w}
}

// RenderHeader writes the filter input, loading indicator, and table header row.
func (tr *TableRenderer) RenderHeader() {
	// Bulk action bar (hidden by default, shown via JS when rows selected)
	if tr.cfg.Selectable && len(tr.cfg.BulkActions) > 0 {
		fmt.Fprint(tr.w, `<div id="bulk-action-bar" class="bulk-bar hidden">`)
		fmt.Fprint(tr.w, `<span class="text-sm" id="bulk-count">0 selected</span>`)
		for _, action := range tr.cfg.BulkActions {
			cls := "btn btn-secondary text-xs"
			if action.Class != "" {
				cls = "btn " + action.Class + " text-xs"
			}
			confirmAttr := ""
			if action.Confirm {
				confirmAttr = ` data-bulk-confirm="true"`
			}
			fmt.Fprintf(tr.w, ` <button class="%s" data-bulk-url="%s"%s onclick="submitBulk(this)">%s</button>`,
				cls, action.URL, confirmAttr, action.Label)
		}
		fmt.Fprint(tr.w, `</div>`)
	}

	// Filter and controls bar
	if tr.cfg.Filterable || tr.cfg.Exportable {
		fmt.Fprint(tr.w, `<div class="flex justify-between items-center mb-3">`)
		if tr.cfg.Filterable {
			fmt.Fprintf(tr.w,
				`<input type="text" name="q" value="%s" placeholder="&#128269; Filter..." class="input flex-1" hx-get="%s" hx-trigger="keyup changed delay:%s" hx-target="closest .table-container" hx-include="[name='limit']" hx-indicator=".table-loading">`,
				escHTML(tr.state.Query), tr.cfg.PartialURL, filterDelay,
			)
		} else {
			fmt.Fprint(tr.w, `<div></div>`)
		}
		rightControls := ""
		if tr.cfg.Exportable {
			rightControls = fmt.Sprintf(`<a href="%s?format=csv&q=%s&sort=%s&order=%s" class="btn btn-secondary text-xs">Export CSV</a>`,
				tr.cfg.PartialURL, tr.state.Query, tr.state.Sort, tr.state.Order)
		}
		fmt.Fprintf(tr.w, `<div class="flex items-center space-x-2">%s</div>`, rightControls)
		fmt.Fprint(tr.w, `</div>`)
	}

	// Loading indicator
	fmt.Fprint(tr.w, `<div class="table-loading htmx-indicator text-center text-sm text-gray-500 py-2">Loading...</div>`)

	// Table open + thead
	fmt.Fprint(tr.w, `<table class="table"><thead><tr>`)

	// Select-all checkbox
	if tr.cfg.Selectable {
		fmt.Fprint(tr.w, `<th class="w-8"><input type="checkbox" id="select-all" onchange="toggleSelectAll(this)"></th>`)
	}

	for _, col := range tr.cfg.Columns {
		if col.Sortable {
			nextOrder := "asc"
			indicator := ""
			if col.Key == tr.state.Sort {
				if tr.state.Order == "asc" {
					nextOrder = "desc"
					indicator = arrowUp
				} else {
					nextOrder = "asc"
					indicator = arrowDown
				}
			}
			fmt.Fprintf(tr.w,
				`<th><a href="#" class="text-blue-600 hover:text-blue-600" hx-get="%s?sort=%s&order=%s&limit=%d&q=%s" hx-target="closest .table-container" hx-swap="innerHTML">%s%s</a></th>`,
				tr.cfg.PartialURL, col.Key, nextOrder, tr.state.Limit, tr.state.Query, col.Label, indicator,
			)
		} else {
			fmt.Fprintf(tr.w, `<th>%s</th>`, col.Label)
		}
	}
	fmt.Fprint(tr.w, `</tr></thead><tbody>`)
}

// RenderEmpty writes a "no results" row spanning all columns.
func (tr *TableRenderer) RenderEmpty(msg string) {
	if msg == "" {
		msg = emptyMessage
	}
	colSpan := len(tr.cfg.Columns)
	if tr.cfg.Selectable {
		colSpan++
	}
	fmt.Fprintf(tr.w, `<tr><td colspan="%d" class="text-center text-gray-500 dark:text-gray-400 text-sm py-4">%s</td></tr>`,
		colSpan, msg)
}

// RenderFooter writes pagination controls, page size selector, and row count.
func (tr *TableRenderer) RenderFooter() {
	fmt.Fprint(tr.w, `</tbody></table>`)

	end := tr.state.Offset + tr.state.Limit
	if end > tr.state.Total {
		end = tr.state.Total
	}

	fmt.Fprint(tr.w, `<div class="flex justify-between items-center mt-3 text-sm text-gray-500 dark:text-gray-400">`)

	// Row count
	if tr.state.Total > 0 {
		fmt.Fprintf(tr.w, `<span>Showing %d-%d of %d</span>`, tr.state.Offset+1, end, tr.state.Total)
	} else {
		fmt.Fprint(tr.w, `<span></span>`)
	}

	// Page size + navigation
	fmt.Fprint(tr.w, `<div class="flex items-center space-x-3">`)

	// Page size selector
	fmt.Fprint(tr.w, `<select name="limit" class="input w-20 text-xs" hx-get="`)
	fmt.Fprint(tr.w, tr.cfg.PartialURL)
	fmt.Fprint(tr.w, `" hx-trigger="change" hx-target="closest .table-container" hx-include="[name='q']">`)
	for _, size := range tr.cfg.PageSizes {
		selected := ""
		if size == tr.state.Limit {
			selected = " selected"
		}
		fmt.Fprintf(tr.w, `<option value="%d"%s>%d</option>`, size, selected, size)
	}
	fmt.Fprint(tr.w, `</select>`)

	// Prev/Next
	fmt.Fprint(tr.w, `<div class="flex space-x-2">`)
	if tr.state.Offset > 0 {
		prev := tr.state.Offset - tr.state.Limit
		if prev < 0 {
			prev = 0
		}
		fmt.Fprintf(tr.w, `<button class="btn btn-secondary text-xs" hx-get="%s?offset=%d&limit=%d&sort=%s&order=%s&q=%s" hx-target="closest .table-container">Prev</button>`,
			tr.cfg.PartialURL, prev, tr.state.Limit, tr.state.Sort, tr.state.Order, tr.state.Query)
	}
	if end < tr.state.Total {
		fmt.Fprintf(tr.w, `<button class="btn btn-secondary text-xs" hx-get="%s?offset=%d&limit=%d&sort=%s&order=%s&q=%s" hx-target="closest .table-container">Next</button>`,
			tr.cfg.PartialURL, end, tr.state.Limit, tr.state.Sort, tr.state.Order, tr.state.Query)
	}
	fmt.Fprint(tr.w, `</div>`)

	fmt.Fprint(tr.w, `</div></div>`)

	// Bulk selection JS (only when selectable)
	if tr.cfg.Selectable {
		fmt.Fprint(tr.w, `<script>
var bulkSelected = new Set();
function toggleSelectAll(el) {
  var boxes = document.querySelectorAll('.bulk-check');
  boxes.forEach(function(cb) { cb.checked = el.checked; toggleRow(cb); });
}
function toggleRow(cb) {
  var id = cb.value;
  if (cb.checked) { bulkSelected.add(id); } else { bulkSelected.delete(id); }
  updateBulkBar();
}
function updateBulkBar() {
  var bar = document.getElementById('bulk-action-bar');
  var count = document.getElementById('bulk-count');
  if (!bar) return;
  if (bulkSelected.size > 0) {
    bar.classList.remove('hidden');
    count.textContent = bulkSelected.size + ' selected';
  } else {
    bar.classList.add('hidden');
  }
}
function submitBulk(btn) {
  if (bulkSelected.size === 0) return;
  var url = btn.getAttribute('data-bulk-url');
  var needsConfirm = btn.getAttribute('data-bulk-confirm');
  if (needsConfirm) {
    var input = prompt('Type "yesiagree" to confirm this action on ' + bulkSelected.size + ' items');
    if (input !== 'yesiagree') return;
  }
  fetch(url, {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({ids: Array.from(bulkSelected)})
  }).then(function(resp) { return resp.json(); }).then(function(data) {
    bulkSelected.clear();
    document.body.dispatchEvent(new CustomEvent('showFlash', {detail: {type: data.type || 'success', text: data.message || 'Done'}}));
    htmx.trigger(document.querySelector('.table-container'), 'htmx:load');
    var container = document.querySelector('.table-container');
    if (container) { htmx.ajax('GET', '` + tr.cfg.PartialURL + `', {target: container, swap: 'innerHTML'}); }
  }).catch(function(err) {
    document.body.dispatchEvent(new CustomEvent('showFlash', {detail: {type: 'error', text: 'Bulk action failed: ' + err.message}}));
  });
}
</script>`)
	}
}
