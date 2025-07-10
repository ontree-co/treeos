# UI Improvements Specification

## 1. App Emoji Feature

### Overview

Add an emoji selection feature to the app creation process that allows users to associate an emoji with each application. The selected emoji will be:
- Stored in the docker-compose.yaml under the `x-ontree` extension
- Displayed before the app name on the app detail page
- Available for both regular app creation and template-based creation

## 2. Primary Button Color Change

### Overview

Update the primary action buttons throughout the application to use a dark green color scheme for better visual hierarchy and branding.

### Implementation Details

#### CSS Changes
Update the primary button styles in `static/css/style.css`:

```css
/* Gradient primary buttons - light to dark green */
.btn-primary {
    background: linear-gradient(135deg, #4a7c28 0%, #2d5016 100%); /* Light to dark green gradient */
    border: 1px solid #2d5016;
    color: white;
    transition: all 0.3s ease;
}

.btn-primary:hover {
    background: linear-gradient(135deg, #5a8f33 0%, #3a6b1e 100%); /* Slightly lighter gradient on hover */
    border-color: #3a6b1e;
    transform: translateY(-1px);
    box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
}

.btn-primary:active,
.btn-primary:focus {
    background: linear-gradient(135deg, #3d6820 0%, #264012 100%); /* Darker gradient when pressed */
    border-color: #264012;
    box-shadow: 0 0 0 0.25rem rgba(74, 124, 40, 0.25);
}

/* Large primary buttons get enhanced gradient effect */
.btn-primary.btn-lg {
    background: linear-gradient(135deg, #5a8f33 0%, #2d5016 100%);
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.btn-primary.btn-lg:hover {
    background: linear-gradient(135deg, #6ba23e 0%, #3a6b1e 100%);
    box-shadow: 0 6px 12px rgba(0, 0, 0, 0.15);
}
```

#### Affected Buttons
1. **App Detail Page**: "Create & Start" button
2. **Homepage**: "Create New App" button (see section 3)
3. **Forms**: All primary submit buttons

## 3. Simplified App Creation Flow

### Overview

Simplify the app creation process by having a single "Create New App" button on the homepage that navigates directly to the templates page, where users can choose between templates or creating from scratch.

### UI Design

#### Homepage Button Structure
```html
<!-- Simple button that goes to templates page -->
<a href="/apps/templates" class="btn btn-primary btn-lg">
    <i class="bi bi-plus-circle"></i> Create New App
</a>
```

#### Templates Page Enhancement
The templates page already includes a "Create from Scratch" button alongside the template options, providing users with both paths in one location.

### User Flow
1. User clicks "Create New App" on homepage
2. User is taken to templates page
3. User can either:
   - Select a template from the available options
   - Click "Create from Scratch" button for manual configuration

### Benefits
1. **Simpler UI**: No dropdown complexity on homepage
2. **Single Path**: All creation options in one place (templates page)
3. **Better Discovery**: Users see all available templates before deciding to create from scratch
4. **Reduced Clicks**: Direct navigation without dropdown interaction

### 1. Emoji Picker Component

Create a reusable HTMX component that can be embedded in both app creation forms.

**Location**: `/templates/components/emoji-picker.html`

**Features**:
- Display 7 random emojis in a grid layout
- Selected emoji highlighted with border/background
- "Shuffle" button to get 7 new random emojis
- Hidden input field to store selected emoji value
- Responsive design that works on mobile

**HTML Structure**:
```html
<div class="emoji-picker" id="emoji-picker">
    <div class="emoji-grid">
        <!-- 7 emoji buttons -->
        <button type="button" class="emoji-option" data-emoji="🚀">🚀</button>
        <!-- ... -->
    </div>
    <button type="button" class="btn btn-sm btn-secondary" 
            hx-get="/components/emoji-picker/shuffle" 
            hx-target="#emoji-picker">
        🔀 Shuffle
    </button>
    <input type="hidden" name="emoji" id="selected-emoji" value="">
</div>
```

### 2. Integration Points

#### A. Regular App Creation (`/apps/create`)
- Add emoji picker component after app name input
- Include selected emoji in form submission

#### B. Template-Based Creation (`/apps/create-from-template`)
- Add emoji picker component in the configuration step
- Include selected emoji when generating docker-compose.yaml

#### C. App Detail Page (`/apps/{name}`)
- Display emoji before app name in header
- Show placeholder if no emoji selected

## Backend Implementation

### 1. Emoji Storage

Store emoji in docker-compose.yaml under `x-ontree` extension:

```yaml
services:
  myapp:
    image: nginx:latest
    x-ontree:
      emoji: "🚀"
      # other ontree-specific metadata
```

