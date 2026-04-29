// Device Management Module
// Handles device list, approval, selection, search/filter, tags, and favorites

// Cached data for client-side filtering
let _allDevices = [];
let _deviceTags = {};     // { device_id: ['tag1', 'tag2'] }
let _userFavorites = {};  // { device_id: true }
let _currentUserId = null;

async function initDevices() {
  if (window.BrowserNotifications) BrowserNotifications.init();

  // Setup search/filter listeners
  const searchInput = document.getElementById('deviceSearchInput');
  const statusFilter = document.getElementById('deviceStatusFilter');
  if (searchInput) searchInput.addEventListener('input', applyDeviceFilters);
  if (statusFilter) statusFilter.addEventListener('change', applyDeviceFilters);

  await loadDevices();
  subscribeToDeviceUpdates();
}

async function loadDevices() {
  const devicesList = document.getElementById('devicesList');
  const emptyState = document.getElementById('emptyState');
  const loadingDevices = document.getElementById('loadingDevices');

  // Show loading
  loadingDevices.style.display = 'block';
  devicesList.innerHTML = '';
  emptyState.style.display = 'none';

  try {
    const { data: { session } } = await supabase.auth.getSession();
    if (!session) return;
    _currentUserId = session.user.id;

    // Check if user is admin/super_admin
    const { data: approval } = await supabase
      .from('user_approvals')
      .select('role')
      .eq('user_id', session.user.id)
      .single();

    const isAdmin = approval && (approval.role === 'admin' || approval.role === 'super_admin');
    const isSuperAdmin = approval && approval.role === 'super_admin';
    // Expose role for downstream UI helpers (showDeviceMenu, assignDeviceUI)
    window.__rdRole = approval ? approval.role : null;
    window.__rdIsAdmin = !!isAdmin;
    window.__rdIsSuperAdmin = !!isSuperAdmin;

    // Load devices, tags, and favorites in parallel
    // super_admin: sees ALL devices
    // admin/user: sees owned + assigned devices (via get_user_devices RPC)
    const devicesPromise = isSuperAdmin
      ? supabase.from('remote_devices').select('*').order('last_seen', { ascending: false })
      : supabase.rpc('get_user_devices', { p_user_id: session.user.id });

    const tagsPromise = supabase.from('device_tags').select('device_id, tag');
    const favoritesPromise = supabase.from('user_device_favorites').select('device_id').eq('user_id', session.user.id);

    const [devicesSettled, tagsSettled, favoritesSettled] = await Promise.allSettled([devicesPromise, tagsPromise, favoritesPromise]);

    const devicesResult = devicesSettled.status === 'fulfilled' ? devicesSettled.value : { error: devicesSettled.reason };
    const tagsResult = tagsSettled.status === 'fulfilled' ? tagsSettled.value : { data: null };
    const favoritesResult = favoritesSettled.status === 'fulfilled' ? favoritesSettled.value : { data: null };

    if (tagsSettled.status === 'rejected') console.warn('Tags query failed:', tagsSettled.reason);
    if (favoritesSettled.status === 'rejected') console.warn('Favorites query failed:', favoritesSettled.reason);

    if (devicesResult.error) throw devicesResult.error;

    // Process tags
    _deviceTags = {};
    if (tagsResult.data) {
      for (const row of tagsResult.data) {
        if (!_deviceTags[row.device_id]) _deviceTags[row.device_id] = [];
        _deviceTags[row.device_id].push(row.tag);
      }
    }

    // Process favorites
    _userFavorites = {};
    if (favoritesResult.data) {
      for (const row of favoritesResult.data) {
        _userFavorites[row.device_id] = true;
      }
    }

    loadingDevices.style.display = 'none';

    const devices = devicesResult.data;
    if (!devices || devices.length === 0) {
      emptyState.style.display = 'block';
      _allDevices = [];
      return;
    }

    // Deduplicate by device_id
    const uniqueMap = {};
    for (const d of devices) uniqueMap[d.device_id] = d;
    _allDevices = Object.values(uniqueMap);

    applyDeviceFilters();

  } catch (error) {
    console.error('Failed to load devices:', error);
    loadingDevices.style.display = 'none';
    showToast('Kunne ikke indlæse enheder: ' + error.message, 'error');
  }
}

