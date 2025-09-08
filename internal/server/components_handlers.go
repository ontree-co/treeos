package server

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"treeos/internal/yamlutil"
)

// routeComponents routes all /components/* requests to the appropriate handler
func (s *Server) routeComponents(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Route based on the path pattern
	switch {
	case path == "/components/emoji-picker/shuffle":
		s.handleEmojiPickerShuffle(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleEmojiPickerShuffle returns 7 random emojis for the emoji picker
func (s *Server) handleEmojiPickerShuffle(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if this is a demo request (from pattern library)
	isDemo := r.URL.Query().Get("demo") == "true"
	pickerID := "emoji-picker"
	inputID := "selected-emoji"
	onclickFunc := "selectEmoji"

	if isDemo {
		pickerID = "emoji-picker-demo"
		inputID = "demo-selected-emoji"
		onclickFunc = "selectEmojiDemo"
	}

	// Get 7 random emojis
	randomEmojis := getRandomEmojis(7)

	// Build HTML response for HTMX - complete emoji picker structure
	var html strings.Builder
	html.WriteString(fmt.Sprintf(`<div class="emoji-picker" id="%s">`, pickerID))
	html.WriteString(`<label class="form-label">Choose an emoji for your app</label>`)
	html.WriteString(`<div class="emoji-grid mb-3">`)

	for _, emoji := range randomEmojis {
		html.WriteString(fmt.Sprintf(
			`<button type="button" class="emoji-option" data-emoji="%s" onclick="%s(this)" aria-label="Select emoji %s">%s</button>`,
			emoji, onclickFunc, emoji, emoji,
		))
	}

	html.WriteString(`</div>`)

	// Build the shuffle button URL with demo parameter if needed
	shuffleURL := "/components/emoji-picker/shuffle"
	if isDemo {
		shuffleURL += "?demo=true"
	}

	html.WriteString(fmt.Sprintf(`<button type="button" class="btn btn-sm btn-secondary" hx-get="%s" hx-target="#%s" hx-swap="outerHTML">`, shuffleURL, pickerID))
	html.WriteString(`🔀 Shuffle`)
	html.WriteString(`</button>`)
	html.WriteString(fmt.Sprintf(`<input type="hidden" name="%s" id="%s" value="">`,
		map[bool]string{true: "demo-emoji", false: "emoji"}[isDemo], inputID))
	html.WriteString(`</div>`)

	// Include the JavaScript for emoji selection (only for non-demo)
	if !isDemo {
		html.WriteString(`<script>
function selectEmoji(button) {
    // Remove selected class from all emoji buttons
    document.querySelectorAll('.emoji-option').forEach(btn => {
        btn.classList.remove('selected');
    });
    
    // Add selected class to clicked button
    button.classList.add('selected');
    
    // Update hidden input value
    const emoji = button.getAttribute('data-emoji');
    document.getElementById('selected-emoji').value = emoji;
}
</script>`)
	}

	// Return HTML fragment
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(html.String())); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// getRandomEmojis returns n random emojis from the curated list
func getRandomEmojis(n int) []string {
	// Create a random source - using math/rand is fine for emoji selection
	rng := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404 - weak RNG is acceptable for UI emoji selection

	// Copy the emoji list to avoid modifying the original
	emojis := make([]string, len(yamlutil.AppEmojis))
	copy(emojis, yamlutil.AppEmojis)

	// Shuffle the copy
	rng.Shuffle(len(emojis), func(i, j int) {
		emojis[i], emojis[j] = emojis[j], emojis[i]
	})

	// Return the first n emojis
	if n > len(emojis) {
		n = len(emojis)
	}

	return emojis[:n]
}
