// UI Helper Functions for Loading States, Empty States, and Accessibility

/**
 * Loading States
 */

// Show loading spinner in element
function showLoading(elementId, message = 'Indlæser...') {
  const element = document.getElementById(elementId);
  if (!element) return;
  
  element.innerHTML = `
    <div class="loading-container">
      <div class="spinner"></div>
      <p>${message}</p>
    </div>
  `;
  element.setAttribute('aria-busy', 'true');
}

// Show skeleton screens for devices
function showDevicesSkeleton(elementId, count = 3) {
  const element = document.getElementById(elementId);
  if (!element) return;
  
  let html = '';
  for (let i = 0; i < count; i++) {
    html += `
      <div class="skeleton-device">
        <div class="skeleton skeleton-device-icon"></div>
        <div class="skeleton-device-content">
          <div class="skeleton skeleton-text" style="width: 60%;"></div>
          <div class="skeleton skeleton-text-sm" style="width: 40%;"></div>
        </div>
        <div class="skeleton-device-actions">
          <div class="skeleton skeleton-button"></div>
        </div>
      </div>
    `;
  }
  element.innerHTML = html;
  element.setAttribute('aria-busy', 'true');
}

// Show skeleton screens for users
function showUsersSkeleton(elementId, count = 3) {
  const element = document.getElementById(elementId);
  if (!element) return;
  
  let html = '';
  for (let i = 0; i < count; i++) {
    html += `
      <div class="skeleton-user">
        <div class="skeleton skeleton-user-avatar"></div>
        <div class="skeleton-user-content">
          <div class="skeleton skeleton-text" style="width: 70%;"></div>
          <div class="skeleton skeleton-text-sm" style="width: 50%;"></div>
        </div>
        <div class="skeleton-user-actions">
          <div class="skeleton skeleton-button"></div>
          <div class="skeleton skeleton-button"></div>
        </div>
      </div>
    `;
  }
  element.innerHTML = html;
  element.setAttribute('aria-busy', 'true');
}

// Hide loading state
function hideLoading(elementId) {
  const element = document.getElementById(elementId);
  if (!element) return;
  
  element.setAttribute('aria-busy', 'false');
}

// Add loading state to button
function setButtonLoading(buttonId, loading = true) {
  const button = document.getElementById(buttonId);
  if (!button) return;
  
  if (loading) {
    button.classList.add('btn-loading');
    button.disabled = true;
    button.setAttribute('aria-busy', 'true');
  } else {
    button.classList.remove('btn-loading');
    button.disabled = false;
    button.setAttribute('aria-busy', 'false');
  }
}

/**
 * Empty States
 */

// Show empty state for devices
function showEmptyDevices(elementId) {
  const element = document.getElementById(elementId);
  if (!element) return;
  
  element.innerHTML = `
    <div class="empty-state" role="status">
      <div class="empty-state-icon">📱</div>
      <h3>Ingen enheder endnu</h3>
      <p>Download og kør agenten på en computer for at komme i gang</p>
      <div class="empty-state-actions">
        <a href="#" onclick="signedDownload('remote-agent.exe'); return false;" 
           class="btn btn-primary">
          🖥️ Hent Windows Agent
        </a>
        <a href="agent.html" 
           class="btn btn-secondary">
          🌐 Web Agent
        </a>
      </div>
    </div>
  `;
  element.setAttribute('aria-busy', 'false');
}

// Show empty state for users
function showEmptyUsers(elementId) {
  const element = document.getElementById(elementId);
  if (!element) return;
  
  element.innerHTML = `
    <div class="empty-state" role="status">
      <div class="empty-state-icon">👥</div>
      <h3>Ingen brugere fundet</h3>
      <p>Der er ingen brugere der matcher dine filtre</p>
    </div>
  `;
  element.setAttribute('aria-busy', 'false');
}

