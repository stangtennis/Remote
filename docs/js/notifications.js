// Browser Notifications Module
// Sends desktop notifications when devices come online/offline (only when tab is not focused)

const BrowserNotifications = {
  _enabled: false,
  _permission: Notification.permission,

  init() {
    this._enabled = localStorage.getItem('notificationsEnabled') === 'true';
    this._permission = Notification.permission;
    this._updateButton();
  },

  async toggle() {
    if (!('Notification' in window)) {
      showToast('Browseren understøtter ikke notifikationer', 'error');
      return;
    }

    if (this._enabled) {
      this._enabled = false;
      localStorage.setItem('notificationsEnabled', 'false');
      showToast('Notifikationer deaktiveret', 'info');
    } else {
      if (Notification.permission === 'default') {
        const perm = await Notification.requestPermission();
        this._permission = perm;
        if (perm !== 'granted') {
          showToast('Notifikationer blev afvist af browseren', 'error');
          return;
        }
      } else if (Notification.permission === 'denied') {
        showToast('Notifikationer er blokeret. Tillad dem i browserindstillinger.', 'error');
        return;
      }
      this._enabled = true;
      localStorage.setItem('notificationsEnabled', 'true');
      showToast('Notifikationer aktiveret', 'success');
    }
    this._updateButton();
  },

  notify(title, body, tag) {
    if (!this._enabled || Notification.permission !== 'granted') return;
    if (document.hasFocus()) return;

    try {
      new Notification(title, {
        body,
        icon: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">🖥️</text></svg>',
        tag: tag || undefined,
        silent: false
      });
    } catch (e) {
      console.warn('Notification failed:', e);
    }
  },

  _updateButton() {
    const btn = document.getElementById('notificationToggleBtn');
    if (!btn) return;
    if (this._enabled) {
      btn.title = 'Notifikationer aktiveret (klik for at slå fra)';
      btn.classList.add('active');
    } else {
      btn.title = 'Notifikationer deaktiveret (klik for at slå til)';
      btn.classList.remove('active');
    }
  }
};

window.BrowserNotifications = BrowserNotifications;
