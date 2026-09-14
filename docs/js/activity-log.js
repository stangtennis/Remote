// Activity Log Module
// Shows recent audit events in a collapsible panel

const ActivityLog = {
  _limit: 20,
  _offset: 0,
  _filter: 'all',
  _loading: false,
  _loadGeneration: 0,

  async init() {
    const section = document.getElementById('activityLogSection');
    if (!section) return;

    const toggleBtn = document.getElementById('activityLogToggle');
    if (toggleBtn) {
      toggleBtn.addEventListener('click', () => this.toggle());
    }

    const filterSelect = document.getElementById('activityLogFilter');
    if (filterSelect) {
      filterSelect.addEventListener('change', () => {
        this._filter = filterSelect.value;
        this._offset = 0;
        this._loadGeneration += 1;
        this._loading = false;
        this.load(true);
      });
    }

    const loadMoreBtn = document.getElementById('activityLoadMore');
    if (loadMoreBtn) {
      loadMoreBtn.addEventListener('click', () => this.loadMore());
    }
  },

  toggle() {
    const content = document.getElementById('activityLogContent');
    const toggle = document.getElementById('activityLogToggle');
    if (!content) return;

    const isHidden = content.style.display === 'none';
    content.style.display = isHidden ? 'block' : 'none';
    if (toggle) toggle.textContent = isHidden ? '▼' : '▶';

    if (isHidden && content.children.length <= 1) {
      this.load(true);
    }
  },

  async load(reset) {
    if (this._loading) return;
    this._loading = true;
    const loadGeneration = this._loadGeneration;
    const loadOffset = reset ? 0 : this._offset;

    if (reset) this._offset = 0;

    const list = document.getElementById('activityLogList');
    const loadMoreBtn = document.getElementById('activityLoadMore');
    if (!list) { this._loading = false; return; }

    if (reset) list.innerHTML = '';

    try {
      let data;
      if (this._filter === 'support') {
        data = await this.loadSupportHistory(loadOffset);
      } else {
        let query = supabase
          .from('audit_logs')
          .select('*')
          .order('created_at', { ascending: false })
          .range(loadOffset, loadOffset + this._limit - 1);

        if (this._filter === 'sessions') {
          query = query.in('event', ['SESSION_CREATED', 'SESSION_ENDED']);
        } else if (this._filter === 'devices') {
          query = query.in('event', ['DEVICE_ONLINE', 'DEVICE_OFFLINE', 'DEVICE_CLAIMED', 'DEVICE_RENAMED', 'DEVICE_DELETED']);
        }

        const result = await query;
        if (result.error) throw result.error;
        data = result.data;
      }

      // A filter change can finish a previous request after the new one started.
      if (loadGeneration !== this._loadGeneration) return;

      if (!data || data.length === 0) {
        if (reset) {
          const empty = document.createElement('div');
          empty.style.cssText = 'text-align: center; padding: 1rem; color: var(--text-muted, #888); font-size: 0.85rem;';
          empty.textContent = 'Ingen aktivitet endnu';
          list.appendChild(empty);
        }
        if (loadMoreBtn) loadMoreBtn.style.display = 'none';
        if (loadGeneration === this._loadGeneration) this._loading = false;
        return;
      }

      for (const log of data) {
        list.appendChild(this.renderItem(log));
      }

      this._offset += data.length;
      if (loadMoreBtn) {
        loadMoreBtn.style.display = data.length >= this._limit ? 'block' : 'none';
      }
    } catch (e) {
      console.error('Activity log load failed:', e);
    }

    if (loadGeneration === this._loadGeneration) this._loading = false;
  },

  loadMore() {
    this.load(false);
  },

  async loadSupportHistory(offset) {
    const { data, error } = await supabase
      .from('support_action_audit')
      .select('id, support_session_id, device_id, actor_type, action_type, target, status, summary, details, verified, started_at, completed_at, created_at, support_sessions!inner(client_label, support_mode)')
      .eq('support_sessions.support_mode', 'ai')
      .order('created_at', { ascending: false })
      .range(offset, offset + this._limit - 1);

    if (error) throw error;
    if (!data || data.length === 0) return data;
    return data.map((item) => ({
      ...item,
      _supportSession: item.support_sessions,
    }));
  },

  renderItem(log) {
    const item = document.createElement('div');
    item.style.cssText = 'display: flex; align-items: flex-start; gap: 0.5rem; padding: 0.5rem 0; border-bottom: 1px solid var(--border, #333); font-size: 0.8rem;';

    const icon = document.createElement('span');
    icon.style.cssText = 'font-size: 1rem; flex-shrink: 0; margin-top: 0.1rem;';
    icon.textContent = this._eventIcon(log.action_type || log.event);

    const content = document.createElement('div');
    content.style.cssText = 'flex: 1; min-width: 0;';

    const label = document.createElement('div');
    label.style.color = 'var(--text, #fff)';
    label.textContent = log.action_type
      ? this._supportActionLabel(log)
      : this._eventLabel(log.event, log.details);

    const meta = document.createElement('div');
    meta.style.cssText = 'color: var(--text-muted, #888); font-size: 0.75rem; margin-top: 0.15rem;';
    const parts = [];
    if (log.device_id) {
      const deviceName = log.details?.device_name || log.device_id;
      parts.push(deviceName);
    }
    if (log.action_type && log._supportSession?.client_label) {
      parts.push(log._supportSession.client_label);
    }
    if (log.action_type) {
      if (log.actor_type) parts.push(this._actorLabel(log.actor_type));
      parts.push(this._statusLabel(log.status));
      if (log.target) parts.push(log.target);
    }
    parts.push(this._relativeTime(log.created_at));
    meta.textContent = parts.join(' · ');

    content.append(label, meta);
    if (log.action_type) {
      const detail = this._supportActionDetail(log);
      if (detail) {
        const detailNode = document.createElement('div');
        detailNode.style.cssText = 'color: var(--text-muted, #888); font-size: 0.75rem; margin-top: 0.2rem; overflow-wrap: anywhere;';
        detailNode.textContent = detail;
        content.appendChild(detailNode);
      }
    }
    item.append(icon, content);
    return item;
  },

  _eventIcon(event) {
    const icons = {
      'SESSION_CREATED': '🔗',
      'SESSION_ENDED': '🔌',
      'DEVICE_CLAIMED': '🔗',
      'DEVICE_ONLINE': '🟢',
      'DEVICE_OFFLINE': '🔴',
      'DEVICE_RENAMED': '✏️',
      'DEVICE_DELETED': '🗑️',
      'SUPPORT_SESSION_START': '🆘',
      'SUPPORT_SESSION_END': '✅',
      'SUPPORT_SESSION_CREATED': '🆘',
      'CLIENT_CODE_VERIFIED': '🔐',
      'CLIENT_CONSENT_GRANTED': '✅',
      'SUPPORT_SESSION_READY': '🟢',
      'TURN_CREDENTIALS_ISSUED': '🔑',
      'SUPPORT_SESSION_ENDED': '🔌',
      'SUPPORT_SESSION_REVOKED': '🚫',
      'SCREEN_SCREENSHOT': '📸',
      'INPUT_CLICK': '🖱️',
      'INPUT_TYPE': '⌨️',
      'INPUT_KEY': '⌨️',
      'INPUT_SCROLL': '↕️',
      'INPUT_MOUSE_CLICK': '🖱️',
      'INPUT_MOUSE_SCROLL': '↕️',
      'SHELL_EXEC': '⚙️',
      'FILE_UPLOAD': '⬆️',
      'FILE_DOWNLOAD': '⬇️',
      'FILE_OPERATION': '📁',
      'TERMINAL_INPUT': '⌨️',
      'TERMINAL_START': '▶️',
      'TERMINAL_CLOSE': '⏹️',
      'PROCESS_PS': '📊',
      'PROCESS_KILL': '⏹️',
      'PROCESS_SYSINFO': '🖥️',
      'ADMIN_REMOTE_LOGIN': '🔑',
      'ADMIN_FORCE_UPDATE': '⬆️'
    };
    return icons[event] || '📋';
  },

  _supportActionLabel(log) {
    const labels = {
      'SCREEN_SCREENSHOT': 'AI tog skærmbillede',
      'INPUT_CLICK': 'AI klikkede på fjernskærmen',
      'INPUT_TYPE': 'AI skrev på fjernskærmen',
      'INPUT_KEY': 'AI trykkede på en tast',
      'INPUT_SCROLL': 'AI rullede på fjernskærmen',
      'INPUT_MOUSE_CLICK': 'AI klikkede med musen',
      'INPUT_MOUSE_SCROLL': 'AI rullede med musen',
      'SHELL_EXEC': 'AI kørte shell-kommando',
      'FILE_UPLOAD': 'AI uploadede fil',
      'FILE_DOWNLOAD': 'AI downloadede fil',
      'FILE_OPERATION': 'AI udførte filhandling',
      'TERMINAL_INPUT': 'AI sendte terminalinput',
      'TERMINAL_START': 'AI åbnede terminal',
      'TERMINAL_CLOSE': 'AI lukkede terminal',
      'PROCESS_PS': 'AI hentede procesliste',
      'PROCESS_KILL': 'AI stoppede proces',
      'PROCESS_SYSINFO': 'AI hentede systemoplysninger',
      'ADMIN_REMOTE_LOGIN': 'AI loggede fjernbruger ind',
      'ADMIN_FORCE_UPDATE': 'AI tvang opdatering',
      'SUPPORT_SESSION_CREATED': 'AI-supportsession oprettet',
      'CLIENT_CODE_VERIFIED': 'Klientkode verificeret',
      'CLIENT_CONSENT_GRANTED': 'Klientsamtykke givet',
      'SUPPORT_SESSION_READY': 'AI-supportsession klar',
      'TURN_CREDENTIALS_ISSUED': 'Forbindelseslegitimationsoplysninger udstedt',
      'SUPPORT_SESSION_ENDED': 'AI-supportsession afsluttet',
      'SUPPORT_SESSION_REVOKED': 'AI-supportsession tilbagekaldt',
    };
    return labels[log.action_type] || log.summary || log.action_type;
  },

  _statusLabel(status) {
    const labels = {
      started: 'startet',
      succeeded: 'lykkedes',
      failed: 'fejlede',
      cancelled: 'annulleret',
    };
    return labels[status] || status || 'ukendt status';
  },

  _actorLabel(actorType) {
    const labels = {
      ai: 'AI',
      admin: 'admin',
      operator: 'operatør',
      client: 'klient',
      system: 'system',
    };
    return labels[actorType] || actorType;
  },

  _supportActionDetail(log) {
    const details = log.details || {};
    if (typeof details.error === 'string') return `Fejl: ${details.error.slice(0, 360)}`;
    if (typeof details.result === 'string' || typeof details.result === 'number' || typeof details.result === 'boolean') {
      return `Resultat: ${String(details.result).slice(0, 360)}`;
    }
    if (details.exit_code !== undefined) return `Exit code: ${details.exit_code}`;
    if (details.operation) return `Handling: ${details.operation}`;
    if (details.command_length !== undefined) return `Kommando registreret sikkert (${details.command_length} tegn)`;
    return '';
  },

  _eventLabel(event, details) {
    const labels = {
      'SESSION_CREATED': 'Session startet',
      'SESSION_ENDED': 'Session afsluttet',
      'DEVICE_CLAIMED': 'Enhed tilknyttet',
      'DEVICE_ONLINE': 'Enhed online',
      'DEVICE_OFFLINE': 'Enhed offline',
      'DEVICE_RENAMED': details?.old_name
        ? `Omdøbt: ${details.old_name} → ${details.new_name}`
        : 'Enhed omdøbt',
      'DEVICE_DELETED': 'Enhed slettet',
      'AI_SUPPORT_COMMAND': 'AI kørte kommando på supportklient',
      'SUPPORT_SESSION_START': 'Support session startet',
      'SUPPORT_SESSION_END': 'Support session afsluttet'
    };
    if (event === 'AI_SUPPORT_COMMAND' && typeof details?.command === 'string') {
      return `AI kørte: ${details.command.slice(0, 220)}`;
    }
    return labels[event] || event;
  },

  _relativeTime(timestamp) {
    const now = Date.now();
    const then = new Date(timestamp).getTime();
    const diff = Math.floor((now - then) / 1000);

    if (diff < 60) return 'lige nu';
    if (diff < 3600) return `${Math.floor(diff / 60)} min siden`;
    if (diff < 86400) return `${Math.floor(diff / 3600)} timer siden`;
    if (diff < 604800) return `${Math.floor(diff / 86400)} dage siden`;
    return new Date(timestamp).toLocaleDateString('da-DK');
  }
};

window.ActivityLog = ActivityLog;