// Show empty state for invitations
function showEmptyInvitations(elementId) {
  const element = document.getElementById(elementId);
  if (!element) return;
  
  element.innerHTML = `
    <div class="empty-state" role="status">
      <div class="empty-state-icon">✉️</div>
      <h3>Ingen invitationer sendt</h3>
      <p>Send en invitation for at invitere nye brugere</p>
    </div>
  `;
  element.setAttribute('aria-busy', 'false');
}

// Generic empty state
function showEmptyState(elementId, icon, title, description) {
  const element = document.getElementById(elementId);
  if (!element) return;
  
  element.innerHTML = `
    <div class="empty-state" role="status">
      <div class="empty-state-icon">${icon}</div>
      <h3>${title}</h3>
      <p>${description}</p>
    </div>
  `;
  element.setAttribute('aria-busy', 'false');
}

/**
 * Accessibility Helpers
 */

// Announce to screen readers
function announceToScreenReader(message, priority = 'polite') {
  const liveRegion = document.getElementById('live-region') || createLiveRegion();
  liveRegion.setAttribute('aria-live', priority);
  liveRegion.textContent = message;
  
  // Clear after 1 second
  setTimeout(() => {
    liveRegion.textContent = '';
  }, 1000);
}

// Create live region for screen reader announcements
function createLiveRegion() {
  let liveRegion = document.getElementById('live-region');
  if (!liveRegion) {
    liveRegion = document.createElement('div');
    liveRegion.id = 'live-region';
    liveRegion.className = 'live-region';
    liveRegion.setAttribute('aria-live', 'polite');
    liveRegion.setAttribute('aria-atomic', 'true');
    document.body.appendChild(liveRegion);
  }
  return liveRegion;
}

// Keyboard shortcuts manager
const keyboardShortcuts = {
  shortcuts: {},
  
  register(key, callback, description) {
    this.shortcuts[key] = { callback, description };
  },
  
  unregister(key) {
    delete this.shortcuts[key];
  },
  
  handle(event) {
    if (!event.key) return;
    const key = event.key.toLowerCase();
    const ctrl = event.ctrlKey || event.metaKey;
    const shift = event.shiftKey;
    const alt = event.altKey;
    
    let shortcutKey = '';
    if (ctrl) shortcutKey += 'ctrl+';
    if (shift) shortcutKey += 'shift+';
    if (alt) shortcutKey += 'alt+';
    shortcutKey += key;
    
    if (this.shortcuts[shortcutKey]) {
      event.preventDefault();
      this.shortcuts[shortcutKey].callback(event);
      return true;
    }
    return false;
  },
  
  showHelp() {
    const modal = document.createElement('div');
    modal.className = 'keyboard-shortcuts show';
    modal.setAttribute('role', 'dialog');
    modal.setAttribute('aria-labelledby', 'shortcuts-title');
    modal.setAttribute('aria-modal', 'true');
    
    let shortcutsHTML = '<div class="keyboard-shortcuts-list">';
    for (const [key, data] of Object.entries(this.shortcuts)) {
      const keys = key.split('+').map(k => `<kbd>${k}</kbd>`).join(' + ');
      shortcutsHTML += `
        <div class="keyboard-shortcut">
          <div class="keyboard-shortcut-keys">${keys}</div>
          <div class="keyboard-shortcut-description">${data.description}</div>
        </div>
      `;
    }
    shortcutsHTML += '</div>';
    
    modal.innerHTML = `
      <h2 id="shortcuts-title">Tastaturgenveje</h2>
      ${shortcutsHTML}
      <button class="keyboard-shortcuts-close" aria-label="Luk" onclick="this.parentElement.remove()">×</button>
    `;
    
    document.body.appendChild(modal);
    
    // Focus trap
    const focusableElements = modal.querySelectorAll('button');
    const firstFocusable = focusableElements[0];
    const lastFocusable = focusableElements[focusableElements.length - 1];
    
    firstFocusable.focus();
    
    modal.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        modal.remove();
      }
      
      if (e.key === 'Tab') {
        if (e.shiftKey) {
          if (document.activeElement === firstFocusable) {
            e.preventDefault();
            lastFocusable.focus();
          }
        } else {
          if (document.activeElement === lastFocusable) {
            e.preventDefault();
            firstFocusable.focus();
          }
        }
      }
    });
  }
};