### 2. API Endpoints

#### A. Emoji Picker Shuffle Endpoint
```
GET /components/emoji-picker/shuffle
```
Returns HTML fragment with 7 new random emojis

#### B. Update Existing App Emoji (Future Enhancement)
```
POST /apps/{name}/emoji
```
Update emoji for existing apps

### 3. Data Structures

Update `AppConfig` struct:
```go
type OnTreeMetadata struct {
    Emoji string `yaml:"emoji,omitempty"`
    // existing fields...
}
```

### 4. Emoji Pool

Define a curated list of app-appropriate emojis:
```go
var AppEmojis = []string{
    // Development & Technology
    "💻", "🖥️", "⌨️", "🖱️", "💾", "💿", "📱", "☁️", "🌐", "📡",
    "🔌", "🔋", "🛠️", "⚙️", "🔧", "🔨", "⚡", "🚀", "🛸", "🤖",
    
    // Data & Analytics
    "📊", "📈", "📉", "📋", "📌", "📍", "🗂️", "🗄️", "📁", "📂",
    "💹", "🔍", "🔎", "🧮", "💡", "🎯", "📐", "📏", "🗺️", "🧭",
    
    // Security & Monitoring
    "🔒", "🔓", "🔐", "🔑", "🛡️", "⚠️", "🚨", "📢", "🔔", "👁️",
    "🕵️", "🚦", "🚥", "⏰", "⏱️", "⌚", "📅", "📆", "🕐", "🌡️",
    
    // Communication & Media
    "📧", "📨", "📩", "💬", "💭", "🗨️", "📞", "☎️", "📠", "📻",
    "📺", "📷", "📹", "🎥", "🎬", "🎤", "🎧", "🎵", "🎶", "📣",
    
    // Storage & Database
    "🗃️", "🗳️", "📦", "📮", "📪", "📫", "📬", "📭", "🏗️", "🏭",
    "🏪", "🏬", "🏦", "💳", "💰", "💸", "🪙", "💎", "⚖️", "🔗",
    
    // Nature & Science
    "🌍", "🌎", "🌏", "🌐", "🪐", "🌙", "☀️", "⭐", "🌟", "✨",
    "🔬", "🔭", "🧬", "🧪", "⚗️", "🧫", "🦠", "🧲", "⚛️", "🌡️"
}
```

## Implementation Steps

### Phase 1: Core Infrastructure
1. Create emoji picker component template
2. Add shuffle endpoint handler
3. Update AppConfig struct to include emoji field
4. Modify docker-compose.yaml parser to handle emoji in x-ontree

### Phase 2: Integration with Create Flow
1. Add emoji picker to `/apps/create` form
2. Update form handler to save emoji
3. Add emoji picker to template-based creation
4. Update template processing to include emoji

### Phase 3: Display Implementation
1. Update app detail page template to show emoji
2. Add emoji to app list/dashboard views
3. Handle cases where emoji is not set (show default or nothing)

### Phase 4: Enhancements (Future)
1. Allow editing emoji for existing apps
2. Add emoji search/filter functionality
3. Support custom emoji input
4. Add emoji categories for easier selection

## Technical Considerations

### 1. Unicode Support
- Ensure proper UTF-8 encoding throughout the stack
- Test with various emoji characters
- Handle multi-codepoint emojis correctly

### 2. Fallback Behavior
- Apps without emojis should display normally
- Missing or invalid emojis should not break the UI
- Provide sensible defaults

### 3. Performance
- Emoji picker should not slow down app creation
- Consider caching emoji list
- Minimize DOM updates during shuffle

### 4. Accessibility
- Add proper ARIA labels for emoji buttons
- Ensure keyboard navigation works
- Provide text alternatives for screen readers

## Testing Requirements

### 1. Unit Tests
- Emoji storage in docker-compose.yaml
- Emoji shuffle randomization
- Unicode handling

### 2. Integration Tests
- App creation with emoji
- Template creation with emoji
- Emoji display on detail page

### 3. E2E Tests
- Complete app creation flow with emoji selection
- Emoji persistence across app lifecycle
- UI interaction with emoji picker

## Migration Strategy

For existing apps without emojis:
1. No migration needed - they continue to work without emojis
2. Users can manually add emojis later (future feature)
3. Templates can be updated to include default emojis

## Security Considerations

1. **Input Validation**: Only allow emojis from the predefined list
2. **XSS Prevention**: Properly escape emoji content in templates
3. **Storage Limits**: Emoji field should have reasonable length limit

## 4. Settings Dropdown Cleanup

### Overview

Remove outdated menu items from the Settings dropdown in the navigation bar. The current dropdown contains legacy options that are no longer needed.

