/**
 * TreeOS Theme Toggle
 * Manages light/dark mode switching with localStorage persistence
 */

(function() {
  'use strict';

  const STORAGE_KEY = 'treeos-theme';
  const THEME_LIGHT = 'light';
  const THEME_DARK = 'dark';

  /**
   * Get the current theme from localStorage or default to light
   */
  function getCurrentTheme() {
    return localStorage.getItem(STORAGE_KEY) || THEME_LIGHT;
  }

  /**
   * Apply theme to the document
   */
  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem(STORAGE_KEY, theme);
    updateToggleButtons(theme);
  }

  /**
   * Update all theme toggle buttons to reflect current state
   */
  function updateToggleButtons(theme) {
    const buttons = document.querySelectorAll('[data-theme-toggle]');
    const isDark = theme === THEME_DARK;

    buttons.forEach(button => {
      button.setAttribute('aria-label', isDark ? 'Switch to light mode' : 'Switch to dark mode');
      button.setAttribute('aria-pressed', isDark ? 'true' : 'false');
    });
  }

  /**
   * Toggle between light and dark themes
   */
  function toggleTheme() {
    const currentTheme = getCurrentTheme();
    const newTheme = currentTheme === THEME_LIGHT ? THEME_DARK : THEME_LIGHT;
    applyTheme(newTheme);

    // Dispatch custom event for other scripts that might need to know
    window.dispatchEvent(new CustomEvent('theme-changed', {
      detail: { theme: newTheme }
    }));
  }

  /**
   * Initialize theme on page load
   */
  function initTheme() {
    const savedTheme = getCurrentTheme();
    applyTheme(savedTheme);
  }

  /**
   * Set up event listeners for theme toggle buttons
   */
  function setupEventListeners() {
    // Wait for DOM to be ready
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', attachListeners);
    } else {
      attachListeners();
    }
  }

  function attachListeners() {
    const buttons = document.querySelectorAll('[data-theme-toggle]');
    buttons.forEach(button => {
      button.addEventListener('click', function(event) {
        event.preventDefault();
        toggleTheme();
      });
      button.addEventListener('keydown', function(event) {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          toggleTheme();
        }
      });
    });
  }

  // Initialize theme immediately (before page renders)
  initTheme();

  // Set up listeners when DOM is ready
  setupEventListeners();

  // Expose toggle function globally for inline onclick handlers
  window.toggleTheme = toggleTheme;

})();