// Initialize keyboard shortcuts
document.addEventListener('keydown', (e) => {
  keyboardShortcuts.handle(e);
});

// Register common shortcuts
keyboardShortcuts.register('?', () => keyboardShortcuts.showHelp(), 'Vis tastaturgenveje');
keyboardShortcuts.register('escape', () => {
  // Close any open modals
  document.querySelectorAll('.modal, .keyboard-shortcuts').forEach(el => el.remove());
}, 'Luk modal/dialog');

/**
 * Focus Management
 */

// Trap focus within element
function trapFocus(element) {
  const focusableElements = element.querySelectorAll(
    'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
  );
  
  const firstFocusable = focusableElements[0];
  const lastFocusable = focusableElements[focusableElements.length - 1];
  
  element.addEventListener('keydown', (e) => {
    if (e.key === 'Tab') {
      if (e.shiftKey) {
        if (document.activeElement === firstFocusable) {
          e.preventDefault();
          lastFocusable.focus();
        }
      } else {
        if (document.activeElement === lastFocusable) {
          e.preventDefault();
          firstFocusable.focus();
        }
      }
    }
  });
  
  firstFocusable.focus();
}

// Return focus to previous element
let previousFocusedElement = null;

function saveFocus() {
  previousFocusedElement = document.activeElement;
}

function restoreFocus() {
  if (previousFocusedElement) {
    previousFocusedElement.focus();
    previousFocusedElement = null;
  }
}

/**
 * Animation Helpers
 */

// Fade in elements with stagger
function fadeInStagger(elements, delay = 50) {
  elements.forEach((element, index) => {
    element.style.opacity = '0';
    element.style.transform = 'translateY(10px)';
    
    setTimeout(() => {
      element.style.transition = 'opacity 0.3s ease, transform 0.3s ease';
      element.style.opacity = '1';
      element.style.transform = 'translateY(0)';
    }, index * delay);
  });
}

