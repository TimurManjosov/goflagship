# Enhance Admin UI with Search, Filters, Env Tabs, JSON Editor, and Audit View

## Problem / Motivation

The current admin UI (`sdk/admin.html`) is functional but lacks important features for managing flags at scale:

1. **No Search**: Hard to find specific flags when there are many
2. **No Filtering**: Can't filter by enabled/disabled, environment, or other criteria
3. **Single Environment**: No way to view/manage multiple environments simultaneously
4. **Basic JSON Editor**: Config editor is plain textarea (no syntax highlighting, validation)
5. **No Audit Trail**: Can't see who changed what and when
6. **No Bulk Actions**: Can't enable/disable multiple flags at once
7. **Poor Mobile Experience**: Not responsive on small screens

These limitations make the UI difficult to use for teams with:
- 50+ flags
- Multiple environments (dev, staging, prod)
- Multiple team members making changes
- Complex flag configurations

## Proposed Solution

Transform the admin UI into a production-ready dashboard with:

1. **Search & Filters**: Real-time search, filter by status/env/tags
2. **Environment Tabs**: Switch between environments, side-by-side comparison
3. **Advanced JSON Editor**: Syntax highlighting, validation, formatting
4. **Audit View**: Timeline of all changes with user attribution
5. **Bulk Actions**: Select multiple flags, perform batch operations
6. **Responsive Design**: Works well on mobile/tablet

## Concrete Tasks

### Phase 1: Search & Filter Bar
- [ ] Add search input at top of flag list:
  ```html
  <input 
    type="search" 
    placeholder="Search flags by key, description..." 
    id="flag-search"
  />
  ```
- [ ] Implement real-time search (filter as user types):
  - Match against flag key
  - Match against description
  - Highlight matching text
- [ ] Add filter dropdowns:
  - **Status**: All / Enabled / Disabled
  - **Rollout**: All / Full (100%) / Partial (<100%) / Off (0%)
  - **Has Expression**: All / With Expression / Without Expression
- [ ] Combine search + filters (AND logic)
- [ ] Show result count: "Showing 12 of 47 flags"
- [ ] Add "Clear filters" button

### Phase 2: Environment Tabs
- [ ] Add tab navigation at top:
  ```
  [Development] [Staging] [Production]
  ```
- [ ] Store selected environment in local storage
- [ ] Load flags for selected environment
- [ ] Update URL hash: `#env=prod`
- [ ] Show environment badge on each flag card
- [ ] (Optional) Side-by-side environment comparison:
  - Split screen: prod on left, staging on right
  - Highlight differences (flag exists in one but not other)
  - Sync button to copy flag from one env to another

### Phase 3: Advanced JSON Editor
- [ ] Replace plain textarea with Monaco Editor or CodeMirror:
  - **Option A**: Monaco Editor (VS Code editor, 600KB)
  - **Option B**: CodeMirror 6 (lighter, ~100KB)
  - **Option C**: Ace Editor (classic, ~400KB)
  - **Recommendation**: CodeMirror 6 for balance
- [ ] Features to implement:
  - Syntax highlighting for JSON
  - Auto-indentation and formatting
  - Error highlighting (invalid JSON)
  - Line numbers
  - Auto-closing brackets/quotes
  - Format button (prettify JSON)
