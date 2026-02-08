// Device Management Module
// Handles device list, approval, and selection

async function initDevices() {
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

    // Check if user is admin/super_admin
    const { data: approval } = await supabase
      .from('user_approvals')
      .select('role')
      .eq('user_id', session.user.id)
      .single();
    
    const isAdmin = approval && (approval.role === 'admin' || approval.role === 'super_admin');

    let devices;
    let error;

    if (isAdmin) {
      // Admins see ALL devices
      const result = await supabase
        .from('remote_devices')
        .select('*')
        .order('last_seen', { ascending: false });
      devices = result.data;
      error = result.error;
    } else {
      // Regular users: use get_user_devices function to get assigned devices
      const result = await supabase.rpc('get_user_devices', {
        p_user_id: session.user.id
      });
      devices = result.data;
      error = result.error;
    }

    if (error) throw error;

    loadingDevices.style.display = 'none';

    if (!devices || devices.length === 0) {
      emptyState.style.display = 'block';
      return;
    }

    // Render devices (deduplicate by device_id just in case)
    const uniqueDevices = devices.reduce((acc, device) => {
      acc[device.device_id] = device; // Keep last one
      return acc;
    }, {});
    
    Object.values(uniqueDevices).forEach(device => {
      const card = createDeviceCard(device);
      devicesList.appendChild(card);
    });

  } catch (error) {
    console.error('Failed to load devices:', error);
    loadingDevices.style.display = 'none';
    showToast('Kunne ikke indlæse enheder: ' + error.message, 'error');
  }
}

function createDeviceCard(device) {
  const card = document.createElement('div');
  card.className = `device-card ${device.is_online ? '' : 'offline'}`;
  
  const statusClass = device.is_online ? 'online' : 'offline';
  const statusText = device.is_online ? 'Online' : 'Offline';
  
  const lastSeen = device.last_seen 
    ? new Date(device.last_seen).toLocaleString()
    : 'Never';

  // Build DOM safely (no innerHTML with user data)
  const header = document.createElement('div');
  header.className = 'device-header';
  const nameEl = document.createElement('div');
  nameEl.className = 'device-name';
  nameEl.textContent = device.device_name || device.device_id;
  const badge = document.createElement('span');
  badge.className = `status-badge ${statusClass}`;
  badge.textContent = statusText;
  header.append(nameEl, badge);

  const info = document.createElement('div');
  info.className = 'device-info';
  const infoLines = [
    `🆔 ${device.device_id}`,
    `💻 ${device.platform || 'Unknown'} (${device.arch || 'Unknown'})`,
    `🖥️ ${device.cpu_count || '?'} CPUs`,
    `📅 Last seen: ${lastSeen}`
  ];
  infoLines.forEach(text => {
    const div = document.createElement('div');
    div.textContent = text;
    info.appendChild(div);
  });

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

async function claimDevice(device) {
  if (!await showConfirm(`Tilknyt enhed "${device.device_name || device.device_id}"?\n\nDette vil tildele enheden til din konto.`, { title: 'Tilknyt enhed', confirmText: 'Tilknyt', type: 'info', icon: '🔗' })) {
    return;
  }

  try {
    const { data: { session } } = await supabase.auth.getSession();
    if (!session) return;

    // Update device with owner
    const { error } = await supabase
      .from('remote_devices')
      .update({
        owner_id: session.user.id,
        approved_by: session.user.id,
        approved_at: new Date().toISOString()
      })
      .eq('device_id', device.device_id);

    if (error) throw error;

    // Log audit event
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

    // Reload devices
    await loadDevices();

    showToast('Enhed tilknyttet! Du kan nu oprette forbindelse.', 'success');

  } catch (error) {
    console.error('Failed to claim device:', error);
    showToast('Kunne ikke tilknytte enhed: ' + error.message, 'error');
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

    debug('Device deleted:', device.device_id);
    
    // Reload devices (or it will auto-reload via realtime subscription)
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
  }, 500); // Wait 500ms before reloading
}

function subscribeToDeviceUpdates() {
  // Subscribe to real-time device changes
  const channel = supabase
    .channel('devices-changes')
    .on('postgres_changes', {
      event: '*',
      schema: 'public',
      table: 'remote_devices'
    }, (payload) => {
      debug('Device update:', payload);
      // Debounced reload to prevent flickering
      debouncedReload();
    })
    .subscribe();

  debug('📡 Subscribed to device updates');
}

// Export
window.initDevices = initDevices;
window.loadDevices = loadDevices;