// Check if user prefers reduced motion
function prefersReducedMotion() {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

/**
 * Toast Notifications
 */

// Toast container (created once, toasts stack inside it)
let _toastContainer = null;
function _getToastContainer() {
  if (!_toastContainer || !document.body.contains(_toastContainer)) {
    _toastContainer = document.createElement('div');
    _toastContainer.id = 'toast-container';
    _toastContainer.setAttribute('aria-live', 'polite');
    _toastContainer.setAttribute('aria-relevant', 'additions');
    document.body.appendChild(_toastContainer);
  }
  return _toastContainer;
}

function showToast(message, type = 'info', duration = 4000) {
  const container = _getToastContainer();
  
  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.setAttribute('role', 'status');
  
  const icons = {
    success: '<svg width="20" height="20" viewBox="0 0 20 20" fill="none"><circle cx="10" cy="10" r="10" fill="currentColor" opacity="0.15"/><path d="M6 10l3 3 5-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    error: '<svg width="20" height="20" viewBox="0 0 20 20" fill="none"><circle cx="10" cy="10" r="10" fill="currentColor" opacity="0.15"/><path d="M7 7l6 6M13 7l-6 6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>',
    warning: '<svg width="20" height="20" viewBox="0 0 20 20" fill="none"><circle cx="10" cy="10" r="10" fill="currentColor" opacity="0.15"/><path d="M10 6v5M10 13.5v.5" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>',
    info: '<svg width="20" height="20" viewBox="0 0 20 20" fill="none"><circle cx="10" cy="10" r="10" fill="currentColor" opacity="0.15"/><path d="M10 9v5M10 6.5v.5" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>'
  };
  
  toast.innerHTML = `
    <span class="toast-icon">${icons[type] || icons.info}</span>
    <span class="toast-message">${message}</span>
    <button class="toast-close" aria-label="Luk">&times;</button>
  `;
  
  // Close button handler
  toast.querySelector('.toast-close').addEventListener('click', () => dismissToast(toast));
  
  container.appendChild(toast);
  
  // Announce to screen readers
  announceToScreenReader(message);
  
  // Animate in
  requestAnimationFrame(() => {
    requestAnimationFrame(() => toast.classList.add('show'));
  });
  
  // Auto-dismiss (0 = persistent)
  if (duration > 0) {
    toast._timeout = setTimeout(() => dismissToast(toast), duration);
  }
  
  // Pause auto-dismiss on hover
  toast.addEventListener('mouseenter', () => {
    if (toast._timeout) clearTimeout(toast._timeout);
  });
  toast.addEventListener('mouseleave', () => {
    if (duration > 0) {
      toast._timeout = setTimeout(() => dismissToast(toast), 2000);
    }
  });
  
  return toast;
}

function dismissToast(toast) {
  if (!toast || !toast.parentNode) return;
  toast.classList.remove('show');
  toast.classList.add('toast-exit');
  setTimeout(() => toast.remove(), 300);
}

/**
 * Confirm Modal (replaces native confirm())
 * Returns a Promise<boolean>
 */
function showConfirm(message, options = {}) {
  const {
    title = 'Bekræft',
    confirmText = 'Bekræft',
    cancelText = 'Annuller',
    type = 'warning',  // warning, danger, info
    icon = null
  } = options;
  
  return new Promise((resolve) => {
    const overlay = document.createElement('div');
    overlay.className = 'confirm-overlay';
    
    const defaultIcons = {
      warning: '⚠️',
      danger: '🗑️',
      info: 'ℹ️'
    };
    const displayIcon = icon || defaultIcons[type] || '❓';
    
    const confirmBtnClass = type === 'danger' ? 'btn-danger' : 'btn-primary';
    
    overlay.innerHTML = `
      <div class="confirm-modal" role="alertdialog" aria-modal="true" aria-labelledby="confirm-title" aria-describedby="confirm-message">
        <div class="confirm-icon">${displayIcon}</div>
        <h3 id="confirm-title" class="confirm-title">${title}</h3>
        <p id="confirm-message" class="confirm-message">${message}</p>
        <div class="confirm-actions">
          <button class="btn btn-ghost confirm-cancel">${cancelText}</button>
          <button class="btn ${confirmBtnClass} confirm-ok">${confirmText}</button>
        </div>
      </div>
    `;
    
    const cleanup = (result) => {
      overlay.classList.add('confirm-exit');
      setTimeout(() => overlay.remove(), 200);
      resolve(result);
    };
    
    overlay.querySelector('.confirm-cancel').addEventListener('click', () => cleanup(false));
    overlay.querySelector('.confirm-ok').addEventListener('click', () => cleanup(true));
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) cleanup(false);
    });
    
    document.body.appendChild(overlay);
    
    // Animate in
    requestAnimationFrame(() => overlay.classList.add('show'));
    
    // Focus confirm button
    overlay.querySelector('.confirm-ok').focus();
    
    // Escape to cancel
    const escHandler = (e) => {
      if (e.key === 'Escape') {
        document.removeEventListener('keydown', escHandler);
        cleanup(false);
      }
    };
    document.addEventListener('keydown', escHandler);
  });
}

/**
 * Progress Indicator
 */

function showProgress(elementId, percent) {
  const element = document.getElementById(elementId);
  if (!element) return;
  
  element.innerHTML = `
    <div class="progress-bar" role="progressbar" aria-valuenow="${percent}" aria-valuemin="0" aria-valuemax="100">
      <div class="progress-bar-fill" style="width: ${percent}%"></div>
    </div>
  `;
}

function showIndeterminateProgress(elementId) {
  const element = document.getElementById(elementId);
  if (!element) return;
  
  element.innerHTML = `
    <div class="progress-bar">
      <div class="progress-bar-indeterminate"></div>
    </div>
  `;
}

/**
 * Dark / Light Theme Toggle
 */