function applyDeviceFilters() {
  const devicesList = document.getElementById('devicesList');
  const emptyState = document.getElementById('emptyState');
  if (!devicesList) return;

  const search = (document.getElementById('deviceSearchInput')?.value || '').toLowerCase().trim();
  const statusFilter = document.getElementById('deviceStatusFilter')?.value || 'all';

  let filtered = _allDevices.filter(device => {
    // Status filter
    if (statusFilter === 'online' && !device.is_online) return false;
    if (statusFilter === 'offline' && device.is_online) return false;

    // Search filter (matches name, id, platform, or tags)
    if (search) {
      const name = (device.device_name || '').toLowerCase();
      const id = (device.device_id || '').toLowerCase();
      const platform = (device.platform || '').toLowerCase();
      const tags = (_deviceTags[device.device_id] || []).join(' ').toLowerCase();
      if (!name.includes(search) && !id.includes(search) && !platform.includes(search) && !tags.includes(search)) {
        return false;
      }
    }
    return true;
  });

  // Sort: Favorites first → Online → Offline → last_seen DESC
  filtered.sort((a, b) => {
    const aFav = _userFavorites[a.device_id] ? 1 : 0;
    const bFav = _userFavorites[b.device_id] ? 1 : 0;
    if (aFav !== bFav) return bFav - aFav;
    if (a.is_online !== b.is_online) return a.is_online ? -1 : 1;
    const aTime = a.last_seen ? new Date(a.last_seen).getTime() : 0;
    const bTime = b.last_seen ? new Date(b.last_seen).getTime() : 0;
    return bTime - aTime;
  });

  devicesList.innerHTML = '';
  if (filtered.length === 0) {
    if (_allDevices.length === 0) {
      emptyState.style.display = 'block';
    } else {
      const noMatch = document.createElement('div');
      noMatch.style.cssText = 'text-align: center; padding: 2rem; color: var(--text-muted, #888);';
      noMatch.textContent = 'Ingen enheder matcher filteret';
      devicesList.appendChild(noMatch);
      emptyState.style.display = 'none';
    }
    return;
  }

  emptyState.style.display = 'none';

  // Group by online status and add section headers
  const onlineDevices = filtered.filter(d => d.is_online);
  const offlineDevices = filtered.filter(d => !d.is_online);

  if (onlineDevices.length > 0) {
    const header = document.createElement('div');
    header.style.cssText = 'padding: 0.4rem 0.75rem; font-size: 0.7rem; font-weight: 600; color: #22c55e; text-transform: uppercase; letter-spacing: 0.05em; display: flex; align-items: center; gap: 0.4rem;';
    header.innerHTML = `<span class="online-pulse-dot" aria-hidden="true"></span> Online (${onlineDevices.length})`;
    devicesList.appendChild(header);
    for (const device of onlineDevices) {
      devicesList.appendChild(createDeviceCard(device));
    }
  }

  if (offlineDevices.length > 0) {
    const header = document.createElement('div');
    header.style.cssText = 'padding: 0.4rem 0.75rem; font-size: 0.7rem; font-weight: 600; color: #666; text-transform: uppercase; letter-spacing: 0.05em; display: flex; align-items: center; gap: 0.4rem;' + (onlineDevices.length > 0 ? ' margin-top: 0.5rem; border-top: 1px solid var(--border, #333); padding-top: 0.6rem;' : '');
    header.innerHTML = `<span style="width:6px; height:6px; border-radius:50%; background:#666;"></span> Offline (${offlineDevices.length})`;
    devicesList.appendChild(header);
    for (const device of offlineDevices) {
      devicesList.appendChild(createDeviceCard(device));
    }
  }
}

