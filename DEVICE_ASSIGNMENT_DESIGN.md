# 🔧 Device Assignment System Design

## 🎯 Goal

**Current Problem:** Devices are tied to the user who runs the agent, requiring login on each device.

**New Approach:** Devices auto-register and admins assign them to users - like TeamViewer!

---

## 📋 Current vs New Flow

### **Current Flow (Problematic)**
```
1. User runs agent on computer
2. Agent prompts for login
3. User enters credentials
4. Device registered to that user
5. Only that user can see/control it
```

**Problems:**
- ❌ Need to login on every device
- ❌ Device tied to one user
- ❌ Can't reassign devices
- ❌ Admin can't manage all devices

---

### **New Flow (TeamViewer-Style)**
```
1. User runs agent on computer
2. Agent auto-registers (no login needed)
3. Device appears in admin panel
4. Admin assigns device to user(s)
5. Assigned users can see/control it
```

**Benefits:**
- ✅ No login needed on devices
- ✅ Devices appear automatically
- ✅ Admin controls assignments
- ✅ Can reassign anytime
- ✅ Multiple users per device

---

## 🏗️ Architecture Changes

### **1. Device Registration (Agent)**

#### **Old Approach:**
```go
// Agent requires user login
func RegisterDevice() {
    email := promptForEmail()
    password := promptForPassword()
    
    // Authenticate
    user := authenticateUser(email, password)
    
    // Register device to user
    device := Device{
        DeviceID:   generateID(),
        DeviceName: getComputerName(),
        OwnerID:    user.ID,  // ❌ Tied to one user
    }
    
    insertDevice(device)
}
```

#### **New Approach:**
```go
// Agent auto-registers without login
func RegisterDevice() {
    // Generate unique device ID (persistent)
    deviceID := getOrCreateDeviceID()
    
    // Get computer info
    deviceName := getComputerName()
    platform := "Windows"
    
    // Register device (no owner yet)
    device := Device{
        DeviceID:   deviceID,
        DeviceName: deviceName,
        Platform:   platform,
        OwnerID:    nil,  // ✅ No owner - admin assigns later
        Status:     "online",
        Approved:   false, // ✅ Needs admin approval
    }
    
    upsertDevice(device)
}
```

---

### **2. Database Schema Changes**

#### **Update `remote_devices` Table:**

```sql
-- Add new columns
ALTER TABLE remote_devices
ADD COLUMN approved BOOLEAN DEFAULT FALSE,
ADD COLUMN assigned_users TEXT[], -- Array of user IDs
ADD COLUMN assigned_by TEXT REFERENCES auth.users(id),
ADD COLUMN assigned_at TIMESTAMPTZ;

-- Make owner_id nullable
ALTER TABLE remote_devices
ALTER COLUMN owner_id DROP NOT NULL;

-- Add index for unassigned devices
CREATE INDEX idx_unassigned_devices 
ON remote_devices(approved) 
WHERE owner_id IS NULL;
```

#### **New Table: `device_assignments`**

```sql
-- Track device-to-user assignments
CREATE TABLE device_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id TEXT REFERENCES remote_devices(device_id),
    user_id TEXT REFERENCES auth.users(id),
    assigned_by TEXT REFERENCES auth.users(id),
    assigned_at TIMESTAMPTZ DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    UNIQUE(device_id, user_id)
);

-- Index for quick lookups
CREATE INDEX idx_device_assignments_device 
ON device_assignments(device_id) 
WHERE revoked_at IS NULL;

CREATE INDEX idx_device_assignments_user 
ON device_assignments(user_id) 
WHERE revoked_at IS NULL;
```

---

### **3. Admin Panel Updates**

#### **New Section: Device Management**

```html
<!-- admin.html - Add new section -->
<div id="device-management">
    <h2>📱 Device Management</h2>
    
    <!-- Unassigned Devices -->
    <div class="section">
        <h3>Unassigned Devices (Pending)</h3>
        <table id="unassigned-devices">
            <thead>
                <tr>
                    <th>Device Name</th>
                    <th>Platform</th>
                    <th>First Seen</th>
                    <th>Status</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                <!-- Populated dynamically -->
            </tbody>
        </table>
    </div>
    
    <!-- Assigned Devices -->
    <div class="section">
        <h3>Assigned Devices</h3>
        <table id="assigned-devices">
            <thead>
                <tr>
                    <th>Device Name</th>
                    <th>Assigned To</th>
                    <th>Assigned By</th>
                    <th>Date</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                <!-- Populated dynamically -->
            </tbody>
        </table>
    </div>
</div>
```

#### **Assignment Dialog:**