- [ ] Add JSON schema validation (if Issue #3 is implemented)
- [ ] Add "Expand" button for fullscreen editing
- [ ] Show character count and JSON depth

### Phase 4: Audit View / History Tab
- [ ] Add "History" or "Audit" tab in main navigation
- [ ] Fetch audit logs from backend (requires Issue #12 backend)
- [ ] Display timeline of changes:
  ```
  ┌─────────────────────────────────────────┐
  │ Jan 15, 2025 10:30 AM                   │
  │ user@example.com updated feature_x      │
  │ Changed: enabled: true → false          │
  │ Rollout: 50% → 100%                     │
  └─────────────────────────────────────────┘
  
  ┌─────────────────────────────────────────┐
  │ Jan 14, 2025 3:45 PM                    │
  │ admin@example.com created banner_msg    │
  │ Enabled: true, Config: {...}            │
  └─────────────────────────────────────────┘
  ```
- [ ] Filter audit logs by:
  - Date range
  - User
  - Flag key
  - Action (created, updated, deleted)
- [ ] Add pagination (show 20 per page)
- [ ] Add export button (download as CSV/JSON)
- [ ] Show diff for updates (before/after comparison)

### Phase 5: Bulk Actions
- [ ] Add checkbox to each flag card
- [ ] Add "Select all" / "Select none" buttons
- [ ] Show selection count: "3 flags selected"
- [ ] Add bulk action toolbar (appears when flags selected):
  - Enable selected
  - Disable selected
  - Delete selected (with confirmation)
  - Export selected
  - Clone to another environment
- [ ] Implement batch API calls:
  ```javascript
  async function bulkEnable(keys) {
    const promises = keys.map(key => 
      fetch('/v1/flags', {
        method: 'POST',
        body: JSON.stringify({key, enabled: true})
      })
    );
    await Promise.all(promises);
  }
  ```
- [ ] Show progress bar for bulk operations
- [ ] Handle partial failures (some succeed, some fail)

### Phase 6: UI/UX Improvements
- [ ] Add flag tags/labels (visual categorization):
  - Color-coded badges: "beta", "deprecated", "critical"
  - Store in config: `{"tags": ["beta", "frontend"]}`
- [ ] Add keyboard shortcuts:
  - `/` - Focus search
  - `n` - New flag
  - `?` - Show shortcuts help
  - `Esc` - Close modals
- [ ] Add flag preview (expand card to see full config)
- [ ] Add copy button for flag keys
- [ ] Add last updated timestamp on cards
- [ ] Add created by / updated by (requires user tracking)
- [ ] Improve empty states:
  - No flags: "No flags yet. Create your first flag!"
  - No search results: "No flags match your search. Try different terms."

### Phase 7: Responsive Design
- [ ] Make UI mobile-friendly:
  - Stack layout on small screens
  - Touch-friendly buttons (larger tap targets)
  - Simplified editor on mobile
- [ ] Add responsive breakpoints:
  - Desktop: >1200px (full layout)
  - Tablet: 768-1200px (condensed)
  - Mobile: <768px (single column)
- [ ] Test on various devices/browsers:
  - iPhone (Safari)
  - Android (Chrome)
  - Tablet (both orientations)
  - Desktop (Chrome, Firefox, Safari)

### Phase 8: Performance & Polish
- [ ] Virtualize flag list (only render visible flags):
  - Use virtual scrolling library (e.g., `react-window` if migrating to React)
  - Or implement simple windowing manually
- [ ] Debounce search input (wait 300ms after typing stops)
- [ ] Add loading skeletons (instead of spinners)
- [ ] Add optimistic UI updates:
  - Update UI immediately, rollback on error
  - Show pending state during API call
- [ ] Add success/error toast notifications:
  - Green toast: "Flag updated successfully"
  - Red toast: "Failed to update flag: [error]"
- [ ] Cache environment selection, filter state, search term
- [ ] Add dark mode toggle (optional)

## API Changes

### New Requirements (if not already implemented)

If Issue #12 (Audit Log) is implemented:
```
GET /v1/admin/audit-logs?page=1&limit=20&flagKey=feature_x&startDate=2025-01-01
```

### Potential Enhancement (optional)
```
POST /v1/flags/batch - Batch flag operations
{
  "operations": [
    {"action": "update", "key": "flag1", "changes": {"enabled": true}},
    {"action": "delete", "key": "flag2", "env": "dev"}
  ]
}
```

## Acceptance Criteria

### Search & Filters
- [ ] Search filters flags in real-time
- [ ] Search matches key and description
- [ ] Filter dropdowns work (status, rollout, expression)
- [ ] Filters combine with search (AND logic)
- [ ] Clear filters button resets everything
- [ ] Result count updates correctly

### Environment Tabs
- [ ] Tabs switch between environments
- [ ] Selected tab is persisted (localStorage)
- [ ] URL hash reflects selected environment
- [ ] Flags reload when environment changes

### JSON Editor
- [ ] Syntax highlighting works
- [ ] Invalid JSON is highlighted
- [ ] Format button prettifies JSON
- [ ] Editor is usable (not janky)
- [ ] Error messages are clear

### Audit View
- [ ] Audit timeline displays recent changes
- [ ] Changes show before/after diff
- [ ] Filter by date, user, flag works
- [ ] Pagination works
- [ ] Export to CSV/JSON works

### Bulk Actions
- [ ] Checkboxes select multiple flags
- [ ] Bulk toolbar appears when flags selected
- [ ] Bulk enable/disable/delete works
- [ ] Progress is shown during operations
- [ ] Partial failures are handled gracefully

### UX
- [ ] UI is responsive on mobile
- [ ] Keyboard shortcuts work
- [ ] Loading states are clear
- [ ] Errors are user-friendly
- [ ] Toast notifications appear on actions

## Notes / Risks / Edge Cases

### Risks
- **Complexity**: Adding too many features makes UI cluttered
  - Mitigation: Progressive disclosure (advanced features in menus/tabs)
- **Performance**: Rendering 1000+ flags could be slow
  - Mitigation: Virtual scrolling, pagination, lazy loading
- **Browser Compatibility**: Advanced editor might not work on old browsers
  - Mitigation: Test on common browsers, provide fallback
- **State Management**: Complex UI state (search, filters, selection) hard to manage
  - Mitigation: Consider React/Vue/Svelte migration (separate issue?)

### Edge Cases
- Search with special characters (regex escaping)
- Bulk delete all flags (confirmation should be very clear)
- Environment doesn't exist (show helpful message)
- Audit log is empty (show "No history yet")
- Network error during bulk operation (rollback? show which failed?)
- Very long flag key or description (truncate with ellipsis)
- Flag config is huge (>100KB JSON) - editor might slow down

### Technology Choices

**JSON Editor Libraries**
- **Monaco** (VS Code): Best editor, but large (600KB), slow on mobile
- **CodeMirror 6**: Modern, fast, lighter (100KB), good mobile support
- **Ace**: Classic, stable, medium size (400KB), good ecosystem

**Virtual Scrolling**
- **react-window** (if using React)
- **vue-virtual-scroller** (if using Vue)
- **vanilla-virtualize** (vanilla JS)

**UI Framework Migration** (optional)
- Consider migrating to React/Vue/Svelte for better state management
- Current vanilla JS is fine for now, but might need framework at scale
- Separate issue: "Migrate Admin UI to React/Vue"

### Future Enhancements
- Real-time collaboration (see others' cursors while editing)
- Flag templates (create from template)
- Flag groups / folders for organization
- Advanced diff view (unified diff, side-by-side)
- Export all flags to Excel/CSV
- Import flags from CSV
- Flag scheduling (enable/disable at specific time)
- Flag comments/notes
- Favorite flags (pin to top)
- Recent flags (recently viewed/edited)

## Implementation Hints

- Current admin UI is in `sdk/admin.html` (single HTML file with inline CSS/JS)
- Consider splitting into modules:
  - `admin.html` - main HTML structure
  - `styles.css` - extracted styles
  - `app.js` - main application logic
  - `components/` - reusable components (if migrating to framework)
- CodeMirror 6 installation:
  ```bash
  npm install @codemirror/lang-json @codemirror/view @codemirror/state
  ```
- Example CodeMirror setup:
  ```javascript
  import {EditorView, basicSetup} from "codemirror"
  import {json} from "@codemirror/lang-json"
  
  new EditorView({
    doc: JSON.stringify(config, null, 2),
    extensions: [basicSetup, json()],
    parent: document.getElementById('editor')
  })
  ```
- Virtual scrolling example (vanilla JS):
  ```javascript
  const ITEM_HEIGHT = 100;
  const VISIBLE_ITEMS = Math.ceil(window.innerHeight / ITEM_HEIGHT);
  let scrollTop = 0;
  
  function renderVisibleItems() {
    const startIndex = Math.floor(scrollTop / ITEM_HEIGHT);
    const endIndex = startIndex + VISIBLE_ITEMS + 1;
    // Only render items from startIndex to endIndex
  }
  ```

## Labels

`feature`, `frontend`, `ui`, `enhancement`, `good-first-issue` (for search/filter), `help-wanted`

## Estimated Effort

**4-5 days** (can be split into multiple PRs)
- Day 1: Search & filters + environment tabs
- Day 2: Advanced JSON editor integration
- Day 3: Audit view (depends on backend Issue #12)
- Day 4: Bulk actions + keyboard shortcuts
- Day 5: Responsive design + performance + polish

**Note**: Can split into multiple smaller issues:
- Issue 10a: Search, Filters, Env Tabs (2 days)
- Issue 10b: JSON Editor & Audit View (2 days)
- Issue 10c: Bulk Actions & Responsive Design (2 days)