function createDeviceCard(device) {
  const card = document.createElement('div');
  card.className = `device-card ${device.is_online ? '' : 'offline'}`;
  card.dataset.deviceId = device.device_id;
  // Compact single-row layout
  card.style.cssText = 'display: flex; align-items: center; gap: 0.5rem; padding: 0.5rem 0.75rem; cursor: pointer;';

  const isFav = !!_userFavorites[device.device_id];

  // Favorite star
  const starBtn = document.createElement('button');
  starBtn.className = 'btn btn-icon';
  starBtn.style.cssText = 'font-size: 1rem; padding: 0; min-width: auto; flex-shrink: 0;';
  starBtn.textContent = isFav ? '★' : '☆';
  starBtn.title = isFav ? 'Fjern fra favoritter' : 'Tilføj til favoritter';
  if (isFav) starBtn.style.color = '#f59e0b';
  starBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    toggleFavorite(device.device_id);
  });

  // Status dot
  const dot = document.createElement('span');
  dot.style.cssText = `width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; background: ${device.is_online ? '#22c55e' : '#666'};`;
  if (device.is_online) dot.style.boxShadow = '0 0 6px rgba(34,197,94,0.5)';

  // Name + subtitle
  const nameCol = document.createElement('div');
  nameCol.style.cssText = 'flex: 1; min-width: 0; overflow: hidden;';
  const nameEl = document.createElement('div');
  nameEl.style.cssText = 'font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-size: 0.9rem;';
  nameEl.textContent = device.device_name || device.device_id;
  const subtitle = document.createElement('div');
  subtitle.style.cssText = 'font-size: 0.7rem; color: var(--text-muted, #888); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;';
  const parts = [device.platform || 'Unknown'];
  if (device.agent_version) parts.push(device.agent_version);
  if (device.public_ip) parts.push(device.public_ip);
  if (device.isp) parts.push(device.isp);
  if (device.connection_type) {
    const ctLabel = { host: 'P2P', srflx: 'STUN', relay: 'Relay' }[device.connection_type] || device.connection_type;
    parts.push(ctLabel);
  }
  subtitle.textContent = parts.join(' · ');
  nameCol.append(nameEl, subtitle);

  // Tag badges (inline, compact)
  const tags = _deviceTags[device.device_id] || [];
  const tagsContainer = document.createElement('div');
  tagsContainer.style.cssText = 'display: flex; gap: 0.2rem; flex-shrink: 0; align-items: center;';
  for (const tag of tags.slice(0, 3)) {
    const tagBadge = document.createElement('span');
    tagBadge.style.cssText = 'background: rgba(99,102,241,0.2); color: var(--primary, #6366f1); padding: 0.05rem 0.35rem; border-radius: 9999px; font-size: 0.65rem; cursor: pointer;';
    tagBadge.textContent = tag;
    tagBadge.title = 'Klik for at fjerne';
    tagBadge.addEventListener('click', (e) => {
      e.stopPropagation();
      removeTag(device.device_id, tag);
    });
    tagsContainer.appendChild(tagBadge);
  }
  if (tags.length > 3) {
    const more = document.createElement('span');
    more.style.cssText = 'font-size: 0.65rem; color: var(--text-muted, #888);';
    more.textContent = `+${tags.length - 3}`;
    tagsContainer.appendChild(more);
  }

  // Action buttons (compact)
  const actions = document.createElement('div');
  actions.style.cssText = 'display: flex; gap: 0.3rem; flex-shrink: 0; align-items: center;';

  if (device.is_online) {
    const connectBtn = document.createElement('button');
    connectBtn.className = 'btn btn-primary btn-sm';
    connectBtn.style.cssText = 'padding: 0.2rem 0.6rem; font-size: 0.75rem;';
    connectBtn.textContent = 'Connect';
    connectBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      startSession(device);
    });
    actions.appendChild(connectBtn);
  }

  if (!device.owner_id) {
    const claimBtn = document.createElement('button');
    claimBtn.className = 'btn btn-secondary btn-sm';
    claimBtn.style.cssText = 'padding: 0.2rem 0.5rem; font-size: 0.75rem;';
    claimBtn.textContent = '🔗 Claim';
    claimBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      await claimDevice(device);
    });
    actions.appendChild(claimBtn);
  }

  // Overflow menu (rename, delete, tag)
  const menuBtn = document.createElement('button');
  menuBtn.className = 'btn btn-icon';
  menuBtn.style.cssText = 'font-size: 1rem; padding: 0 0.2rem; min-width: auto; position: relative;';
  menuBtn.textContent = '⋮';
  menuBtn.title = 'Flere muligheder';
  menuBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    showDeviceMenu(menuBtn, device);
  });
  actions.appendChild(menuBtn);

  card.append(starBtn, dot, nameCol, tagsContainer, actions);
  return card;
}

