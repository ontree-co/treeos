# UI Improvements Implementation Tickets

## 🎨 Emoji Feature Implementation

### TICKET-001: Update Data Structures for Emoji Support
**Priority**: High  
**Dependencies**: None

**Tasks**:
- [ ] Update `AppConfig` struct in `internal/models/app.go` to include `Emoji` field in `OnTreeMetadata`
- [ ] Update YAML parsing to handle emoji field in `x-ontree` section
- [ ] Add emoji validation to ensure only valid Unicode emojis are accepted
- [ ] Update database schema if needed to store emoji data

**Acceptance Criteria**:
- AppConfig can store and retrieve emoji values
- YAML files with `x-ontree.emoji` are parsed correctly
- Invalid emoji values are rejected with appropriate error

---

### TICKET-002: Create Emoji Picker Component
**Priority**: High  
**Dependencies**: TICKET-001

**Tasks**:
- [ ] Create `/templates/components/emoji-picker.html` with HTMX component
- [ ] Implement 7-emoji grid layout with selection functionality
- [ ] Add hidden input field to store selected emoji
- [ ] Style component with Bootstrap classes
- [ ] Add JavaScript for emoji selection handling

**Acceptance Criteria**:
- Component displays 7 emojis in a responsive grid
- Clicking an emoji highlights it and updates hidden input
- Component can be reused in multiple forms

---

### TICKET-003: Implement Emoji Shuffle Endpoint
**Priority**: High  
**Dependencies**: TICKET-002

**Tasks**:
- [ ] Add `GET /components/emoji-picker/shuffle` endpoint in router
- [ ] Create handler that returns 7 random emojis from curated list
- [ ] Define emoji pool constant with app-appropriate emojis
- [ ] Return HTMX-compatible HTML fragment

**Acceptance Criteria**:
- Endpoint returns 7 different random emojis each time
- Response is valid HTMX fragment that replaces emoji grid
- No duplicate emojis in single response

---

### TICKET-004: Integrate Emoji Picker into App Creation
**Priority**: Medium  
**Dependencies**: TICKET-002, TICKET-003

**Tasks**:
- [ ] Add emoji picker component to `/apps/create` form
- [ ] Position after app name input field
- [ ] Update form handler to process emoji selection
- [ ] Save emoji to docker-compose.yaml in `x-ontree` section

**Acceptance Criteria**:
- Emoji picker appears in create form
- Selected emoji is saved with new app
- Form validation includes emoji field

---

### TICKET-005: Integrate Emoji Picker into Template Creation
**Priority**: Medium  
**Dependencies**: TICKET-002, TICKET-003

**Tasks**:
- [ ] Add emoji picker to template configuration step
- [ ] Update template processing to include emoji in generated YAML
- [ ] Ensure emoji is properly escaped in YAML output

**Acceptance Criteria**:
- Template creation shows emoji picker
- Generated docker-compose includes selected emoji
- Templates work with or without emoji selection

---

### TICKET-006: Display Emoji on App Detail Page
**Priority**: Medium  
**Dependencies**: TICKET-001

**Tasks**:
- [ ] Update app detail template to show emoji before app name
- [ ] Handle cases where emoji is not set (graceful fallback)
- [ ] Add emoji to page title/header section

**Acceptance Criteria**:
- Emoji appears before app name when set
- No visual issues when emoji is missing
- Emoji displays correctly on all browsers

---

### TICKET-007: Display Emoji on Dashboard Cards
**Priority**: Medium  
**Dependencies**: TICKET-001

**Tasks**:
- [ ] Update dashboard app cards to show emoji
- [ ] Position emoji before app name in card title
- [ ] Ensure proper spacing and alignment

**Acceptance Criteria**:
- Dashboard cards show app emojis
- Cards without emojis display normally
- Consistent styling across all cards

---

## 🎨 UI Style Updates

### TICKET-008: Implement Gradient Primary Buttons
**Priority**: High  
**Dependencies**: None

