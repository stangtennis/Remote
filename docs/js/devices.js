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

  card.innerHTML = `
    <div class="device-header">
      <div class="device-name">${device.device_name || device.device_id}</div>
      <span class="status-badge ${statusClass}">${statusText}</span>
    </div>
    <div class="device-info">
      <div>🆔 ${device.device_id}</div>
      <div>💻 ${device.platform || 'Unknown'} (${device.arch || 'Unknown'})</div>
      <div>🖥️ ${device.cpu_count || '?'} CPUs</div>
      <div>📅 Last seen: ${lastSeen}</div>
    </div>
    ${device.is_online ? `
      <div class="device-actions">
        <button class="btn btn-primary connect-btn" data-device-id="${device.device_id}">
          Connect
        </button>
        <button class="btn btn-danger delete-btn" data-device-id="${device.device_id}">
          Delete
        </button>
      </div>
    ` : `
      <div class="device-actions">
        <button class="btn btn-danger delete-btn" data-device-id="${device.device_id}">
          Delete
        </button>
      </div>
    `}
    ${!device.owner_id ? `
      <div class="device-actions">
        <span class="status-badge pending">Unassigned</span>
        <button class="btn btn-primary claim-btn" data-device-id="${device.device_id}">
          🔗 Claim Device
        </button>
      </div>
    ` : ''}
  `;

  // Add click handlers
  const connectBtn = card.querySelector('.connect-btn');
  if (connectBtn) {
    connectBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      startSession(device);
    });
  }

  const claimBtn = card.querySelector('.claim-btn');
  if (claimBtn) {
    claimBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      await claimDevice(device);
    });
  }

  const deleteBtn = card.querySelector('.delete-btn');
  if (deleteBtn) {
    deleteBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      await deleteDevice(device);
    });
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

    console.log('Device deleted:', device.device_id);
    
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
      console.log('Device update:', payload);
      // Debounced reload to prevent flickering
      debouncedReload();
    })
    .subscribe();

  console.log('📡 Subscribed to device updates');
}

// Export
window.initDevices = initDevices;
window.loadDevices = loadDevices;