function showDeviceMenu(anchor, device) {
  // Remove any existing menu
  document.querySelectorAll('.device-context-menu').forEach(m => m.remove());

  const menu = document.createElement('div');
  menu.className = 'device-context-menu';
  menu.style.cssText = 'position: absolute; z-index: 100; background: var(--surface, #1e1e2e); border: 1px solid var(--border, #333); border-radius: 6px; padding: 0.25rem 0; min-width: 120px; box-shadow: 0 4px 12px rgba(0,0,0,0.3);';

  const items = [
    { label: '🏷️ Tag', action: () => addTagPrompt(device.device_id) },
    { label: '✏️ Omdøb', action: () => renameDevice(device) },
    { label: '🔄 Opdater agent', action: () => forceUpdateDevice(device), show: device.is_online },
    // Admin-only — assign / transfer device ownership
    { label: '👥 Tildel adgang', action: () => assignDevicePrompt(device), show: !!window.__rdIsAdmin },
    { label: '🗑️ Slet', action: () => deleteDevice(device), danger: true }
  ].filter(i => i.show !== false);

  for (const item of items) {
    const btn = document.createElement('button');
    btn.style.cssText = `display: block; width: 100%; text-align: left; padding: 0.35rem 0.75rem; background: none; border: none; color: ${item.danger ? '#ef4444' : 'var(--text, #fff)'}; cursor: pointer; font-size: 0.8rem;`;
    btn.textContent = item.label;
    btn.addEventListener('mouseenter', () => btn.style.background = 'rgba(255,255,255,0.05)');
    btn.addEventListener('mouseleave', () => btn.style.background = 'none');
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      menu.remove();
      item.action();
    });
    menu.appendChild(btn);
  }

  // Position relative to anchor
  const rect = anchor.getBoundingClientRect();
  menu.style.position = 'fixed';
  menu.style.top = rect.bottom + 4 + 'px';
  menu.style.right = (window.innerWidth - rect.right) + 'px';

  document.body.appendChild(menu);

  // Close on click outside
  const closeMenu = (e) => {
    if (!menu.contains(e.target)) {
      menu.remove();
      document.removeEventListener('click', closeMenu);
    }
  };
  setTimeout(() => document.addEventListener('click', closeMenu), 0);
}

// ==================== FAVORITES ====================

async function toggleFavorite(deviceId) {
  if (!_currentUserId) return;
  try {
    if (_userFavorites[deviceId]) {
      await supabase.from('user_device_favorites').delete().eq('user_id', _currentUserId).eq('device_id', deviceId);
      delete _userFavorites[deviceId];
    } else {
      await supabase.from('user_device_favorites').insert({ user_id: _currentUserId, device_id: deviceId });
      _userFavorites[deviceId] = true;
    }
    applyDeviceFilters();
  } catch (e) {
    console.warn('Favorite toggle failed:', e);
  }
}

// ==================== TAGS ====================

