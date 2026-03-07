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

    // Load devices, tags, and favorites in parallel
    const devicesPromise = isAdmin
      ? supabase.from('remote_devices').select('*').order('last_seen', { ascending: false })
      : supabase.rpc('get_user_devices', { p_user_id: session.user.id });

    const tagsPromise = supabase.from('device_tags').select('device_id, tag');
    const favoritesPromise = supabase.from('user_device_favorites').select('device_id').eq('user_id', session.user.id);

    const [devicesResult, tagsResult, favoritesResult] = await Promise.all([devicesPromise, tagsPromise, favoritesPromise]);

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
  for (const device of filtered) {
    devicesList.appendChild(createDeviceCard(device));
  }
}

function createDeviceCard(device) {
  const card = document.createElement('div');
  card.className = `device-card ${device.is_online ? '' : 'offline'}`;
  card.dataset.deviceId = device.device_id;

  const statusClass = device.is_online ? 'online' : 'offline';
  const statusText = device.is_online ? 'Online' : 'Offline';

  const lastSeen = device.last_seen
    ? new Date(device.last_seen).toLocaleString()
    : 'Never';

  // Build DOM safely (no innerHTML with user data)
  const header = document.createElement('div');
  header.className = 'device-header';

  // Favorite star
  const starBtn = document.createElement('button');
  starBtn.className = 'btn btn-icon device-fav-btn';
  starBtn.style.cssText = 'font-size: 1.1rem; padding: 0 0.25rem; min-width: auto; margin-right: 0.25rem;';
  const isFav = !!_userFavorites[device.device_id];
  starBtn.textContent = isFav ? '★' : '☆';
  starBtn.title = isFav ? 'Fjern fra favoritter' : 'Tilføj til favoritter';
  if (isFav) starBtn.style.color = '#f59e0b';
  starBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    toggleFavorite(device.device_id);
  });

  const nameEl = document.createElement('div');
  nameEl.className = 'device-name';
  nameEl.textContent = device.device_name || device.device_id;
  const badge = document.createElement('span');
  badge.className = `status-badge ${statusClass}`;
  badge.textContent = statusText;
  header.append(starBtn, nameEl, badge);

  const info = document.createElement('div');
  info.className = 'device-info';
  const infoLines = [
    `💻 ${device.platform || 'Unknown'} (${device.arch || 'Unknown'})`,
    `🖥️ ${device.cpu_count || '?'} CPUs`,
    device.agent_version ? `📦 Agent ${device.agent_version}` : null,
    `📅 Last seen: ${lastSeen}`
  ].filter(Boolean);
  infoLines.forEach(text => {
    const div = document.createElement('div');
    div.textContent = text;
    info.appendChild(div);
  });

  // Tag badges
  const tags = _deviceTags[device.device_id] || [];
  if (tags.length > 0) {
    const tagsRow = document.createElement('div');
    tagsRow.style.cssText = 'display: flex; gap: 0.25rem; flex-wrap: wrap; margin-top: 0.25rem;';
    for (const tag of tags) {
      const tagBadge = document.createElement('span');
      tagBadge.style.cssText = 'background: rgba(99,102,241,0.2); color: var(--primary, #6366f1); padding: 0.1rem 0.4rem; border-radius: 9999px; font-size: 0.7rem; cursor: pointer;';
      tagBadge.textContent = tag;
      tagBadge.title = 'Klik for at fjerne tag';
      tagBadge.addEventListener('click', (e) => {
        e.stopPropagation();
        removeTag(device.device_id, tag);
      });
      tagsRow.appendChild(tagBadge);
    }
    info.appendChild(tagsRow);
  }

  card.append(header, info);

  // Action buttons
  const actions = document.createElement('div');
  actions.className = 'device-actions';

  if (device.is_online) {
    const connectBtn = document.createElement('button');
    connectBtn.className = 'btn btn-primary connect-btn';
    connectBtn.textContent = 'Connect';
    connectBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      startSession(device);
    });
    actions.appendChild(connectBtn);
  }

  const tagBtn = document.createElement('button');
  tagBtn.className = 'btn btn-secondary';
  tagBtn.textContent = '🏷️';
  tagBtn.title = 'Tilføj tag';
  tagBtn.style.minWidth = 'auto';
  tagBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    addTagPrompt(device.device_id);
  });
  actions.appendChild(tagBtn);

  const renameBtn = document.createElement('button');
  renameBtn.className = 'btn btn-secondary rename-btn';
  renameBtn.textContent = 'Rename';
  renameBtn.addEventListener('click', async (e) => {
    e.stopPropagation();
    await renameDevice(device);
  });
  actions.appendChild(renameBtn);

  const deleteBtn = document.createElement('button');
  deleteBtn.className = 'btn btn-danger delete-btn';
  deleteBtn.textContent = 'Delete';
  deleteBtn.addEventListener('click', async (e) => {
    e.stopPropagation();
    await deleteDevice(device);
  });
  actions.appendChild(deleteBtn);
  card.appendChild(actions);

  if (!device.owner_id) {
    const claimActions = document.createElement('div');
    claimActions.className = 'device-actions';
    const unassigned = document.createElement('span');
    unassigned.className = 'status-badge pending';
    unassigned.textContent = 'Unassigned';
    const claimBtn = document.createElement('button');
    claimBtn.className = 'btn btn-primary claim-btn';
    claimBtn.textContent = '🔗 Claim Device';
    claimBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      await claimDevice(device);
    });
    claimActions.append(unassigned, claimBtn);
    card.appendChild(claimActions);
  }

  return card;
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
  const tag = prompt('Indtast tag:');
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
  const newName = prompt(`Nyt navn for "${currentName}":`, currentName);
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

// Export
window.initDevices = initDevices;
window.loadDevices = loadDevices;