```javascript
// Show assignment dialog
function showAssignmentDialog(device) {
    const dialog = `
        <div class="dialog">
            <h3>Assign Device: ${device.device_name}</h3>
            
            <label>Assign to User:</label>
            <select id="user-select">
                <!-- Populated with approved users -->
            </select>
            
            <label>
                <input type="checkbox" id="approve-device">
                Approve device for use
            </label>
            
            <div class="actions">
                <button onclick="assignDevice()">Assign</button>
                <button onclick="closeDialog()">Cancel</button>
            </div>
        </div>
    `;
    
    showDialog(dialog);
}

// Assign device to user
async function assignDevice() {
    const deviceId = currentDevice.device_id;
    const userId = document.getElementById('user-select').value;
    const approve = document.getElementById('approve-device').checked;
    
    // Insert assignment
    const { error } = await supabase
        .from('device_assignments')
        .insert({
            device_id: deviceId,
            user_id: userId,
            assigned_by: currentAdmin.id
        });
    
    if (approve) {
        // Approve device
        await supabase
            .from('remote_devices')
            .update({ approved: true })
            .eq('device_id', deviceId);
    }
    
    if (!error) {
        alert('Device assigned successfully!');
        refreshDeviceList();
    }
}
```

---

### **4. Controller Updates**

#### **Fetch Assigned Devices:**

```go
// controller/internal/supabase/client.go

// GetAssignedDevices fetches devices assigned to current user
func (c *Client) GetAssignedDevices(userID string) ([]Device, error) {
    // Query devices assigned to this user
    url := fmt.Sprintf(
        "%s/rest/v1/rpc/get_user_devices?user_id=%s",
        c.URL,
        userID,
    )
    
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("apikey", c.AnonKey)
    req.Header.Set("Authorization", "Bearer "+c.AuthToken)
    
    resp, err := c.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var devices []Device
    if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
        return nil, err
    }
    
    return devices, nil
}
```

#### **Database Function:**

```sql
-- Function to get devices assigned to a user
CREATE OR REPLACE FUNCTION get_user_devices(user_id TEXT)
RETURNS TABLE (
    device_id TEXT,
    device_name TEXT,
    platform TEXT,
    status TEXT,
    last_heartbeat TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        d.device_id,
        d.device_name,
        d.platform,
        d.status,
        d.last_heartbeat
    FROM remote_devices d
    INNER JOIN device_assignments da 
        ON d.device_id = da.device_id
    WHERE da.user_id = $1
        AND da.revoked_at IS NULL
        AND d.approved = TRUE
    ORDER BY d.last_heartbeat DESC;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
```

---

### **5. Agent Updates**

#### **Remove Login Requirement:**

```go
// agent/cmd/remote-agent/main.go

func main() {
    log.Println("🚀 Starting Remote Desktop Agent...")
    
    // OLD: Require login
    // user := promptForLogin()
    
    // NEW: Auto-register device
    deviceID := getOrCreateDeviceID()
    deviceName := getComputerName()
    
    // Register device (no user needed)
    device := &device.Device{
        DeviceID:   deviceID,
        DeviceName: deviceName,
        Platform:   "Windows",
        Status:     "online",
    }
    
    if err := device.Register(); err != nil {
        log.Fatalf("Failed to register device: %v", err)
    }
    
    log.Printf("✅ Device registered: %s", deviceName)
    log.Println("⏳ Waiting for admin to assign this device...")
    
    // Start heartbeat
    go device.StartHeartbeat()
    
    // Listen for sessions
    listenForSessions(deviceID)
}
```

#### **Persistent Device ID:**

```go
// agent/internal/device/id.go

// getOrCreateDeviceID returns a persistent device ID
func getOrCreateDeviceID() string {
    // Check if ID exists in registry/file
    id, err := loadDeviceID()
    if err == nil && id != "" {
        return id
    }
    
    // Generate new ID
    id = generateDeviceID()
    
    // Save for future use
    saveDeviceID(id)
    
    return id
}

func generateDeviceID() string {
    // Use hardware info for stable ID
    hostname, _ := os.Hostname()
    
    // Get MAC address or other hardware ID
    hwID := getHardwareID()
    
    // Generate unique ID
    data := fmt.Sprintf("%s-%s-%d", hostname, hwID, time.Now().Unix())
    hash := sha256.Sum256([]byte(data))
    
    return fmt.Sprintf("device_%x", hash[:16])
}

func saveDeviceID(id string) error {
    // Windows: Save to registry
    // Or save to file in AppData
    configPath := filepath.Join(os.Getenv("APPDATA"), "RemoteDesktop", "device.id")
    return os.WriteFile(configPath, []byte(id), 0644)
}

func loadDeviceID() (string, error) {
    configPath := filepath.Join(os.Getenv("APPDATA"), "RemoteDesktop", "device.id")
    data, err := os.ReadFile(configPath)
    if err != nil {
        return "", err
    }
    return string(data), nil
}
```