function initThemeToggle() {
  // Restore saved preference or default to dark
  const saved = localStorage.getItem('theme');
  if (saved === 'light') {
    document.documentElement.setAttribute('data-theme', 'light');
  }

  // Find or create toggle button in header
  const header = document.querySelector('.header-actions');
  if (!header) return;

  const btn = document.createElement('button');
  btn.className = 'theme-toggle';
  btn.title = 'Skift tema';
  btn.setAttribute('aria-label', 'Skift mellem lyst og mørkt tema');
  updateThemeIcon(btn);

  btn.addEventListener('click', () => {
    const current = document.documentElement.getAttribute('data-theme');
    const next = current === 'light' ? 'dark' : 'light';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('theme', next);
    updateThemeIcon(btn);
  });

  header.insertBefore(btn, header.firstChild);
}

function updateThemeIcon(btn) {
  const isLight = document.documentElement.getAttribute('data-theme') === 'light';
  btn.textContent = isLight ? '🌙' : '☀️';
}

/**
 * Password Strength Indicator
 */

function getPasswordStrength(password) {
  let score = 0;
  if (password.length >= 6) score++;
  if (password.length >= 10) score++;
  if (/[a-z]/.test(password) && /[A-Z]/.test(password)) score++;
  if (/\d/.test(password)) score++;
  if (/[^a-zA-Z0-9]/.test(password)) score++;

  const levels = [
    { label: 'Meget svag', color: '#ef4444', width: 20 },
    { label: 'Svag', color: '#f97316', width: 40 },
    { label: 'Middel', color: '#eab308', width: 60 },
    { label: 'Stærk', color: '#22c55e', width: 80 },
    { label: 'Meget stærk', color: '#10b981', width: 100 }
  ];
  return levels[Math.min(score, levels.length - 1)];
}

function initPasswordStrength(inputId, meterId) {
  const input = document.getElementById(inputId);
  const meter = document.getElementById(meterId);
  if (!input || !meter) return;

  meter.innerHTML = '<div class="pw-bar"><div class="pw-fill"></div></div><span class="pw-label"></span>';
  meter.style.display = 'none';

  input.addEventListener('input', () => {
    const val = input.value;
    if (!val) { meter.style.display = 'none'; return; }
    meter.style.display = 'flex';
    const s = getPasswordStrength(val);
    const fill = meter.querySelector('.pw-fill');
    const label = meter.querySelector('.pw-label');
    fill.style.width = s.width + '%';
    fill.style.background = s.color;
    label.textContent = s.label;
    label.style.color = s.color;
  });
}

/**
 * Initialize UI Helpers
 */

// Create skip to main content link
function initSkipLink() {
  const skipLink = document.createElement('a');
  skipLink.href = '#main-content';
  skipLink.className = 'skip-to-main';
  skipLink.textContent = 'Spring til hovedindhold';
  document.body.insertBefore(skipLink, document.body.firstChild);
  
  // Add main content ID if not exists
  const main = document.querySelector('main') || document.querySelector('.dashboard-main') || document.querySelector('.admin-container');
  if (main && !main.id) {
    main.id = 'main-content';
    main.setAttribute('tabindex', '-1');
  }
}

// Initialize on load
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', () => {
    initSkipLink();
    createLiveRegion();
    initThemeToggle();
  });
} else {
  initSkipLink();
  createLiveRegion();
  initThemeToggle();
}

// Export functions for use in other scripts
window.UIHelpers = {
  showLoading,
  hideLoading,
  showDevicesSkeleton,
  showUsersSkeleton,
  setButtonLoading,
  showEmptyDevices,
  showEmptyUsers,
  showEmptyInvitations,
  showEmptyState,
  announceToScreenReader,
  keyboardShortcuts,
  trapFocus,
  saveFocus,
  restoreFocus,
  fadeInStagger,
  prefersReducedMotion,
  showToast,
  dismissToast,
  showConfirm,
  showProgress,
  showIndeterminateProgress,
  initPasswordStrength,
  initThemeToggle
};

console.log('✅ UI Helpers initialized');
