# Settings Guide

## Overview
The controller now includes a comprehensive settings system that lets you customize every aspect of the remote desktop experience. You can enable/disable high-quality mode and adjust all performance settings to match your hardware and network capabilities.

## Accessing Settings
1. Launch the controller
2. Click on the **"Settings"** tab
3. All settings are saved automatically when changed

## Settings Categories

### 🎯 Performance Mode

#### High-Performance Mode Toggle
- **Enable High-Performance Mode** - Master switch for ultra-quality settings
- When enabled: Automatically applies 4K, 60 FPS, and maximum quality
- When disabled: Allows manual configuration of all settings
- **Recommended:** Enable for powerful computers with good internet

### 🎬 Quick Presets

Three one-click presets for different scenarios:

#### Ultra (4K, 60 FPS)
- **Resolution:** 4K (3840x2160)
- **FPS:** 60
- **Quality:** 80%
- **Bitrate:** 50 Mbps
- **Best for:** High-end gaming PCs, workstations
- **Requires:** Powerful GPU, 50+ Mbps internet

#### High (1440p, 60 FPS)
- **Resolution:** 1440p (2560x1440)
- **FPS:** 60
- **Quality:** 70%
- **Bitrate:** 25 Mbps
- **Best for:** Modern laptops, mid-range PCs
- **Requires:** Decent GPU, 25+ Mbps internet

#### Low (1080p, 30 FPS)
- **Resolution:** 1080p (1920x1080)
- **FPS:** 30
- **Quality:** 50%
- **Bitrate:** 10 Mbps
- **Best for:** Older computers, slow internet
- **Requires:** Basic GPU, 10+ Mbps internet

### 📺 Video Settings

#### Resolution
Choose maximum resolution for remote desktop:
- **720p** (1280x720) - Basic quality, low bandwidth
- **1080p** (1920x1080) - Standard HD quality
- **1440p** (2560x1440) - High quality, sharp details
- **4K** (3840x2160) - Ultra quality, requires powerful hardware

**Note:** Actual resolution may be lower if remote device doesn't support it.

#### Target FPS
Set target frames per second:
- **30 FPS** - Smooth for basic tasks, low bandwidth
- **60 FPS** - Very smooth, recommended for most users
- **120 FPS** - Ultra smooth, for high-refresh displays

**Note:** Higher FPS requires more bandwidth and processing power.

#### Codec
Select video compression codec:
- **H.264** - Best compatibility, widely supported
- **H.265** - Better compression, newer hardware required
- **VP9** - Open source, good quality

**Recommended:** H.264 for best compatibility.

#### Video Quality Slider
Fine-tune video quality (1-100%):
- **1-30%** - Low quality, very compressed
- **40-60%** - Medium quality, balanced
- **70-90%** - High quality, minimal compression
- **90-100%** - Maximum quality, large bandwidth

**Tip:** Start at 80% and adjust based on your experience.

### 🌐 Network Settings

#### Max Bitrate
Maximum network bandwidth to use (5-100 Mbps):
- **5-10 Mbps** - Slow internet, basic quality
- **15-25 Mbps** - Average internet, good quality
- **30-50 Mbps** - Fast internet, high quality
- **50-100 Mbps** - Very fast internet, ultra quality

**Important:** Set this below your actual upload/download speed to avoid buffering.

#### Adaptive Bitrate
- **Enabled:** Automatically adjusts quality based on network conditions
- **Disabled:** Uses fixed bitrate (may cause stuttering on slow networks)
- **Recommended:** Keep enabled for stable experience

### ⚡ Advanced Options

#### Hardware Acceleration
- **Enabled:** Uses GPU for video encoding/decoding
- **Disabled:** Uses CPU only (slower but more compatible)
- **Recommended:** Enable if you have a dedicated GPU

**Benefits:**
- Lower CPU usage
- Higher frame rates
- Better quality at same bitrate

#### Low Latency Mode
- **Enabled:** Minimizes delay between input and response
- **Disabled:** May buffer more for smoother video
- **Recommended:** Enable for interactive tasks (gaming, design work)