async function addTagPrompt(deviceId) {
  const tag = await showPrompt('Skriv et tag for at organisere denne enhed', {
    title: 'Tilføj tag',
    icon: '🏷️',
    placeholder: 'fx prod, kontor, server',
    confirmText: 'Tilføj',
    validator: (v) => {
      const t = (v || '').trim();
      if (!t) return 'Skriv et tag-navn';
      if (t.length > 30) return 'Maks 30 tegn';
      if (!/^[a-zA-Z0-9æøåÆØÅ_-]+$/.test(t)) return 'Kun bogstaver, tal, _ og -';
      return null;
    },
  });
  if (!tag || !tag.trim()) return;
  const cleanTag = tag.trim().toLowerCase().substring(0, 30);

  try {
    const { error } = await supabase.from('device_tags').insert({
      device_id: deviceId,
      tag: cleanTag,
      created_by: _currentUserId
    });
    if (error) {
      if (error.code === '23505') { // unique violation
        showToast('Tag findes allerede', 'info');
        return;
      }
      throw error;
    }
    if (!_deviceTags[deviceId]) _deviceTags[deviceId] = [];
    _deviceTags[deviceId].push(cleanTag);
    applyDeviceFilters();
    showToast(`Tag "${cleanTag}" tilføjet`, 'success');
  } catch (e) {
    console.error('Add tag failed:', e);
    showToast('Kunne ikke tilføje tag', 'error');
  }
}

async function removeTag(deviceId, tag) {
  try {
    await supabase.from('device_tags').delete().eq('device_id', deviceId).eq('tag', tag);
    if (_deviceTags[deviceId]) {
      _deviceTags[deviceId] = _deviceTags[deviceId].filter(t => t !== tag);
    }
    applyDeviceFilters();
    showToast(`Tag "${tag}" fjernet`, 'success');
  } catch (e) {
    console.error('Remove tag failed:', e);
  }
}

// ==================== FORCE UPDATE ====================

async function forceUpdateDevice(device) {
  // Try WebRTC data channel first (gives real-time status feedback)
  const ctx = window.SessionManager?.sessions.get(device.device_id);
  if (ctx && ctx.dataChannel && ctx.dataChannel.readyState === 'open') {
    ctx.dataChannel.send(JSON.stringify({ type: 'force_update' }));
    showToast('Opdatering startet på ' + (device.device_name || device.device_id), 'info');
    return;
  }

  // Fallback: set pending_command via Supabase (agent picks it up at next heartbeat)
  try {
    const { error } = await supabase
      .from('remote_devices')
      .update({ pending_command: 'force_update' })
      .eq('device_id', device.device_id);

    if (error) throw error;

    showToast('Opdatering sat i kø for ' + (device.device_name || device.device_id) + ' (agent tjekker inden 30 sek)', 'info');
  } catch (e) {
    console.error('Force update failed:', e);
    showToast('Kunne ikke sende opdateringskommando: ' + e.message, 'error');
  }
}

// ==================== DEVICE OPERATIONS ====================

async function claimDevice(device) {
  if (!await showConfirm(`Tilknyt enhed "${device.device_name || device.device_id}"?\n\nDette vil tildele enheden til din konto.`, { title: 'Tilknyt enhed', confirmText: 'Tilknyt', type: 'info', icon: '🔗' })) {
    return;
  }

  try {
    const { data: { session } } = await supabase.auth.getSession();
    if (!session) return;

    const { error } = await supabase
      .from('remote_devices')
      .update({
        owner_id: session.user.id,
        approved_by: session.user.id,
        approved_at: new Date().toISOString()
      })
      .eq('device_id', device.device_id);

    if (error) throw error;

    try {
      await supabase.rpc('log_audit_event', {
        p_session_id: null,
        p_device_id: device.device_id,
        p_event: 'DEVICE_CLAIMED',
        p_details: { device_name: device.device_name, claimed_by: session.user.email },
        p_severity: 'info'
      });
    } catch (e) {
      console.warn('Audit log failed:', e);
    }

    await loadDevices();
    showToast('Enhed tilknyttet! Du kan nu oprette forbindelse.', 'success');
  } catch (error) {
    console.error('Failed to claim device:', error);
    showToast('Kunne ikke tilknytte enhed: ' + error.message, 'error');
  }
}

