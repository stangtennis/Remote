// Remote Desktop Application - Main Server
const express = require('express');
const http = require('http');
const socketIo = require('socket.io');
const path = require('path');
const { createClient } = require('@supabase/supabase-js');
const cors = require('cors');
const bcrypt = require('bcrypt');
const jwt = require('jsonwebtoken');

// Initialize Express app
const app = express();
const server = http.createServer(app);
const io = socketIo(server, {
  cors: {
    origin: "*",
    methods: ["GET", "POST"]
  }
});

// Supabase configuration
const supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
const supabaseKey = 'sb_publishable_OwJ04BrJ6gZZ57fSeT-q5Q_mSsVaNia';
const supabase = createClient(supabaseUrl, supabaseKey);

// Middleware
app.use(cors());
app.use(express.json());
app.use(express.static(path.join(__dirname, 'public')));

// JWT Secret (in production, use environment variable)
const JWT_SECRET = 'your-jwt-secret-key';

// Authentication middleware
const authenticateToken = (req, res, next) => {
  const authHeader = req.headers['authorization'];
  const token = authHeader && authHeader.split(' ')[1];

  if (!token) {
    return res.status(401).json({ error: 'Access token required' });
  }

  jwt.verify(token, JWT_SECRET, (err, user) => {
    if (err) {
      return res.status(403).json({ error: 'Invalid token' });
    }
    req.user = user;
    next();
  });
};

// Routes

// Health check
app.get('/api/health', (req, res) => {
  res.json({ status: 'OK', timestamp: new Date().toISOString() });
});

// User registration
app.post('/api/register', async (req, res) => {
  try {
    const { email, username, fullName, password } = req.body;

    // Hash password
    const saltRounds = 10;
    const hashedPassword = await bcrypt.hash(password, saltRounds);

    // Insert user into database
    const { data, error } = await supabase
      .from('users')
      .insert([
        {
          email,
          username,
          full_name: fullName,
          password_hash: hashedPassword
        }
      ])
      .select()
      .single();

    if (error) {
      return res.status(400).json({ error: error.message });
    }

    // Generate JWT token
    const token = jwt.sign(
      { userId: data.id, username: data.username },
      JWT_SECRET,
      { expiresIn: '24h' }
    );

    res.json({
      message: 'User registered successfully',
      token,
      user: {
        id: data.id,
        email: data.email,
        username: data.username,
        fullName: data.full_name
      }
    });
  } catch (error) {
    console.error('Registration error:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// User login
app.post('/api/login', async (req, res) => {
  try {
    const { username, password } = req.body;

    // Find user in database
    const { data: user, error } = await supabase
      .from('users')
      .select('*')
      .eq('username', username)
      .single();

    if (error || !user) {
      return res.status(401).json({ error: 'Invalid credentials' });
    }

    // Verify password
    const validPassword = await bcrypt.compare(password, user.password_hash);
    if (!validPassword) {
      return res.status(401).json({ error: 'Invalid credentials' });
    }

    // Update last login
    await supabase
      .from('users')
      .update({ last_login: new Date().toISOString() })
      .eq('id', user.id);

    // Generate JWT token
    const token = jwt.sign(
      { userId: user.id, username: user.username },
      JWT_SECRET,
      { expiresIn: '24h' }
    );

    res.json({
      message: 'Login successful',
      token,
      user: {
        id: user.id,
        email: user.email,
        username: user.username,
        fullName: user.full_name
      }
    });
  } catch (error) {
    console.error('Login error:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// Get user's devices
app.get('/api/devices', authenticateToken, async (req, res) => {
  try {
    const { data, error } = await supabase
      .from('remote_devices')
      .select('*')
      .eq('owner_id', req.user.userId)
      .order('created_at', { ascending: false });

    if (error) {
      return res.status(400).json({ error: error.message });
    }

    res.json(data);
  } catch (error) {
    console.error('Get devices error:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// Add new device
app.post('/api/devices', authenticateToken, async (req, res) => {
  try {
    const { deviceName, deviceType, operatingSystem, ipAddress, port } = req.body;

    // Generate unique access key
    const accessKey = require('crypto').randomBytes(32).toString('hex');

    const { data, error } = await supabase
      .from('remote_devices')
      .insert([
        {
          owner_id: req.user.userId,
          device_name: deviceName,
          device_type: deviceType,
          operating_system: operatingSystem,
          ip_address: ipAddress,
          port: port || 3389,
          access_key: accessKey
        }
      ])
      .select()
      .single();

    if (error) {
      return res.status(400).json({ error: error.message });
    }

    res.json({
      message: 'Device added successfully',
      device: data
    });
  } catch (error) {
    console.error('Add device error:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// Get user's sessions
app.get('/api/sessions', authenticateToken, async (req, res) => {
  try {
    const { data, error } = await supabase
      .from('remote_sessions')
      .select(`
        *,
        remote_devices (
          device_name,
          device_type,
          operating_system
        )
      `)
      .eq('user_id', req.user.userId)
      .order('started_at', { ascending: false })
      .limit(50);

    if (error) {
      return res.status(400).json({ error: error.message });
    }

    res.json(data);
  } catch (error) {
    console.error('Get sessions error:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// Start remote session
app.post('/api/sessions/start', authenticateToken, async (req, res) => {
  try {
    const { deviceId } = req.body;

    // Verify user has access to device
    const { data: device, error: deviceError } = await supabase
      .from('remote_devices')
      .select('*')
      .eq('id', deviceId)
      .eq('owner_id', req.user.userId)
      .single();

    if (deviceError || !device) {
      return res.status(404).json({ error: 'Device not found or access denied' });
    }

    // Generate session token
    const sessionToken = require('crypto').randomBytes(32).toString('hex');

    // Create session record
    const { data: session, error: sessionError } = await supabase
      .from('remote_sessions')
      .insert([
        {
          user_id: req.user.userId,
          device_id: deviceId,
          session_token: sessionToken,
          status: 'connecting'
        }
      ])
      .select()
      .single();

    if (sessionError) {
      return res.status(400).json({ error: sessionError.message });
    }

    res.json({
      message: 'Session started',
      session: session,
      device: device
    });
  } catch (error) {
    console.error('Start session error:', error);
    res.status(500).json({ error: 'Internal server error' });
  }
});

// Socket.IO connection handling for real-time communication
io.on('connection', (socket) => {
  console.log('User connected:', socket.id);

  // Join session room
  socket.on('join-session', (sessionToken) => {
    socket.join(sessionToken);
    console.log(`Socket ${socket.id} joined session ${sessionToken}`);
  });

  // Handle remote desktop control events
  socket.on('mouse-move', (data) => {
    socket.to(data.sessionToken).emit('mouse-move', data);
  });

  socket.on('mouse-click', (data) => {
    socket.to(data.sessionToken).emit('mouse-click', data);
  });

  socket.on('key-press', (data) => {
    socket.to(data.sessionToken).emit('key-press', data);
  });

  socket.on('screen-capture', (data) => {
    socket.to(data.sessionToken).emit('screen-capture', data);
  });

  // Handle disconnection
  socket.on('disconnect', () => {
    console.log('User disconnected:', socket.id);
  });
});

// Serve main application
app.get('/', (req, res) => {
  res.sendFile(path.join(__dirname, 'public', 'index.html'));
});

// Start server
const PORT = process.env.PORT || 3000;
server.listen(PORT, () => {
  console.log(`Remote Desktop Server running on port ${PORT}`);
  console.log(`Access the application at: http://localhost:${PORT}`);
});

module.exports = app;