**Trade-offs:**
- Enabled: Lower latency, may have occasional stutters
- Disabled: Smoother video, slightly higher latency

### 🎨 Features

#### Enable File Transfer
- **Enabled:** Can send files to remote device
- **Disabled:** File transfer button hidden
- **Status:** UI ready, implementation coming soon

#### Enable Clipboard Sync
- **Enabled:** Clipboard content shared between devices
- **Disabled:** Clipboards remain separate
- **Status:** UI ready, implementation coming soon

#### Enable Audio Streaming
- **Enabled:** Stream audio from remote device
- **Disabled:** Video only, no audio
- **Status:** Coming soon

### 🎨 Appearance

#### Theme
- **Dark:** Dark theme (default, easy on eyes)
- **Light:** Light theme (better for bright environments)

**Note:** Restart required to apply theme changes.

### 🔄 Reset to Defaults

The **"Reset to Defaults"** button restores all settings to their original values:
- High-Performance Mode: Enabled
- Resolution: 4K
- FPS: 60
- Quality: 80%
- All features: Enabled

**Warning:** This cannot be undone. Your custom settings will be lost.

## Settings Storage

Settings are automatically saved to:
- **Windows:** `%APPDATA%\RemoteDesktopController\settings.json`
- **Linux:** `~/.config/RemoteDesktopController/settings.json`
- **macOS:** `~/Library/Application Support/RemoteDesktopController/settings.json`

You can manually edit this file if needed (JSON format).

## Recommended Settings by Use Case

### 📊 Office Work / Browsing
- Resolution: 1080p or 1440p
- FPS: 30 or 60
- Quality: 60-70%
- Bitrate: 15-25 Mbps
- Low Latency: Disabled

### 🎮 Gaming / Interactive
- Resolution: 1080p or 1440p
- FPS: 60 or 120
- Quality: 70-80%
- Bitrate: 25-50 Mbps
- Low Latency: Enabled
- Hardware Acceleration: Enabled

### 🎨 Design / Video Editing
- Resolution: 1440p or 4K
- FPS: 60
- Quality: 80-90%
- Bitrate: 30-50 Mbps
- Hardware Acceleration: Enabled

### 📱 Mobile / Slow Connection
- Resolution: 720p or 1080p
- FPS: 30
- Quality: 40-50%
- Bitrate: 5-10 Mbps
- Adaptive Bitrate: Enabled

## Troubleshooting

### Video is Laggy
1. Lower resolution (try 1080p)
2. Reduce FPS to 30
3. Lower video quality slider
4. Reduce max bitrate
5. Enable adaptive bitrate

### High CPU Usage
1. Enable hardware acceleration
2. Lower resolution
3. Reduce FPS
4. Use H.264 codec (most efficient)

### Poor Quality Despite High Settings
1. Check your internet speed
2. Increase bitrate limit
3. Disable adaptive bitrate temporarily
4. Check remote device's upload speed

### Settings Not Saving
1. Check file permissions in config directory
2. Ensure disk has free space
3. Try running as administrator (Windows)
4. Check logs for error messages

## Tips for Best Experience

1. **Match your hardware** - Don't set 4K if your GPU can't handle it
2. **Test your network** - Use speedtest.net to check your bandwidth
3. **Start high, adjust down** - Begin with Ultra preset, lower if needed
4. **Monitor performance** - Watch FPS and latency in viewer status bar
5. **Use wired connection** - Ethernet is more stable than WiFi
6. **Close other apps** - Free up bandwidth and system resources
7. **Update drivers** - Keep GPU drivers current for best performance

## Future Enhancements

Planned settings features:
- [ ] Custom resolution input
- [ ] Per-device settings profiles
- [ ] Bandwidth usage graphs
- [ ] Performance monitoring
- [ ] Automatic quality adjustment
- [ ] Network diagnostics
- [ ] Keyboard shortcut customization
- [ ] Multi-monitor configuration

## Support

If you encounter issues with settings:
1. Check the logs in `logs/` directory
2. Try resetting to defaults
3. Restart the application
4. Report issues with log files