**Tasks**:
- [ ] Update `static/css/style.css` with new gradient button styles
- [ ] Add gradient from light green (#4a7c28) to dark green (#2d5016)
- [ ] Implement hover and active states with appropriate gradients
- [ ] Add enhanced styles for `.btn-lg` variants
- [ ] Test across different browsers for gradient compatibility

**Acceptance Criteria**:
- All primary buttons show gradient effect
- Hover states work smoothly
- Buttons are accessible and maintain contrast
- Works in Chrome, Firefox, Safari, Edge

---

### TICKET-009: Simplify Homepage Create Button
**Priority**: High  
**Dependencies**: None

**Tasks**:
- [ ] Remove "Create from Template" button from homepage
- [ ] Update "Create New App" button to link directly to `/apps/templates`
- [ ] Remove any dropdown functionality
- [ ] Apply gradient styling from TICKET-008

**Acceptance Criteria**:
- Single "Create New App" button on homepage
- Button navigates to templates page
- No dropdown menu present

---

### TICKET-010: Clean Up Settings Navigation
**Priority**: High  
**Dependencies**: None

**Tasks**:
- [ ] Update `/templates/components/navigation.html`
- [ ] Remove Settings dropdown menu items (Dashboard, Create, App, Docker Operations)
- [ ] Convert Settings to direct link to `/settings`
- [ ] Remove dropdown toggle functionality
- [ ] Update any JavaScript that references the dropdown

**Acceptance Criteria**:
- Settings is a direct link, not a dropdown
- Old menu items are completely removed
- Navigation works correctly on all pages

---

### TICKET-011: Make App Cards Fully Clickable
**Priority**: High  
**Dependencies**: TICKET-007

**Tasks**:
- [ ] Remove "Manage" button from app cards
- [ ] Wrap entire card in anchor tag linking to app detail
- [ ] Add iOS-style chevron icon (bi-chevron-right) on right side
- [ ] Update CSS for clickable card hover effects
- [ ] Add cursor pointer and hover animations
- [ ] Ensure text doesn't get underlined on hover

**Acceptance Criteria**:
- Entire card is clickable
- Chevron icon appears on right side
- Hover effect lifts card slightly
- No text decoration on hover
- Keyboard navigation works properly

---

## 🧪 Testing & Validation

### TICKET-012: Fix and Update Unit Tests
**Priority**: High  
**Dependencies**: All implementation tickets

**Tasks**:
- [ ] Update existing tests to account for new emoji fields
- [ ] Add tests for emoji picker component rendering
- [ ] Test emoji shuffle endpoint
- [ ] Verify YAML parsing with emoji data
- [ ] Update E2E tests for new UI flows
- [ ] Ensure all existing tests still pass

**Acceptance Criteria**:
- All unit tests pass
- New functionality has appropriate test coverage
- No regression in existing features

---

### TICKET-013: Linting and Code Quality
**Priority**: High  
**Dependencies**: All implementation tickets

**Tasks**:
- [ ] Run `go fmt` on all Go files
- [ ] Run `go vet` and fix any issues
- [ ] Run golangci-lint and address findings
- [ ] Validate HTML templates for proper structure
- [ ] Check CSS for any syntax errors
- [ ] Ensure consistent code style throughout

**Acceptance Criteria**:
- No linting errors in Go code
- HTML validates properly
- CSS has no syntax errors
- Code follows project conventions

---

### TICKET-014: Manual E2E Testing with Playwright
**Priority**: High  
**Dependencies**: All other tickets

**Tasks**:
- [ ] Set up Playwright MCP if not already configured
- [ ] Test complete app creation flow with emoji selection
- [ ] Verify emoji shuffle functionality works
- [ ] Test template creation with emoji
- [ ] Verify gradient buttons display correctly
- [ ] Test simplified homepage navigation flow
- [ ] Verify Settings link works without dropdown
- [ ] Test clickable app cards with chevron
- [ ] Check all hover effects and animations
- [ ] Test on different screen sizes (responsive)
- [ ] Verify emoji display on dashboard and detail pages
- [ ] Test keyboard navigation for accessibility

**Test Scenarios**:
1. **Happy Path - Create App with Emoji**
   - Navigate to homepage
   - Click "Create New App"
   - Select "Create from Scratch"
   - Enter app name
   - Select an emoji
   - Click shuffle and select different emoji
   - Submit form
   - Verify app created with emoji

2. **Template Creation with Emoji**
   - Go to templates page
   - Select a template
   - Configure with emoji
   - Verify emoji appears in final app

3. **UI Navigation**
   - Click on app cards (not buttons)
   - Verify chevron indicators
   - Test Settings direct link
   - Verify gradient buttons throughout

4. **Edge Cases**
   - Create app without selecting emoji
   - Test with very long app names
   - Test on mobile viewport
   - Test with keyboard only navigation

**Acceptance Criteria**:
- All manual tests pass
- No visual regressions
- UI feels polished and responsive
- All features work as specified
- Good performance (no lag on interactions)

---

## Implementation Order

1. **Phase 1 - Foundation** (TICKET-001, TICKET-008)
   - Data structure updates
   - CSS gradient implementation

2. **Phase 2 - UI Components** (TICKET-002, TICKET-003, TICKET-009, TICKET-010, TICKET-011)
   - Build reusable components
   - Update navigation and buttons

3. **Phase 3 - Integration** (TICKET-004, TICKET-005, TICKET-006, TICKET-007)
   - Integrate emoji picker into forms
   - Update display templates

4. **Phase 4 - Quality Assurance** (TICKET-012, TICKET-013, TICKET-014)
   - Testing and validation
   - Manual verification

## Notes

- Each ticket should be completed and tested before moving to dependent tickets
- All work should be done directly on the main branch
- Document any API changes in the respective CLAUDE.md files
- Update main CLAUDE.md with new UI features once complete