async function renameDevice(device) {
  const currentName = device.device_name || device.device_id;
  const newName = await showPrompt(`Vælg et nyt navn for "${currentName}"`, {
    title: 'Omdøb enhed',
    icon: '✏️',
    defaultValue: currentName,
    placeholder: 'Enhedens nye navn',
    confirmText: 'Gem',
    validator: (v) => {
      const t = (v || '').trim();
      if (!t) return 'Navnet kan ikke være tomt';
      if (t.length > 64) return 'Maks 64 tegn';
      return null;
    },
  });
  if (!newName || newName === currentName) return;

  try {
    const { error } = await supabase
      .from('remote_devices')
      .update({ device_name: newName })
      .eq('device_id', device.device_id);

    if (error) throw error;

    try {
      await supabase.rpc('log_audit_event', {
        p_session_id: null,
        p_device_id: device.device_id,
        p_event: 'DEVICE_RENAMED',
        p_details: { old_name: currentName, new_name: newName },
        p_severity: 'info'
      });
    } catch (e) {
      console.warn('Audit log failed:', e);
    }

    showToast(`Enhed omdøbt til "${newName}"`, 'success');
    await loadDevices();
  } catch (error) {
    console.error('Failed to rename device:', error);
    showToast('Kunne ikke omdøbe enhed: ' + error.message, 'error');
  }
}

async function deleteDevice(device) {
  if (!await showConfirm(`Slet enhed "${device.device_name || device.device_id}"?\n\nDette kan ikke fortrydes.`, { title: 'Slet enhed', confirmText: 'Slet', type: 'danger', icon: '🗑️' })) {
    return;
  }

  try {
    const { error } = await supabase
      .from('remote_devices')
      .delete()
      .eq('device_id', device.device_id);

    if (error) throw error;

    try {
      await supabase.rpc('log_audit_event', {
        p_session_id: null,
        p_device_id: device.device_id,
        p_event: 'DEVICE_DELETED',
        p_details: { device_name: device.device_name },
        p_severity: 'info'
      });
    } catch (e) {
      console.warn('Audit log failed:', e);
    }

    debug('Device deleted:', device.device_id);
    await loadDevices();
  } catch (error) {
    console.error('Failed to delete device:', error);
    showToast('Kunne ikke slette enhed: ' + error.message, 'error');
  }
}

// Debounce helper to prevent too frequent reloads
let reloadTimeout;
function debouncedReload() {
  clearTimeout(reloadTimeout);
  reloadTimeout = setTimeout(() => {
    loadDevices();
  }, 500);
}

function subscribeToDeviceUpdates() {
  const channel = supabase
    .channel('devices-changes')
    .on('postgres_changes', {
      event: '*',
      schema: 'public',
      table: 'remote_devices'
    }, (payload) => {
      debug('Device update:', payload);

      // Detect online/offline changes for notifications
      if (payload.eventType === 'UPDATE' && payload.new && payload.old) {
        const wasOnline = payload.old.is_online;
        const isOnline = payload.new.is_online;
        if (wasOnline !== isOnline && window.BrowserNotifications) {
          const name = payload.new.device_name || payload.new.device_id;
          if (isOnline) {
            BrowserNotifications.notify('Enhed online', `${name} er nu online`, `device-${payload.new.device_id}`);
          } else {
            BrowserNotifications.notify('Enhed offline', `${name} er gået offline`, `device-${payload.new.device_id}`);
          }
        }
      }

      debouncedReload();
    })
    .subscribe();

  debug('📡 Subscribed to device updates');
}

