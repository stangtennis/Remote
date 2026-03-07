// Activity Log Module
// Shows recent audit events in a collapsible panel

const ActivityLog = {
  _limit: 20,
  _offset: 0,
  _filter: 'all',
  _loading: false,

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

    if (reset) this._offset = 0;

    const list = document.getElementById('activityLogList');
    const loadMoreBtn = document.getElementById('activityLoadMore');
    if (!list) { this._loading = false; return; }

    if (reset) list.innerHTML = '';

    try {
      let query = supabase
        .from('audit_logs')
        .select('*')
        .order('created_at', { ascending: false })
        .range(this._offset, this._offset + this._limit - 1);

      // Apply filter
      if (this._filter === 'sessions') {
        query = query.in('event', ['SESSION_CREATED', 'SESSION_ENDED']);
      } else if (this._filter === 'devices') {
        query = query.in('event', ['DEVICE_ONLINE', 'DEVICE_OFFLINE', 'DEVICE_CLAIMED', 'DEVICE_RENAMED', 'DEVICE_DELETED']);
      } else if (this._filter === 'support') {
        query = query.in('event', ['SUPPORT_SESSION_START', 'SUPPORT_SESSION_END']);
      }

      const { data, error } = await query;
      if (error) throw error;

      if (!data || data.length === 0) {
        if (reset) {
          const empty = document.createElement('div');
          empty.style.cssText = 'text-align: center; padding: 1rem; color: var(--text-muted, #888); font-size: 0.85rem;';
          empty.textContent = 'Ingen aktivitet endnu';
          list.appendChild(empty);
        }
        if (loadMoreBtn) loadMoreBtn.style.display = 'none';
        this._loading = false;
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

    this._loading = false;
  },

  loadMore() {
    this.load(false);
  },

  renderItem(log) {
    const item = document.createElement('div');
    item.style.cssText = 'display: flex; align-items: flex-start; gap: 0.5rem; padding: 0.5rem 0; border-bottom: 1px solid var(--border, #333); font-size: 0.8rem;';

    const icon = document.createElement('span');
    icon.style.cssText = 'font-size: 1rem; flex-shrink: 0; margin-top: 0.1rem;';
    icon.textContent = this._eventIcon(log.event);

    const content = document.createElement('div');
    content.style.cssText = 'flex: 1; min-width: 0;';

    const label = document.createElement('div');
    label.style.color = 'var(--text, #fff)';
    label.textContent = this._eventLabel(log.event, log.details);

    const meta = document.createElement('div');
    meta.style.cssText = 'color: var(--text-muted, #888); font-size: 0.75rem; margin-top: 0.15rem;';
    const parts = [];
    if (log.device_id) {
      const deviceName = log.details?.device_name || log.device_id;
      parts.push(deviceName);
    }
    parts.push(this._relativeTime(log.created_at));
    meta.textContent = parts.join(' · ');

    content.append(label, meta);
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
      'SUPPORT_SESSION_END': '✅'
    };
    return icons[event] || '📋';
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
      'SUPPORT_SESSION_START': 'Support session startet',
      'SUPPORT_SESSION_END': 'Support session afsluttet'
    };
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
