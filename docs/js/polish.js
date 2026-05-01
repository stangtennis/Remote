// Dashboard polish v1.1 — 2026-04-22
// - Caps Lock warning on password fields
// - Live password requirements checklist
// - Login brute-force UI lockout (client-side rate feedback)
// - Auto-logout on inactivity (30 min)
// - Keyboard shortcuts modal (Ctrl+/)
// - Multi-tab session sync via BroadcastChannel
// - Dark/light theme toggle (v1.1)

(function() {
  'use strict';

  // ===================== THEME TOGGLE =====================
  // Sets data-theme="dark" | "light" on <html>.
  // Persists to localStorage. Mørk er ALTID default — vi følger ikke
  // prefers-color-scheme automatisk (brugeren kan stadig vælge light).
  function setupThemeToggle() {
    const STORAGE_KEY = 'rd-theme';

    function resolveInitial() {
      try {
        const saved = localStorage.getItem(STORAGE_KEY);
        if (saved === 'light' || saved === 'dark') return saved;
      } catch (_) {}
      return 'dark';
    }

    function apply(theme) {
      document.documentElement.setAttribute('data-theme', theme);
      updateIcon(theme);
    }

    function updateIcon(theme) {
      document.querySelectorAll('.theme-toggle-btn i').forEach(i => {
        i.className = theme === 'light' ? 'fas fa-moon' : 'fas fa-sun';
      });
      document.querySelectorAll('.theme-toggle-btn').forEach(b => {
        b.setAttribute('title', theme === 'light' ? 'Skift til mørkt tema' : 'Skift til lyst tema');
        b.setAttribute('aria-label', b.getAttribute('title'));
      });
    }

    function toggle() {
      const current = document.documentElement.getAttribute('data-theme') || 'dark';
      const next = current === 'light' ? 'dark' : 'light';
      try { localStorage.setItem(STORAGE_KEY, next); } catch (_) {}
      apply(next);
      // Broadcast til andre tabs
      try {
        if (window._rdThemeChannel) window._rdThemeChannel.postMessage({ type: 'theme', value: next });
      } catch (_) {}
    }

    function ensureButton() {
      // Prøv først at finde eksisterende header-actions (dashboard/admin)
      const headerActions = document.querySelector('.header-actions');
      const existing = document.querySelector('.theme-toggle-btn');
      if (existing) return existing;

      const btn = document.createElement('button');
      btn.className = 'theme-toggle-btn';
      btn.type = 'button';
      btn.innerHTML = '<i class="fas fa-sun"></i>';

      if (headerActions) {
        // Indsæt som første element i header-actions
        headerActions.insertBefore(btn, headerActions.firstChild);
      } else {
        // Login eller andre sider: floating øverst højre
        btn.classList.add('floating');
        document.body.appendChild(btn);
      }

      btn.addEventListener('click', toggle);
      return btn;
    }

    // Apply initial theme ASAP for no flash — tema sættes allerede via inline snippet i HTML
    const initial = document.documentElement.getAttribute('data-theme') || resolveInitial();
    apply(initial);
    ensureButton();

    // Sync på tværs af tabs via BroadcastChannel
    try {
      if ('BroadcastChannel' in window) {
        window._rdThemeChannel = new BroadcastChannel('rd-theme');
        window._rdThemeChannel.onmessage = (ev) => {
          if (ev.data && ev.data.type === 'theme' && (ev.data.value === 'light' || ev.data.value === 'dark')) {
            apply(ev.data.value);
          }
        };
      }
    } catch (_) {}
  }

  // ===================== CAPS LOCK WARNING =====================
  function setupCapsLockWarning() {
    document.querySelectorAll('input[type="password"]').forEach(input => {
      if (input.dataset.capsLockHooked) return;
      input.dataset.capsLockHooked = '1';

      const warn = document.createElement('span');
      warn.className = 'caps-lock-warning';
      warn.innerHTML = '<i class="fas fa-arrow-up"></i> Caps Lock';
      warn.setAttribute('role', 'status');
      warn.setAttribute('aria-live', 'polite');
      // Insert after the input's parent form-group (password-toggle button is absolute)
      input.parentElement && input.parentElement.appendChild(warn);

      const check = (e) => {
        try {
          const on = e.getModifierState && e.getModifierState('CapsLock');
          warn.classList.toggle('visible', !!on);
        } catch (_) {}
      };
      input.addEventListener('keydown', check);
      input.addEventListener('keyup', check);
      input.addEventListener('blur', () => warn.classList.remove('visible'));
    });
  }

  // ===================== LIVE PASSWORD REQUIREMENTS =====================
  function setupPasswordRequirements() {
    const signupPw = document.getElementById('signupPassword');
    if (!signupPw || signupPw.dataset.reqHooked) return;
    signupPw.dataset.reqHooked = '1';

    const wrapper = document.createElement('div');
    wrapper.className = 'pw-requirements';
    wrapper.setAttribute('aria-live', 'polite');
    wrapper.innerHTML = `
      <ul>
        <li data-req="len">Mindst 8 tegn</li>
        <li data-req="upper">Stort bogstav</li>
        <li data-req="lower">Lille bogstav</li>
        <li data-req="digit">Et tal</li>
        <li data-req="symbol">Specialtegn</li>
        <li data-req="nospaces">Ingen mellemrum</li>
      </ul>
    `;
    // Insert after the meter if present, else after the input
    const meter = document.getElementById('pwStrength');
    const anchor = meter || signupPw;
    anchor.parentElement.insertBefore(wrapper, anchor.nextSibling);

    const tests = {
      len: v => v.length >= 8,
      upper: v => /[A-ZÆØÅ]/.test(v),
      lower: v => /[a-zæøå]/.test(v),
      digit: v => /[0-9]/.test(v),
      symbol: v => /[^A-Za-zÆØÅæøå0-9]/.test(v),
      nospaces: v => v.length > 0 && !/\s/.test(v),
    };

    const update = () => {
      const val = signupPw.value;
      if (!val) { wrapper.classList.remove('visible'); return; }
      wrapper.classList.add('visible');
      wrapper.querySelectorAll('li').forEach(li => {
        const k = li.dataset.req;
        li.classList.toggle('met', tests[k](val));
      });
    };
    signupPw.addEventListener('input', update);
    signupPw.addEventListener('focus', update);
    signupPw.addEventListener('blur', () => {
      if (!signupPw.value) wrapper.classList.remove('visible');
    });
  }

  // ===================== LOGIN LOCKOUT (client-side UI) =====================
  // Note: This is UI feedback only. Supabase enforces server-side rate limits.
  // Keeps attempt count in sessionStorage; lockout = 30s exponential up to 5 min.
  function setupLoginLockout() {
    const form = document.getElementById('loginForm');
    if (!form || form.dataset.lockoutHooked) return;
    form.dataset.lockoutHooked = '1';

    const STATE_KEY = 'rd-login-attempts';
    const banner = document.createElement('div');
    banner.className = 'login-lockout-banner';
    banner.setAttribute('role', 'alert');
    form.parentElement.insertBefore(banner, form);

    function getState() {
      try { return JSON.parse(sessionStorage.getItem(STATE_KEY)) || { count: 0, until: 0 }; }
      catch { return { count: 0, until: 0 }; }
    }
    function saveState(s) {
      try { sessionStorage.setItem(STATE_KEY, JSON.stringify(s)); } catch {}
    }

    function updateBanner() {
      const s = getState();
      const now = Date.now();
      if (s.until > now) {
        const remain = Math.ceil((s.until - now) / 1000);
        banner.innerHTML = `<i class="fas fa-shield-halved"></i> For mange login-forsøg. Prøv igen om <strong>${remain}s</strong>.`;
        banner.classList.add('visible');
        const loginBtn = document.getElementById('loginBtn');
        if (loginBtn) loginBtn.disabled = true;
        setTimeout(updateBanner, 1000);
      } else {
        banner.classList.remove('visible');
        const loginBtn = document.getElementById('loginBtn');
        if (loginBtn) loginBtn.disabled = false;
      }
    }
    updateBanner();

    // Wrap submit to detect failures
    const origSubmit = form.onsubmit;
    form.addEventListener('submit', () => {
      const s = getState();
      if (s.until > Date.now()) return; // already locked
    }, true);

    // Intercept error messages — we know auth.js writes to authMessage on failure
    const authMessage = document.getElementById('authMessage');
    if (authMessage) {
      const mo = new MutationObserver(() => {
        if (authMessage.style.display !== 'none'
            && authMessage.className.includes('error')
            && authMessage.textContent.toLowerCase().includes('forkert')) {
          const s = getState();
          s.count = (s.count || 0) + 1;
          if (s.count >= 3) {
            const lockSec = Math.min(300, 30 * Math.pow(2, s.count - 3));
            s.until = Date.now() + lockSec * 1000;
          }
          saveState(s);
          updateBanner();
        }
      });
      mo.observe(authMessage, { attributes: true, childList: true, subtree: true });
    }
  }

  // ===================== AUTO-LOGOUT ON INACTIVITY =====================
  function setupAutoLogout() {
    const logoutBtn = document.getElementById('logoutBtn');
    if (!logoutBtn) return; // only run on pages with logout (dashboard/admin)

    const IDLE_MINUTES = 30;
    const WARNING_MINUTES = 1; // show warning 1 min before logout
    let lastActivity = Date.now();
    let warningShown = false;

    // Inject toast
    const toast = document.createElement('div');
    toast.className = 'idle-warning-toast';
    toast.setAttribute('role', 'alert');
    document.body.appendChild(toast);

    const resetTimer = () => {
      lastActivity = Date.now();
      if (warningShown) {
        toast.classList.remove('visible');
        warningShown = false;
      }
    };

    ['mousedown', 'keydown', 'touchstart', 'click', 'scroll'].forEach(ev => {
      document.addEventListener(ev, resetTimer, { passive: true, capture: true });
    });

    setInterval(() => {
      const idleMs = Date.now() - lastActivity;
      const idleMin = idleMs / 60000;
      const remainMin = IDLE_MINUTES - idleMin;

      if (remainMin <= 0) {
        // Force logout
        if (window.supabase && window.supabase.auth) {
          window.supabase.auth.signOut().finally(() => {
            window.location.href = 'login.html?status=idle';
          });
        } else {
          window.location.href = 'login.html?status=idle';
        }
      } else if (remainMin <= WARNING_MINUTES && !warningShown) {
        warningShown = true;
        toast.innerHTML = `
          <i class="fas fa-clock"></i>
          Du bliver logget ud om under 1 min pga. inaktivitet.
          <button id="stayLoggedInBtn">Bliv logget ind</button>
        `;
        toast.classList.add('visible');
        const btn = document.getElementById('stayLoggedInBtn');
        if (btn) btn.addEventListener('click', resetTimer);
      }
    }, 10000);
  }

  // ===================== KEYBOARD SHORTCUTS MODAL =====================
  const SHORTCUTS_LOGIN = [
    { cat: 'Login', keys: ['Tab'], desc: 'Skift mellem felter' },
    { cat: 'Login', keys: ['Enter'], desc: 'Log ind' },
    { cat: 'Login', keys: ['Ctrl', '/'], desc: 'Vis denne genvejsliste' },
  ];
  const SHORTCUTS_APP = [
    { cat: 'Generelt', keys: ['Ctrl', '/'], desc: 'Vis denne genvejsliste' },
    { cat: 'Generelt', keys: ['Esc'], desc: 'Luk dialog / forlad fuldskærm' },
    { cat: 'Fjernsession', keys: ['F11'], desc: 'Fuldskærm' },
    { cat: 'Fjernsession', keys: ['Ctrl', 'Alt', 'Del'], desc: 'Send Ctrl+Alt+Del til remote' },
    { cat: 'Filhåndtering', keys: ['F5'], desc: 'Upload fil(er)' },
    { cat: 'Filhåndtering', keys: ['F6'], desc: 'Download valgt' },
    { cat: 'Filhåndtering', keys: ['F7'], desc: 'Ny mappe' },
    { cat: 'Filhåndtering', keys: ['F8'], desc: 'Slet valgt' },
  ];

  function renderShortcuts(filter, list, container) {
    const f = (filter || '').toLowerCase().trim();
    const groups = {};
    list.forEach(s => {
      const hay = (s.desc + ' ' + s.keys.join(' ') + ' ' + s.cat).toLowerCase();
      if (f && !hay.includes(f)) return;
      (groups[s.cat] = groups[s.cat] || []).push(s);
    });
    const cats = Object.keys(groups);
    if (!cats.length) {
      container.innerHTML = '<div class="shortcuts-no-match"><i class="fas fa-search"></i> Ingen genveje matcher</div>';
      return;
    }
    container.innerHTML = cats.map(c => `
      <div class="shortcut-cat">${esc(c)}</div>
      ${groups[c].map(s => `
        <div class="shortcut-item">
          <span class="shortcut-item-desc">${esc(s.desc)}</span>
          <span class="shortcut-item-keys">${s.keys.map((k, i) => `${i > 0 ? '<span class="plus">+</span>' : ''}<kbd>${esc(k)}</kbd>`).join('')}</span>
        </div>
      `).join('')}
    `).join('');
  }

  function esc(s) {
    const d = document.createElement('div');
    d.textContent = String(s);
    return d.innerHTML;
  }

  function setupShortcutsModal() {
    // Create overlay once
    const overlay = document.createElement('div');
    overlay.className = 'shortcuts-overlay';
    overlay.innerHTML = `
      <div class="shortcuts-panel" role="dialog" aria-modal="true" aria-label="Tastaturgenveje">
        <div class="shortcuts-panel-header">
          <h2><i class="fas fa-keyboard"></i> Tastaturgenveje</h2>
          <button class="shortcuts-close" aria-label="Luk">&times;</button>
        </div>
        <div class="shortcuts-panel-body">
          <input type="text" class="shortcuts-search-input" placeholder="Søg genveje..." autocomplete="off">
          <div class="shortcuts-list"></div>
        </div>
      </div>
    `;
    document.body.appendChild(overlay);

    const onLoginPage = !!document.getElementById('loginForm');
    const list = onLoginPage ? SHORTCUTS_LOGIN : SHORTCUTS_APP;
    const listEl = overlay.querySelector('.shortcuts-list');
    const searchEl = overlay.querySelector('.shortcuts-search-input');
    const closeBtn = overlay.querySelector('.shortcuts-close');

    const open = () => {
      searchEl.value = '';
      renderShortcuts('', list, listEl);
      overlay.classList.add('visible');
      setTimeout(() => searchEl.focus(), 30);
    };
    const close = () => overlay.classList.remove('visible');

    closeBtn.addEventListener('click', close);
    overlay.addEventListener('click', (e) => { if (e.target === overlay) close(); });
    searchEl.addEventListener('input', () => renderShortcuts(searchEl.value, list, listEl));

    document.addEventListener('keydown', (e) => {
      if (e.ctrlKey && (e.key === '/' || e.key === '?')) {
        e.preventDefault();
        if (overlay.classList.contains('visible')) close(); else open();
      } else if (e.key === 'Escape' && overlay.classList.contains('visible')) {
        close();
      }
    });

    // Floating hint button (not on login)
    if (!onLoginPage) {
      const btn = document.createElement('button');
      btn.className = 'shortcuts-hint-btn';
      btn.title = 'Tastaturgenveje (Ctrl+/)';
      btn.setAttribute('aria-label', 'Vis tastaturgenveje');
      btn.innerHTML = '<i class="fas fa-keyboard"></i>';
      btn.addEventListener('click', open);
      document.body.appendChild(btn);
    }
  }

  // ===================== MULTI-TAB SESSION SYNC =====================
  // When the user logs out in one tab, log out all other tabs/windows too.
  function setupMultiTabSync() {
    if (!('BroadcastChannel' in window)) return;
    try {
      const ch = new BroadcastChannel('rd-auth');

      // Broadcast on logout click
      const logoutBtn = document.getElementById('logoutBtn');
      if (logoutBtn) {
        logoutBtn.addEventListener('click', () => {
          try { ch.postMessage({ type: 'logout' }); } catch {}
        }, { capture: true });
      }

      ch.onmessage = (ev) => {
        if (ev.data && ev.data.type === 'logout') {
          if (window.location.pathname.endsWith('login.html')) return;
          window.location.href = 'login.html?status=logout';
        }
      };
    } catch {}
  }

  // ===================== INIT =====================
  function init() {
    setupThemeToggle();
    setupCapsLockWarning();
    setupPasswordRequirements();
    setupLoginLockout();
    setupAutoLogout();
    setupShortcutsModal();
    setupMultiTabSync();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
