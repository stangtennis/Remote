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

    // Fetch devices owned by current user OR pending approval (owner_id = null)
    const { data: devices, error } = await supabase
      .from('remote_devices')
      .select('*')
      .or(`owner_id.eq.${session.user.id},owner_id.is.null`)
      .order('last_seen', { ascending: false });

    if (error) throw error;

    loadingDevices.style.display = 'none';

    if (!devices || devices.length === 0) {
      emptyState.style.display = 'block';
      return;
    }

    // Render devices
    devices.forEach(device => {
      const card = createDeviceCard(device);
      devicesList.appendChild(card);
    });

  } catch (error) {
    console.error('Failed to load devices:', error);
    loadingDevices.style.display = 'none';
    alert('Failed to load devices: ' + error.message);
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
      <div>💻 ${device.platform || 'Unknown'} (${device.arch || 'Unknown'})</div>
      <div>🖥️ ${device.cpu_count || '?'} CPUs</div>
      <div>📅 Last seen: ${lastSeen}</div>
    </div>
    ${device.is_online ? `
      <div class="device-actions">
        <button class="btn btn-primary connect-btn" data-device-id="${device.device_id}">
          Connect
        </button>
      </div>
    ` : ''}
    ${!device.approved_at ? `
      <div class="device-actions">
        <span class="status-badge pending">Pending Approval</span>
        <button class="btn btn-primary approve-btn" data-device-id="${device.device_id}">
          Approve
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

  const approveBtn = card.querySelector('.approve-btn');
  if (approveBtn) {
    approveBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      await approveDevice(device);
    });
  }

  return card;
}

async function approveDevice(device) {
  try {
    const { data: { session } } = await supabase.auth.getSession();
    if (!session) return;

    // Update device with approval
    const { error } = await supabase
      .from('remote_devices')
      .update({
        approved_by: session.user.id,
        approved_at: new Date().toISOString(),
        owner_id: session.user.id
      })
      .eq('device_id', device.device_id);

    if (error) throw error;

    // Log audit event
    await supabase.rpc('log_audit_event', {
      p_session_id: null,
      p_device_id: device.device_id,
      p_event: 'DEVICE_APPROVED',
      p_details: { device_name: device.device_name },
      p_severity: 'info'
    });

    // Reload devices
    await loadDevices();

    alert('Device approved successfully!');

  } catch (error) {
    console.error('Failed to approve device:', error);
    alert('Failed to approve device: ' + error.message);
  }
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
      // Reload devices list on any change
      loadDevices();
    })
    .subscribe();

  console.log('📡 Subscribed to device updates');
}

// Export
window.initDevices = initDevices;
window.loadDevices = loadDevices;