---

## 🔄 Complete Workflow

### **1. Device Registration**
```
1. User downloads agent.exe
2. Runs agent.exe on computer
3. Agent generates unique device ID
4. Agent registers device in database
5. Device shows as "Unassigned" in admin panel
```

### **2. Admin Assignment**
```
1. Admin opens admin panel
2. Sees new unassigned device
3. Clicks "Assign"
4. Selects user from dropdown
5. Optionally approves device
6. Device now assigned to user
```

### **3. User Access**
```
1. User opens controller.exe
2. Logs in with credentials
3. Sees devices assigned to them
4. Can connect to assigned devices
```

### **4. Reassignment**
```
1. Admin can revoke assignment
2. Admin can assign to different user
3. Changes take effect immediately
```

---

## 📊 Database Queries

### **Get Unassigned Devices:**
```sql
SELECT * FROM remote_devices
WHERE owner_id IS NULL
  AND approved = FALSE
ORDER BY created_at DESC;
```

### **Get User's Devices:**
```sql
SELECT d.* 
FROM remote_devices d
INNER JOIN device_assignments da ON d.device_id = da.device_id
WHERE da.user_id = $1
  AND da.revoked_at IS NULL
  AND d.approved = TRUE;
```

### **Assign Device:**
```sql
INSERT INTO device_assignments (device_id, user_id, assigned_by)
VALUES ($1, $2, $3);

UPDATE remote_devices
SET approved = TRUE
WHERE device_id = $1;
```

### **Revoke Assignment:**
```sql
UPDATE device_assignments
SET revoked_at = NOW()
WHERE device_id = $1 AND user_id = $2;
```

---

## 🎯 Implementation Checklist

### **Phase 1: Database (1 day)**
- [ ] Add columns to `remote_devices`
- [ ] Create `device_assignments` table
- [ ] Create `get_user_devices()` function
- [ ] Update RLS policies
- [ ] Test queries

### **Phase 2: Agent (1 day)**
- [ ] Remove login requirement
- [ ] Implement persistent device ID
- [ ] Update registration logic
- [ ] Test auto-registration
- [ ] Update agent documentation

### **Phase 3: Admin Panel (2 days)**
- [ ] Add device management section
- [ ] List unassigned devices
- [ ] Create assignment dialog
- [ ] Implement assign/revoke
- [ ] Test assignment flow

### **Phase 4: Controller (1 day)**
- [ ] Update `GetDevices()` to use assignments
- [ ] Test with assigned devices
- [ ] Handle no devices case
- [ ] Update documentation

### **Phase 5: Testing (1 day)**
- [ ] Test full workflow
- [ ] Test multiple assignments
- [ ] Test reassignment
- [ ] Test revocation
- [ ] Update user guides

**Total: ~1 week**

---

## 🚀 Migration Plan

### **For Existing Devices:**

```sql
-- Migrate existing devices to new system
-- Keep current owner as assigned user

INSERT INTO device_assignments (device_id, user_id, assigned_by)
SELECT 
    device_id,
    owner_id,
    owner_id  -- Self-assigned
FROM remote_devices
WHERE owner_id IS NOT NULL;

-- Mark existing devices as approved
UPDATE remote_devices
SET approved = TRUE
WHERE owner_id IS NOT NULL;
```

---

## ✅ Benefits

### **For Admins:**
- ✅ Central device management
- ✅ Easy assignment/reassignment
- ✅ See all devices at a glance
- ✅ Control who accesses what

### **For Users:**
- ✅ No login needed on devices
- ✅ Just run agent.exe
- ✅ Devices appear automatically
- ✅ Simple deployment

### **For IT:**
- ✅ Deploy agent via GPO/script
- ✅ No user interaction needed
- ✅ Centralized management
- ✅ Easy to scale

---

## 🎉 Summary

This design transforms the system from **user-centric** to **admin-managed**, making it much more practical for real-world deployment!

**Key Changes:**
1. Agents auto-register (no login)
2. Devices appear in admin panel
3. Admins assign devices to users
4. Users see only assigned devices
5. Easy reassignment

**Like TeamViewer, but better!** 🚀

---

## 📝 Next Steps

1. **Review this design** - Make sure it fits your needs
2. **Update database schema** - Add new tables/columns
3. **Update agent** - Remove login, add auto-registration
4. **Update admin panel** - Add device management
5. **Update controller** - Use assignments
6. **Test thoroughly** - Ensure smooth workflow

**Ready to implement?**
