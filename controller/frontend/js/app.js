// Remote Desktop Controller — Frontend App Logic
// Communicates with Go backend via window.go.main.App.*

const App = {
  // ==================== INITIALIZATION ====================
  async init() {
    // Set version
    try {
      const version = await window.go.main.App.GetVersion();
      document.getElementById('loginVersion').textContent = version;
      document.getElementById('headerVersion').textContent = version;
    } catch (e) { console.error('GetVersion failed:', e); }

    // Setup event listeners
    this.setupLogin();
    this.setupTabs();
    this.setupHeader();
    this.setupSettings();
    this.setupModals();
    this.setupKeyboardShortcuts();
    this.setupBackendEvents();

    // Try auto-login
    await this.tryAutoLogin();
  },

  // ==================== LOGIN ====================
  setupLogin() {
    document.getElementById('loginForm').addEventListener('submit', (e) => {
      e.preventDefault();
      this.login();
    });
  },

  async tryAutoLogin() {
    try {
      const creds = await window.go.main.App.LoadCredentials();
      if (creds && creds.remember && creds.email && creds.password) {
        document.getElementById('email').value = creds.email;
        document.getElementById('password').value = creds.password;
        document.getElementById('rememberMe').checked = true;
        // Auto-login with slight delay for UI
        setTimeout(() => this.login(), 300);
      }
    } catch (e) {
      console.log('No saved credentials');
    }
  },

  async login() {
    const email = document.getElementById('email').value.trim();
    const password = document.getElementById('password').value;
    const remember = document.getElementById('rememberMe').checked;
    const statusEl = document.getElementById('loginStatus');
    const btn = document.getElementById('loginBtn');

    if (!email || !password) {
      statusEl.textContent = 'Indtast email og adgangskode';
      statusEl.className = 'status-text error';
      return;
    }

    // Show loading
    btn.querySelector('.btn-text').style.display = 'none';
    btn.querySelector('.btn-loading').style.display = '';
    btn.disabled = true;
    statusEl.textContent = '';

    try {
      // Save credentials
      await window.go.main.App.SaveCredentials(email, password, remember);

      // Login
      const result = await window.go.main.App.Login(email, password);

      if (!result.approved) {
        statusEl.textContent = 'Konto afventer godkendelse';
        statusEl.className = 'status-text warning';
        btn.querySelector('.btn-text').style.display = '';
        btn.querySelector('.btn-loading').style.display = 'none';
        btn.disabled = false;
        return;
      }

      // Success — switch to main view
      document.getElementById('userEmail').querySelector('span').textContent = result.email;
      this.showMainView();
      this.loadDevices();
      this.loadPendingDevices();
      this.loadSettings();

    } catch (err) {
      statusEl.textContent = err.message || 'Login mislykkedes';
      statusEl.className = 'status-text error';
    }

    btn.querySelector('.btn-text').style.display = '';
    btn.querySelector('.btn-loading').style.display = 'none';
    btn.disabled = false;
  },

  showMainView() {
    document.getElementById('loginView').style.display = 'none';
    document.getElementById('mainView').style.display = 'flex';
  },

  showLoginView() {
    document.getElementById('mainView').style.display = 'none';
    document.getElementById('loginView').style.display = 'flex';
    document.getElementById('loginStatus').textContent = '';
  },

  // ==================== TABS ====================
  setupTabs() {
    document.querySelectorAll('.tab-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        const tab = btn.dataset.tab;
        // Deactivate all
        document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
        document.querySelectorAll('.tab-pane').forEach(p => p.classList.remove('active'));
        // Activate target
        btn.classList.add('active');
        document.getElementById(tab + 'Tab').classList.add('active');
      });
    });
  },

  switchToTab(tabName) {
    document.querySelectorAll('.tab-btn').forEach(b => {
      b.classList.toggle('active', b.dataset.tab === tabName);
    });
    document.querySelectorAll('.tab-pane').forEach(p => p.classList.remove('active'));
    document.getElementById(tabName + 'Tab').classList.add('active');
  },

  // ==================== HEADER BUTTONS ====================
  setupHeader() {
    document.getElementById('logoutBtn').addEventListener('click', async () => {
      await window.go.main.App.Logout();
      this.showLoginView();
    });

    document.getElementById('supportBtn').addEventListener('click', () => this.showSupportModal());
    document.getElementById('updateBtn').addEventListener('click', () => this.showUpdateModal());
    document.getElementById('settingsBtn').addEventListener('click', () => this.switchToTab('settings'));

    document.getElementById('refreshDevicesBtn').addEventListener('click', () => this.loadDevices());
    document.getElementById('refreshPendingBtn').addEventListener('click', () => this.loadPendingDevices());
  },

  // ==================== DEVICES ====================
  async loadDevices() {
    const container = document.getElementById('deviceList');
    try {
      const devices = await window.go.main.App.GetDevices();
      this.renderDevices(devices, container);
    } catch (err) {
      container.innerHTML = `<div class="empty-state"><i class="fas fa-exclamation-triangle"></i><p>${err.message}</p></div>`;
    }
  },

  renderDevices(devices, container) {
    if (!devices || devices.length === 0) {
      container.innerHTML = '<div class="empty-state"><i class="fas fa-plug"></i><p>Ingen enheder endnu.</p></div>';
      return;
    }

    container.innerHTML = devices.map(d => `
      <div class="device-card" data-id="${d.device_id}">
        <div class="device-card-header">
          <div class="status-dot ${d.status}"></div>
          <span class="device-name">${this.esc(d.device_name)}</span>
        </div>
        <div class="device-meta">
          <span><i class="fas fa-${d.platform === 'darwin' ? 'apple' : 'windows'}"></i> ${this.esc(d.platform)}</span>
          ${d.agent_version ? `<span><i class="fas fa-code-branch"></i> ${this.esc(d.agent_version)}</span>` : ''}
          <span><i class="fas fa-clock"></i> ${this.esc(d.time_since)}</span>
        </div>
        <div class="device-actions">
          <button class="btn btn-sm btn-primary" ${d.is_online ? '' : 'disabled'} onclick="App.connectDevice('${d.device_id}', '${this.esc(d.device_name)}')">
            <i class="fas fa-plug"></i> ${d.is_online ? 'Connect' : 'Offline'}
          </button>
          <button class="btn btn-sm btn-secondary" onclick="App.renameDevice('${d.device_id}', '${this.esc(d.device_name)}')">
            <i class="fas fa-pen"></i>
          </button>
          <button class="btn btn-sm btn-secondary" onclick="App.removeDevice('${d.device_id}', '${this.esc(d.device_name)}')">
            <i class="fas fa-user-minus"></i>
          </button>
          <button class="btn btn-sm btn-danger" onclick="App.deleteDevice('${d.device_id}', '${this.esc(d.device_name)}')">
            <i class="fas fa-trash"></i>
          </button>
        </div>
      </div>
    `).join('');
  },

  async connectDevice(deviceId, deviceName) {
    try {
      console.log('connectDevice called:', deviceId, deviceName);
      // Switch to viewer tab
      this.switchToTab('viewer');
      // Initialize viewer connection
      if (window.Viewer) {
        window.Viewer.connect(deviceId, deviceName);
      } else {
        showToast('FEJL: Viewer ikke loaded', 'error');
      }
    } catch (err) {
      console.error('connectDevice error:', err);
      showToast('Connect fejl: ' + err.message, 'error');
    }
  },

  async renameDevice(deviceId, currentName) {
    const newName = prompt('Nyt navn for enheden:', currentName);
    if (!newName || newName === currentName) return;
    try {
      await window.go.main.App.RenameDevice(deviceId, newName);
      showToast('Enhed omdøbt!', 'success');
      this.loadDevices();
    } catch (err) {
      showToast('Fejl: ' + err.message, 'error');
    }
  },

  async removeDevice(deviceId, name) {
    if (!confirm(`Fjern '${name}' fra din konto?\n\nEnheden slettes ikke, men tildelingen fjernes.`)) return;
    try {
      await window.go.main.App.RemoveDevice(deviceId);
      showToast('Enhed fjernet', 'success');
      this.loadDevices();
    } catch (err) {
      showToast('Fejl: ' + err.message, 'error');
    }
  },

  async deleteDevice(deviceId, name) {
    if (!confirm(`Slet '${name}' permanent?\n\nDette kan ikke fortrydes!`)) return;
    try {
      await window.go.main.App.DeleteDevice(deviceId);
      showToast('Enhed slettet', 'success');
      this.loadDevices();
      this.loadPendingDevices();
    } catch (err) {
      showToast('Fejl: ' + err.message, 'error');
    }
  },

  // ==================== PENDING DEVICES ====================
  async loadPendingDevices() {
    const container = document.getElementById('pendingList');
    try {
      const devices = await window.go.main.App.GetPendingDevices();
      this.renderPendingDevices(devices, container);
    } catch (err) {
      container.innerHTML = `<div class="empty-state"><i class="fas fa-exclamation-triangle"></i><p>${err.message}</p></div>`;
    }
  },

  renderPendingDevices(devices, container) {
    if (!devices || devices.length === 0) {
      container.innerHTML = '<div class="empty-state"><i class="fas fa-check-circle"></i><p>Ingen ventende enheder.</p></div>';
      return;
    }

    container.innerHTML = devices.map(d => `
      <div class="device-card" data-id="${d.device_id}">
        <div class="device-card-header">
          <div class="status-dot ${d.status}"></div>
          <span class="device-name">${this.esc(d.device_name)}</span>
        </div>
        <div class="device-meta">
          <span><i class="fas fa-${d.platform === 'darwin' ? 'apple' : 'windows'}"></i> ${this.esc(d.platform)}</span>
          <span><i class="fas fa-fingerprint"></i> ${d.device_id.substring(0, 12)}...</span>
        </div>
        <div class="device-actions">
          <button class="btn btn-sm btn-success" onclick="App.approveDevice('${d.device_id}', '${this.esc(d.device_name)}')">
            <i class="fas fa-check"></i> Godkend
          </button>
          <button class="btn btn-sm btn-danger" onclick="App.deleteDevice('${d.device_id}', '${this.esc(d.device_name)}')">
            <i class="fas fa-trash"></i> Slet
          </button>
        </div>
      </div>
    `).join('');
  },

  async approveDevice(deviceId, name) {
    if (!confirm(`Godkend '${name}' og tildel til din konto?`)) return;
    try {
      await window.go.main.App.ApproveDevice(deviceId);
      showToast('Enhed godkendt!', 'success');
      this.loadDevices();
      this.loadPendingDevices();
    } catch (err) {
      showToast('Fejl: ' + err.message, 'error');
    }
  },

  // ==================== SETTINGS ====================
  setupSettings() {
    // Range sliders
    document.getElementById('settQuality').addEventListener('input', (e) => {
      document.getElementById('qualityValue').textContent = e.target.value;
    });
    document.getElementById('settBitrate').addEventListener('input', (e) => {
      document.getElementById('bitrateValue').textContent = e.target.value;
    });

    // Save on change (debounced)
    let saveTimer;
    const autoSave = () => {
      clearTimeout(saveTimer);
      saveTimer = setTimeout(() => this.saveSettings(), 500);
    };

    ['settResolution', 'settFPS', 'settCodec', 'settQuality', 'settBitrate',
     'settAdaptive', 'settFileTransfer', 'settClipboard', 'settLowLatency'
    ].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.addEventListener('change', autoSave);
      if (el && el.type === 'range') el.addEventListener('input', autoSave);
    });

    // System buttons
    document.getElementById('installBtn').addEventListener('click', () => this.handleInstall());
    document.getElementById('restartBtn').addEventListener('click', () => this.handleRestart());
    document.getElementById('viewLogBtn').addEventListener('click', () => this.showLogModal());
    document.getElementById('resetSettingsBtn').addEventListener('click', () => this.resetSettings());
  },

  async loadSettings() {
    try {
      const s = await window.go.main.App.GetSettings();
      document.getElementById('settResolution').value = s.max_resolution;
      document.getElementById('settFPS').value = String(s.target_fps);
      document.getElementById('settCodec').value = s.codec;
      document.getElementById('settQuality').value = s.video_quality;
      document.getElementById('qualityValue').textContent = s.video_quality;
      document.getElementById('settBitrate').value = s.max_bitrate;
      document.getElementById('bitrateValue').textContent = s.max_bitrate;
      document.getElementById('settAdaptive').checked = s.adaptive_bitrate;
      document.getElementById('settFileTransfer').checked = s.enable_file_transfer;
      document.getElementById('settClipboard').checked = s.enable_clipboard_sync;
      document.getElementById('settLowLatency').checked = s.low_latency_mode;

      // Update install button
      const installed = await window.go.main.App.IsInstalled();
      document.getElementById('installBtn').innerHTML = installed
        ? '<i class="fas fa-trash"></i> Afinstaller'
        : '<i class="fas fa-download"></i> Installer';

      // Settings info
      document.getElementById('settingsInfo').textContent =
        `${s.max_resolution} @ ${s.target_fps} FPS | ${s.codec} | Kvalitet: ${s.video_quality}% | Bitrate: ${s.max_bitrate} Mbps`;
    } catch (err) {
      console.error('Failed to load settings:', err);
    }
  },

  async saveSettings() {
    try {
      const s = {
        max_resolution: document.getElementById('settResolution').value,
        target_fps: parseInt(document.getElementById('settFPS').value),
        codec: document.getElementById('settCodec').value,
        video_quality: parseInt(document.getElementById('settQuality').value),
        max_bitrate: parseInt(document.getElementById('settBitrate').value),
        adaptive_bitrate: document.getElementById('settAdaptive').checked,
        enable_file_transfer: document.getElementById('settFileTransfer').checked,
        enable_clipboard_sync: document.getElementById('settClipboard').checked,
        low_latency_mode: document.getElementById('settLowLatency').checked,
        // Keep defaults for fields not shown
        high_quality_mode: true,
        theme: 'dark',
        window_width: 1100,
        window_height: 750,
        hardware_acceleration: true,
        enable_audio: false,
      };
      await window.go.main.App.SaveSettings(s);
      document.getElementById('settingsInfo').textContent =
        `${s.max_resolution} @ ${s.target_fps} FPS | ${s.codec} | Kvalitet: ${s.video_quality}% | Bitrate: ${s.max_bitrate} Mbps`;
    } catch (err) {
      console.error('Failed to save settings:', err);
    }
  },

  async applyPreset(preset) {
    try {
      await window.go.main.App.ApplyPreset(preset);
      await this.loadSettings();
      showToast(`${preset.charAt(0).toUpperCase() + preset.slice(1)} preset anvendt`, 'success');
    } catch (err) {
      showToast('Fejl: ' + err.message, 'error');
    }
  },

  async handleInstall() {
    try {
      const installed = await window.go.main.App.IsInstalled();
      if (installed) {
        if (!confirm('Afinstaller controller?\n\nDette fjerner autostart og genveje.')) return;
        const admin = await window.go.main.App.IsAdmin();
        if (!admin) {
          if (confirm('Administrator-rettigheder kræves.\nGenstart som administrator?')) {
            await window.go.main.App.RunAsAdmin();
          }
          return;
        }
        await window.go.main.App.UninstallController();
        showToast('Controller afinstalleret', 'success');
      } else {
        if (!confirm('Installer controller?\n\nKopierer til Program Files og sætter autostart.')) return;
        const admin = await window.go.main.App.IsAdmin();
        if (!admin) {
          if (confirm('Administrator-rettigheder kræves.\nGenstart som administrator?')) {
            await window.go.main.App.RunAsAdmin();
          }
          return;
        }
        await window.go.main.App.InstallController();
        showToast('Controller installeret!', 'success');
      }
      this.loadSettings(); // Refresh install button state
    } catch (err) {
      showToast('Fejl: ' + err.message, 'error');
    }
  },

  async handleRestart() {
    if (!confirm('Genstart applikationen?')) return;
    await window.go.main.App.RestartApp();
  },

  async resetSettings() {
    if (!confirm('Nulstil alle indstillinger til standard?')) return;
    try {
      await window.go.main.App.ApplyPreset('ultra');
      await this.loadSettings();
      showToast('Indstillinger nulstillet', 'success');
    } catch (err) {
      showToast('Fejl: ' + err.message, 'error');
    }
  },

  // ==================== MODALS ====================
  setupModals() {
    // Close modal on backdrop click
    document.querySelectorAll('.modal').forEach(modal => {
      modal.addEventListener('click', (e) => {
        if (e.target === modal) modal.style.display = 'none';
      });
    });

    // ESC to close
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        document.querySelectorAll('.modal').forEach(m => m.style.display = 'none');
      }
    });
  },

  async showSupportModal() {
    const modal = document.getElementById('supportModal');
    const body = document.getElementById('supportModalBody');
    modal.style.display = 'flex';
    body.innerHTML = '<p><i class="fas fa-spinner fa-spin"></i> Opretter support session...</p>';

    try {
      const info = await window.go.main.App.CreateSupportSession();
      body.innerHTML = `
        <p>Del denne PIN eller link med personen der skal hjælpe dig:</p>
        <div class="pin-display">${info.pin}</div>
        <div class="form-group">
          <label>Delelink</label>
          <input type="text" class="share-url" value="${info.share_url}" readonly onclick="this.select()">
        </div>
        <div style="display:flex;gap:0.5rem;">
          <button class="btn btn-sm btn-secondary" onclick="navigator.clipboard.writeText('${info.share_url}');showToast('Link kopieret!','success')">
            <i class="fas fa-copy"></i> Kopier link
          </button>
        </div>
        <p style="margin-top:0.75rem;font-size:0.75rem;color:var(--text-muted)">Udløber: ${info.expires_at}</p>
      `;
    } catch (err) {
      body.innerHTML = `<p style="color:var(--danger)">Fejl: ${err.message}</p>`;
    }
  },

  async showUpdateModal() {
    const modal = document.getElementById('updateModal');
    const body = document.getElementById('updateModalBody');
    modal.style.display = 'flex';
    body.innerHTML = '<p><i class="fas fa-spinner fa-spin"></i> Tjekker for opdateringer...</p>';

    try {
      const info = await window.go.main.App.CheckForUpdate();
      body.innerHTML = `
        <div style="margin-bottom:1rem;">
          <p><strong>Controller (denne app):</strong></p>
          <p style="font-size:0.85rem;color:var(--text-muted)">Installeret: ${info.current_version}</p>
          <p style="font-size:0.85rem;color:var(--text-muted)">Tilgængelig: ${info.controller_version}</p>
          ${info.available ? '<p style="color:var(--success);font-weight:600;">NY VERSION TILGÆNGELIG</p>' : '<p style="color:var(--text-muted);">Opdateret</p>'}
        </div>
        <div style="margin-bottom:1rem;">
          <p><strong>Agent:</strong></p>
          <p style="font-size:0.85rem;color:var(--text-muted)">Tilgængelig: ${info.agent_version}</p>
        </div>
        ${info.available ? `
          <button class="btn btn-primary" id="updateDownloadBtn" onclick="App.downloadUpdate()">
            <i class="fas fa-download"></i> Download og installer
          </button>
          <div class="progress-bar" id="updateProgress" style="display:none;">
            <div class="progress-bar-fill" id="updateProgressFill" style="width:0%"></div>
          </div>
          <p id="updateStatus" style="font-size:0.8rem;color:var(--text-muted);margin-top:0.5rem;"></p>
        ` : ''}
      `;
    } catch (err) {
      body.innerHTML = `<p style="color:var(--danger)">Fejl: ${err.message}</p>`;
    }
  },

  async downloadUpdate() {
    const btn = document.getElementById('updateDownloadBtn');
    const progress = document.getElementById('updateProgress');
    const status = document.getElementById('updateStatus');
    if (btn) btn.disabled = true;
    if (progress) progress.style.display = '';
    if (status) status.textContent = 'Downloader...';

    try {
      await window.go.main.App.DownloadAndInstallUpdate();
    } catch (err) {
      if (status) status.textContent = 'Fejl: ' + err.message;
      if (btn) btn.disabled = false;
    }
  },

  async showLogModal() {
    const modal = document.getElementById('logModal');
    const content = document.getElementById('logContent');
    modal.style.display = 'flex';
    content.textContent = 'Henter log...';

    try {
      const log = await window.go.main.App.GetLogContent(200);
      content.textContent = log;
      content.scrollTop = content.scrollHeight;
    } catch (err) {
      content.textContent = 'Fejl: ' + err.message;
    }
  },

  // ==================== KEYBOARD SHORTCUTS ====================
  setupKeyboardShortcuts() {
    document.addEventListener('keydown', async (e) => {
      // Ctrl+1/2/3: Connect to device #1/#2/#3
      if (e.ctrlKey && e.key >= '1' && e.key <= '9') {
        e.preventDefault();
        const index = parseInt(e.key) - 1;
        try {
          const devices = await window.go.main.App.GetDevices();
          const onlineDevices = devices.filter(d => d.is_online);
          if (index < onlineDevices.length) {
            const d = onlineDevices[index];
            showToast(`Ctrl+${e.key}: Forbinder til ${d.device_name}...`, 'info');
            this.connectDevice(d.device_id, d.device_name);
          } else {
            showToast(`Ctrl+${e.key}: Ingen online enhed #${index + 1}`, 'warning');
          }
        } catch (err) {
          showToast('Fejl: ' + err.message, 'error');
        }
      }
    });
  },

  // ==================== BACKEND EVENTS ====================
  setupBackendEvents() {
    // Listen for device updates from Go backend
    if (window.runtime) {
      window.runtime.EventsOn('devices-updated', (devices) => {
        const container = document.getElementById('deviceList');
        if (container && document.getElementById('devicesTab').classList.contains('active')) {
          this.renderDevices(devices, container);
        }
      });

      window.runtime.EventsOn('update-available', (tagName) => {
        showToast(`Opdatering tilgængelig: ${tagName}`, 'info');
      });

      window.runtime.EventsOn('update-progress', (percent) => {
        const fill = document.getElementById('updateProgressFill');
        if (fill) fill.style.width = percent + '%';
        const status = document.getElementById('updateStatus');
        if (status && !status.dataset.phase) {
          status.textContent = `Downloader... ${Math.round(percent)}%`;
        }
      });

      window.runtime.EventsOn('update-status', (msg) => {
        const status = document.getElementById('updateStatus');
        if (status) {
          status.textContent = msg;
          if (msg.includes('UAC') || msg.includes('Installerer')) {
            status.dataset.phase = 'install';
          } else {
            delete status.dataset.phase;
          }
        }
      });
    }
  },

  // ==================== HELPERS ====================
  esc(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }
};

// ==================== GLOBAL HELPERS ====================
function showToast(message, type = 'info', duration = 4000) {
  const container = document.getElementById('toastContainer');
  const toast = document.createElement('div');
  toast.className = `toast ${type}`;
  toast.textContent = message;
  container.appendChild(toast);
  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateX(100%)';
    setTimeout(() => toast.remove(), 300);
  }, duration);
}

function closeModal(id) {
  document.getElementById(id).style.display = 'none';
}

// ==================== BOOT ====================
document.addEventListener('DOMContentLoaded', () => {
  // Wails runtime ready
  App.init();
});