// ─── Device assignment (admin/super_admin only) ──────────────────────
//
// Two ways to give another user access to a device:
//   1. Reassign owner_id   → recipient becomes the owner (current owner
//      loses access unless they're also admin)
//   2. Delegate access      → row in device_assignments table; recipient
//      sees the device alongside their own without changing ownership
async function assignDevicePrompt(device) {
  if (!window.__rdIsAdmin) {
    showToast('Kun admin/super_admin kan tildele', 'error');
    return;
  }

  // Build modal
  const overlay = document.createElement('div');
  overlay.id = 'assignModal';
  overlay.style.cssText = 'position:fixed;inset:0;z-index:200;display:flex;align-items:center;justify-content:center;background:rgba(0,0,0,0.65);backdrop-filter:blur(4px)';
  overlay.innerHTML = `
    <div style="background:var(--surface,#1e1e2e);border:1px solid var(--border,#333);border-radius:12px;padding:1.5rem;width:min(480px,92vw);max-height:90vh;overflow-y:auto;box-shadow:0 12px 32px rgba(0,0,0,0.5);font-family:inherit;color:var(--text,#fff)">
      <h3 style="margin:0 0 .25rem 0;font-size:1.1rem">Tildel adgang til enhed</h3>
      <div style="opacity:0.7;font-size:.85rem;margin-bottom:1rem">${device.device_name || device.device_id}</div>

      <div style="font-size:.85rem;font-weight:600;margin-bottom:.4rem;opacity:0.85">Nuværende adgang</div>
      <div id="assignList" style="border:1px solid var(--border,#333);border-radius:6px;padding:.5rem;background:rgba(255,255,255,0.02);margin-bottom:1.25rem;font-size:.85rem;min-height:2.5rem">
        <div style="opacity:0.5;text-align:center;padding:.5rem">Indlæser...</div>
      </div>

      <div style="font-size:.85rem;font-weight:600;margin-bottom:.4rem;opacity:0.85">Tilføj ny</div>
      <input type="email" id="assignEmail" placeholder="bruger@example.dk" style="width:100%;padding:.55rem .7rem;border:1px solid var(--border,#333);border-radius:6px;background:rgba(255,255,255,0.05);color:inherit;font-size:.95rem;margin-bottom:.75rem;box-sizing:border-box" autocomplete="email" />

      <div style="display:flex;flex-direction:column;gap:.35rem;margin-bottom:1.25rem">
        <label style="display:flex;align-items:center;gap:.5rem;font-size:.85rem;cursor:pointer">
          <input type="radio" name="assignKind" value="delegate" checked />
          <span><strong>Tildel adgang</strong> — modtager ser enheden, ejer beholder fuld kontrol</span>
        </label>
        <label style="display:flex;align-items:center;gap:.5rem;font-size:.85rem;cursor:pointer">
          <input type="radio" name="assignKind" value="transfer" />
          <span><strong>Overdrag ejerskab</strong> — modtager bliver ny ejer</span>
        </label>
      </div>

      <div style="display:flex;gap:.5rem;justify-content:flex-end">
        <button id="assignCancel" type="button" style="padding:.5rem 1rem;border:1px solid var(--border,#444);border-radius:6px;background:transparent;color:inherit;cursor:pointer">Luk</button>
        <button id="assignSubmit" type="button" style="padding:.5rem 1.25rem;border:none;border-radius:6px;background:var(--primary,#3b82f6);color:#fff;font-weight:600;cursor:pointer">Tilføj</button>
      </div>

      <div id="assignStatus" style="margin-top:.85rem;font-size:.85rem;min-height:1.2em"></div>
    </div>`;
  document.body.appendChild(overlay);

  // Load + render existing assignments for this device
  const refreshList = async () => {
    const list = overlay.querySelector('#assignList');
    list.innerHTML = '<div style="opacity:0.5;text-align:center;padding:.5rem">Indlæser...</div>';
    try {
      const { data, error } = await supabase.rpc('list_device_access', { p_device_id: device.device_id });
      if (error) throw error;
      list.innerHTML = '';

      // Show owner first
      if (data && data.length) {
        const ownerRow = data.find(r => r.access_kind === 'owner');
        const assigned = data.filter(r => r.access_kind === 'assignment');
        if (ownerRow) {
          const row = document.createElement('div');
          row.style.cssText = 'display:flex;align-items:center;justify-content:space-between;padding:.4rem .25rem;border-bottom:1px solid rgba(255,255,255,0.06)';
          row.innerHTML = `<span><strong>👑 Ejer:</strong> ${ownerRow.email}</span>`;
          list.appendChild(row);
        }
        for (const a of assigned) {
          const row = document.createElement('div');
          row.style.cssText = 'display:flex;align-items:center;justify-content:space-between;padding:.4rem .25rem;border-bottom:1px solid rgba(255,255,255,0.06)';
          const span = document.createElement('span');
          span.innerHTML = `👤 ${a.email}`;
          const btn = document.createElement('button');
          btn.type = 'button';
          btn.textContent = '✕ Fjern';
          btn.style.cssText = 'padding:.25rem .6rem;border:1px solid rgba(239,68,68,0.4);border-radius:4px;background:transparent;color:#ef4444;cursor:pointer;font-size:.78rem';
          btn.addEventListener('click', async () => {
            if (!confirm(`Fjern adgang for ${a.email}?`)) return;
            btn.disabled = true;
            btn.textContent = '...';
            const { error: revokeErr } = await supabase
              .from('device_assignments')
              .update({ revoked_at: new Date().toISOString() })
              .eq('id', a.assignment_id);
            if (revokeErr) {
              alert('Fejl: ' + revokeErr.message);
              btn.disabled = false;
              btn.textContent = '✕ Fjern';
              return;
            }
            await refreshList();
            if (typeof loadDevices === 'function') loadDevices();
          });
          row.append(span, btn);
          list.appendChild(row);
        }
      }
      if (!list.children.length) {
        list.innerHTML = '<div style="opacity:0.5;text-align:center;padding:.5rem">Ingen tildelinger</div>';
      }
    } catch (err) {
      list.innerHTML = `<div style="color:#ef4444;text-align:center;padding:.5rem">Kunne ikke indlæse: ${err.message || err}</div>`;
    }
  };
  refreshList();

  const close = () => overlay.remove();
  overlay.addEventListener('click', e => { if (e.target === overlay) close(); });
  overlay.querySelector('#assignCancel').addEventListener('click', close);
  document.addEventListener('keydown', function escClose(e) {
    if (e.key === 'Escape') { close(); document.removeEventListener('keydown', escClose); }
  });
  setTimeout(() => overlay.querySelector('#assignEmail').focus(), 50);

  overlay.querySelector('#assignSubmit').addEventListener('click', async () => {
    const email = overlay.querySelector('#assignEmail').value.trim().toLowerCase();
    const kind = overlay.querySelector('input[name="assignKind"]:checked').value;
    const status = overlay.querySelector('#assignStatus');
    status.style.color = 'var(--text,#fff)';
    if (!email || !email.includes('@')) {
      status.textContent = 'Ugyldig e-mail.';
      status.style.color = '#ef4444';
      return;
    }
    status.textContent = 'Søger bruger...';
    try {
      // Look up the recipient by email — works only with sufficient
      // privileges. The auth.users table isn't directly queryable, so we
      // use a small RPC (defined in a migration below) that resolves
      // email → user_id under SECURITY DEFINER.
      const { data: lookup, error: lookupErr } = await supabase.rpc('find_user_id_by_email', { p_email: email });
      if (lookupErr) throw lookupErr;
      if (!lookup) {
        status.textContent = 'Ingen bruger fundet med den e-mail.';
        status.style.color = '#ef4444';
        return;
      }
      const userId = lookup;

      if (kind === 'transfer') {
        const { error } = await supabase
          .from('remote_devices')
          .update({ owner_id: userId })
          .eq('device_id', device.device_id);
        if (error) throw error;
        status.style.color = '#22c55e';
        status.textContent = '✓ Ejerskab overdraget';
      } else {
        const { data: { session } } = await supabase.auth.getSession();
        const { error } = await supabase
          .from('device_assignments')
          .insert({ device_id: device.device_id, user_id: userId, assigned_by: session?.user?.id });
        if (error) throw error;
        status.style.color = '#22c55e';
        status.textContent = '✓ Adgang tildelt';
      }

      // Refresh the in-modal list and the main device card view, but
      // keep the modal open so the admin can stack more assignments.
      overlay.querySelector('#assignEmail').value = '';
      await refreshList();
      if (typeof loadDevices === 'function') loadDevices();
      setTimeout(() => { status.textContent = ''; }, 2000);
    } catch (err) {
      status.style.color = '#ef4444';
      status.textContent = 'Fejl: ' + (err.message || err);
    }
  });
}

// Export
window.initDevices = initDevices;
window.loadDevices = loadDevices;
window.assignDevicePrompt = assignDevicePrompt;
