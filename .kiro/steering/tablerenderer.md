---
inclusion: fileMatch
fileMatchPattern: "**/frontend/partials.go,**/frontend/table.go"
---

# TableRenderer Usage Guide

## Overview

`TableRenderer` (in `internal/web/frontend/table.go`) is a reusable HTML table component for HTMX-powered partial responses. All list views (users, groups, SSH certs, FIDO2 keys, service accounts, dashboard activity) use it.

## Creating a New Table

### 1. Define the TableConfig

```go
cfg := TableConfig{
    Columns: []Column{
        {Key: "uid", Label: "Username", Sortable: true},
        {Key: "status", Label: "Status", Sortable: false},
        {Key: "_actions", Label: "", Sortable: false},
    },
    PartialURL:  "/partials/things/list",
    Filterable:  true,
    Selectable:  true,
    BulkActions: []BulkAction{
        {Label: "Delete", URL: "/things/bulk/delete", Class: "btn-danger", Confirm: true},
    },
}
```

### 2. Parse request state

```go
state := ParseTableState(r, "uid") // "uid" is the default sort column
```

### 3. Filter, sort, paginate, render

```go
// Filter
var filtered []myRow
for _, item := range allItems {
    if q != "" && !strings.Contains(strings.ToLower(item.Name), q) {
        continue
    }
    filtered = append(filtered, item)
}

// Sort (required for sortable columns to work)
sort.Slice(filtered, func(i, j int) bool {
    var less bool
    switch state.Sort {
    case "name":
        less = strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
    default:
        less = strings.ToLower(filtered[i].UID) < strings.ToLower(filtered[j].UID)
    }
    if state.Order == "desc" {
        return !less
    }
    return less
})

// Paginate
total := len(filtered)
end := state.Offset + state.Limit
if end > total { end = total }
page := filtered[state.Offset:end]
state.Total = total

// Render
tr := NewTableRenderer(w, cfg, state)
tr.RenderHeader()
if len(page) == 0 {
    tr.RenderEmpty("No results")
} else {
    for _, item := range page {
        fmt.Fprintf(w, `<tr><td>...</td></tr>`)
    }
}
tr.RenderFooter()
```

### 4. Register the partial route

In `registerPartials()`:
```go
r.Get("/partials/things/list", f.h.partialThingList)
```

### 5. Add the table container in the page template

```html
<div class="table-container" hx-get="/partials/things/list" hx-trigger="load" hx-swap="innerHTML">
  <p class="text-gray-500 dark:text-gray-400 text-sm">Loading...</p>
</div>
```

## Bulk Actions

### BulkAction struct fields

| Field | Type | Purpose |
|-------|------|---------|
| Label | string | Button text |
| URL | string | POST endpoint for the action |
| Class | string | CSS class (e.g., "btn-danger") |
| Confirm | bool | Requires "yesiagree" prompt |
| Prompt | string | If set, prompts user for a value (sent as "value" in JSON body) |
| EligibleIf | string | JS expression for two-phase eligibility check (empty = all eligible) |

### EligibleIf expressions

Expressions are evaluated per row against `data-*` attributes on the row's checkbox element. Available variables depend on what data attributes the row renders:

```go
// User rows provide these attributes:
// data-disabled="true|false"
// data-type="{employeeType}"
// data-self="true|false"
// data-admin="true|false"

{EligibleIf: "disabled=='true' || type=='contact'"}           // Delete: only disabled or contacts
{EligibleIf: "self!='true' && type!='contact' && disabled!='true'"} // Disable: not self, not contact, not already disabled
```

### Adding data attributes to row checkboxes

```go
fmt.Fprintf(w, `<input type="checkbox" class="bulk-check" value="%s" data-disabled="%s" data-type="%s" onchange="toggleRow(this)">`,
    escHTML(item.UID), disabledAttr, escHTML(item.Type))
```

### Two-phase confirmation flow

1. First click: JS evaluates `EligibleIf` on all selected rows
2. Ineligible rows get `conflict-row-bg` class (amber highlight)
3. Bar shows: "25 selected (5 ineligible)", button shows "Delete (20)"
4. Second click: fires the POST with all selected IDs
5. Backend still validates independently (defense in depth)

## Bulk Action Backend Handler Pattern

```go
func (h *handlers) actionBulkDeleteThings(w http.ResponseWriter, r *http.Request) {
    var req bulkRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
        respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "No items selected"})
        return
    }

    deleted := 0
    skipped := 0
    for _, id := range req.IDs {
        // validate eligibility server-side
        if err := h.deps.Repo.DeleteThing(id); err == nil {
            deleted++
        } else {
            skipped++
        }
    }

    msg := fmt.Sprintf("%d deleted", deleted)
    if skipped > 0 {
        msg += fmt.Sprintf(" (%d skipped)", skipped)
    }
    respondJSON(w, http.StatusOK, map[string]any{
        "type":    "success",
        "message": msg,
        "deleted": deleted,
        "skipped": skipped,
    })
}
```

## Safety Checks (for user-related tables)

Always include in destructive handlers:
- **Self-protection**: skip if `uid == emailToUID(claims.Email)`
- **Last-admin protection**: skip if `IsUserAdmin(uid) && CountActiveAdmins() <= 1`
- **Disable-before-delete**: skip if `!user.Disabled && user.EmployeeType != "contact"`

## CSS Classes

- `.table-container` - wrapper div that HTMX targets for swaps
- `.bulk-bar` - selection action bar (light blue bg, hidden by default)
- `.bulk-check` - checkbox class for row selection
- `.conflict-row-bg` - amber highlight for ineligible rows (light: #fffbeb, dark: rgba(180,83,9,0.15))

## Sort and Page Size Persistence

Sort and page size preferences are saved to localStorage and restored on page navigation.

- Save `{sort, order, limit}` to localStorage on every partial render, keyed by `tableSort:<partialURL>`
- Global `htmx:configRequest` listener injects saved sort/order/limit into initial table load requests
- Only applies when the request has no explicit `sort` param (initial load only)
- Pagination offset is NOT saved (returning to page 37 is confusing)
- Page size select fires through the same listener, so changing limit preserves saved sort