### Current State

The Settings dropdown currently shows:
- Dashboard
- Create
- App
- Docker Operations

### Proposed Change

Remove all four outdated menu items from the Settings dropdown. The Settings button should link directly to the settings page without a dropdown menu.

### Implementation

#### Update Navigation Template

Modify `/templates/components/navigation.html`:

```html
<!-- Before: Dropdown button -->
<li class="nav-item dropdown">
    <a class="nav-link dropdown-toggle" href="#" id="settingsDropdown" 
       role="button" data-bs-toggle="dropdown" aria-expanded="false">
        <i class="bi bi-gear"></i> Settings
    </a>
    <ul class="dropdown-menu" aria-labelledby="settingsDropdown">
        <li><a class="dropdown-item" href="/dashboard">Dashboard</a></li>
        <li><a class="dropdown-item" href="/create">Create</a></li>
        <li><a class="dropdown-item" href="/app">App</a></li>
        <li><a class="dropdown-item" href="/docker-operations">Docker Operations</a></li>
    </ul>
</li>

<!-- After: Direct link -->
<li class="nav-item">
    <a class="nav-link" href="/settings">
        <i class="bi bi-gear"></i> Settings
    </a>
</li>
```

### Benefits

1. **Cleaner Navigation**: Removes confusing/outdated options
2. **Faster Access**: Direct link to settings without dropdown interaction
3. **Consistency**: Aligns with modern single-page settings approach

### Migration Notes

- Ensure any routes for the removed pages are properly deprecated
- Update any documentation that references these menu items
- Consider redirecting old URLs to appropriate new locations if needed

## 5. App Cards Click Behavior

### Overview

Improve the user experience on the main page by making entire app cards clickable and adding iOS-style visual indicators. Remove the separate "Manage" button to streamline the interface.

### Current State

- App cards have a separate "Manage" button
- Only the button is clickable
- No visual indication that cards could be interactive

### Proposed Changes

1. **Remove "Manage" button** from each app card
2. **Make entire card clickable** to navigate to app detail page
3. **Add iOS-style chevron** on the right side of each card
4. **Preserve existing hover effect** (highlighting)

### Implementation

#### Update App Card Template

Modify the app card structure in `/templates/dashboard.html` or relevant template:

```html
<!-- Before: Card with Manage button -->
<div class="col-md-6 col-lg-4 mb-4">
    <div class="card app-card h-100">
        <div class="card-body">
            <h5 class="card-title">{{ .Name }}</h5>
            <p class="card-text">{{ .Status }}</p>
            <a href="/apps/{{ .Name }}" class="btn btn-primary">Manage</a>
        </div>
    </div>
</div>

<!-- After: Clickable card with chevron -->
<div class="col-md-6 col-lg-4 mb-4">
    <a href="/apps/{{ .Name }}" class="text-decoration-none">
        <div class="card app-card h-100 clickable-card">
            <div class="card-body d-flex justify-content-between align-items-center">
                <div>
                    <h5 class="card-title mb-1">{{ .Emoji }} {{ .Name }}</h5>
                    <p class="card-text text-muted mb-0">{{ .Status }}</p>
                </div>
                <i class="bi bi-chevron-right text-muted"></i>
            </div>
        </div>
    </a>
</div>
```

#### CSS Styling

Add to `static/css/style.css`:

```css
/* Clickable card styling */
.clickable-card {
    cursor: pointer;
    transition: all 0.2s ease-in-out;
}

.clickable-card:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

/* iOS-style chevron */
.clickable-card .bi-chevron-right {
    font-size: 1.2rem;
    opacity: 0.5;
    transition: opacity 0.2s ease-in-out;
}

.clickable-card:hover .bi-chevron-right {
    opacity: 0.8;
}

/* Ensure text doesn't change color on hover */
a.text-decoration-none:hover .card-title,
a.text-decoration-none:hover .card-text {
    color: inherit;
}
```

### Visual Design

- **Chevron Icon**: Bootstrap Icons `bi-chevron-right` in muted color
- **Hover Effect**: Slight elevation and enhanced shadow
- **Cursor**: Pointer cursor on entire card
- **Text**: No underline or color change on hover

### Benefits

1. **Larger Click Target**: Entire card is clickable (better accessibility)
2. **Cleaner Design**: Removes redundant button
3. **Familiar Pattern**: iOS-style indicator is widely recognized
4. **Improved UX**: Clear visual feedback on hover

### Accessibility Considerations

- Ensure proper focus states for keyboard navigation
- Add appropriate ARIA labels if needed
- Maintain sufficient color contrast for the chevron icon