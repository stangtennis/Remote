// File Transfer Module — Total Commander Style
// Dual-panel file browser with keyboard navigation over WebRTC data channel
// Protocol matches agent/internal/filetransfer/handler.go

const FileTransfer = {
  _channel: null,
  _pendingCallbacks: {},
  _nextFid: 1,
  _isOpen: false,

  // Dual panel state
  _panels: {
    left:  { path: '', entries: [], selected: new Set(), focusIdx: 0, sortCol: 'name', sortAsc: true },
    right: { path: '', entries: [], selected: new Set(), focusIdx: 0, sortCol: 'name', sortAsc: true }
  },
  _activePanel: 'left',

  setChannel(dc) {
    this._channel = dc;
    if (dc) dc.onmessage = (event) => this._handleMessage(event);
  },

  isReady() { return this._channel && this._channel.readyState === 'open'; },

  _send(obj) {
    if (!this.isReady()) { console.warn('File channel not ready'); return false; }
    this._channel.send(JSON.stringify(obj));
    return true;
  },

  _handleMessage(event) {
    let msg;
    if (typeof event.data === 'string') {
      try { msg = JSON.parse(event.data); } catch (e) { return; }
    } else if (event.data instanceof ArrayBuffer) {
      try { msg = JSON.parse(new TextDecoder().decode(event.data)); } catch (e) { return; }
    } else return;
    if (!msg.op) return;

    switch (msg.op) {
      case 'list': this._handleList(msg); break;
      case 'drives': this._handleDrives(msg); break;
      case 'put': this._handleDownloadChunk(msg); break;
      case 'ack': this._handleAck(msg); break;
      case 'err': this._handleError(msg); break;
    }
  },

  // ==================== PROTOCOL ====================

  async listDrives(panel) {
    this._pendingPanel = panel || this._activePanel;
    this._send({ op: 'drives' });
  },

  async listDirectory(path, panel) {
    this._pendingPanel = panel || this._activePanel;
    this._send({ op: 'list', path: path || '' });
  },

  _handleList(msg) {
    const panel = this._pendingPanel || this._activePanel;
    const p = this._panels[panel];
    p.path = msg.path || '';
    p.entries = msg.entries || [];
    p.selected.clear();
    p.focusIdx = 0;
    this._renderPanel(panel);
  },

  _handleDrives(msg) {
    const panel = this._pendingPanel || this._activePanel;
    const p = this._panels[panel];
    p.path = '';
    p.entries = msg.entries || [];
    p.selected.clear();
    p.focusIdx = 0;
    this._renderPanel(panel);
  },

  // ==================== DOWNLOAD ====================

  downloadFile(path, filename, size) {
    const fid = this._nextFid++;
    this._pendingCallbacks[fid] = {
      type: 'download', filename, size: size || 0,
      chunks: [], totalChunks: 0, receivedChunks: 0, data: []
    };
    this._send({ op: 'get', path, fid });
    this._updateProgress(fid, 0, 'Downloader ' + filename + '...');
  },

  downloadSelected() {
    const p = this._panels[this._activePanel];
    const indices = p.selected.size > 0 ? [...p.selected] : [p.focusIdx];
    const allEntries = this._getDisplayEntries(p);
    for (const idx of indices) {
      const entry = allEntries[idx];
      if (entry && !entry.dir && !entry._upnav) {
        this.downloadFile(entry.path, entry.name, entry.size);
      }
    }
  },

  _handleDownloadChunk(msg) {
    const pending = this._pendingCallbacks[msg.fid];
    if (!pending || pending.type !== 'download') return;
    pending.totalChunks = msg.t || 1;
    pending.receivedChunks++;
    if (msg.data) {
      let bytes;
      if (typeof msg.data === 'string') {
        const binary = atob(msg.data);
        bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
      } else if (Array.isArray(msg.data)) bytes = new Uint8Array(msg.data);
      if (bytes) pending.data.push(bytes);
    }
    const pct = Math.round((pending.receivedChunks / pending.totalChunks) * 100);
    this._updateProgress(msg.fid, pct, `Downloader ${pending.filename}... ${pct}%`);
    if (pending.receivedChunks >= pending.totalChunks) {
      const totalLen = pending.data.reduce((s, c) => s + c.length, 0);
      const combined = new Uint8Array(totalLen);
      let offset = 0;
      for (const chunk of pending.data) { combined.set(chunk, offset); offset += chunk.length; }
      const blob = new Blob([combined]);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url; a.download = pending.filename || 'download';
      document.body.appendChild(a); a.click(); document.body.removeChild(a);
      URL.revokeObjectURL(url);
      delete this._pendingCallbacks[msg.fid];
      this._updateProgress(msg.fid, 100, 'Download fuldført: ' + pending.filename);
      setTimeout(() => this._hideProgress(), 2000);
    }
  },

  // ==================== UPLOAD ====================

  async uploadFile(file, remotePath) {
    const fid = this._nextFid++;
    const CHUNK_SIZE = 60000;
    const totalChunks = Math.ceil(file.size / CHUNK_SIZE);
    this._pendingCallbacks[fid] = { type: 'upload', filename: file.name, size: file.size, totalChunks };
    const buffer = await file.arrayBuffer();
    const data = new Uint8Array(buffer);
    for (let c = 0; c < totalChunks; c++) {
      const start = c * CHUNK_SIZE;
      const end = Math.min(start + CHUNK_SIZE, file.size);
      const chunk = data.slice(start, end);
      let binary = '';
      for (let i = 0; i < chunk.length; i++) binary += String.fromCharCode(chunk[i]);
      this._send({ op: 'put', path: remotePath, fid, c, t: totalChunks, size: file.size, data: btoa(binary) });
      const pct = Math.round(((c + 1) / totalChunks) * 100);
      this._updateProgress(fid, pct, `Uploader ${file.name}... ${pct}%`);
      if (c % 10 === 9) await new Promise(r => setTimeout(r, 5));
    }
  },

  _handleAck(msg) {
    const pending = this._pendingCallbacks[msg.fid];
    if (!pending) return;
    if (pending.type === 'upload' && !msg.c) {
      delete this._pendingCallbacks[msg.fid];
      this._updateProgress(msg.fid, 100, 'Upload fuldført: ' + pending.filename);
      setTimeout(() => { this._hideProgress(); this.refreshPanel(); }, 1500);
    }
  },

  _handleError(msg) {
    console.error('File transfer error:', msg.error);
    if (typeof showToast === 'function') showToast('Fil fejl: ' + msg.error, 'error');
    this._hideProgress();
  },

  // ==================== OPERATIONS ====================

  createDirectory(path) {
    this._send({ op: 'mkdir', path });
    setTimeout(() => this.refreshPanel(), 500);
  },

  createDirectoryPrompt() {
    const name = prompt('Mappenavn:');
    if (!name) return;
    const p = this._panels[this._activePanel];
    const sep = p.path.includes('/') ? '/' : '\\';
    this.createDirectory(p.path + sep + name);
  },

  deleteItem(path) {
    this._send({ op: 'rm', path });
    setTimeout(() => this.refreshPanel(), 500);
  },

  deleteSelected() {
    const p = this._panels[this._activePanel];
    const indices = p.selected.size > 0 ? [...p.selected] : [p.focusIdx];
    const allEntries = this._getDisplayEntries(p);
    const names = [];
    for (const idx of indices) {
      const entry = allEntries[idx];
      if (entry && !entry._upnav) names.push(entry.name);
    }
    if (names.length === 0) return;
    if (!confirm('Slet ' + names.length + ' element(er)?\n\n' + names.join('\n'))) return;
    for (const idx of indices) {
      const entry = allEntries[idx];
      if (entry && !entry._upnav) this._send({ op: 'rm', path: entry.path });
    }
    setTimeout(() => this.refreshPanel(), 500);
  },

  viewFile() {
    const p = this._panels[this._activePanel];
    const allEntries = this._getDisplayEntries(p);
    const entry = allEntries[p.focusIdx];
    if (entry && !entry.dir && !entry._upnav) {
      this.downloadFile(entry.path, entry.name, entry.size);
    }
  },

  refreshPanel() {
    const p = this._panels[this._activePanel];
    if (p.path) this.listDirectory(p.path, this._activePanel);
    else this.listDrives(this._activePanel);
  },

  // ==================== UI ====================

  open() {
    const modal = document.getElementById('fileTransferModal');
    if (!modal) return;
    this._isOpen = true;
    modal.style.display = 'flex';

    if (!this.isReady()) {
      if (typeof showToast === 'function') showToast('Filkanal ikke tilgængelig.', 'error');
      return;
    }

    // Setup upload input
    const uploadInput = document.getElementById('fileUploadInput');
    if (uploadInput && !uploadInput._wired) {
      uploadInput._wired = true;
      uploadInput.addEventListener('change', async (e) => {
        if (!this.isReady() || !e.target.files.length) return;
        const p = this._panels[this._activePanel];
        const sep = p.path.includes('/') ? '/' : '\\';
        for (const file of e.target.files) {
          await this.uploadFile(file, p.path + sep + file.name);
        }
        e.target.value = '';
      });
    }

    // Setup keyboard handler
    if (!this._keyHandler) {
      this._keyHandler = (e) => this._handleKeyboard(e);
      modal.addEventListener('keydown', this._keyHandler);
    }

    // Setup panel click activation
    for (const side of ['left', 'right']) {
      const panel = document.getElementById(side === 'left' ? 'tcPanelLeft' : 'tcPanelRight');
      if (panel && !panel._wired) {
        panel._wired = true;
        panel.addEventListener('click', () => this._setActivePanel(side));
      }
    }

    // Load both panels
    this.listDrives('left');
    setTimeout(() => this.listDrives('right'), 100);
    this._setActivePanel('left');
  },

  close() {
    const modal = document.getElementById('fileTransferModal');
    if (modal) modal.style.display = 'none';
    this._isOpen = false;
  },

  _setActivePanel(side) {
    this._activePanel = side;
    const left = document.getElementById('tcPanelLeft');
    const right = document.getElementById('tcPanelRight');
    if (left) left.classList.toggle('active', side === 'left');
    if (right) right.classList.toggle('active', side === 'right');
    // Focus the file list
    const list = document.getElementById(side === 'left' ? 'tcFileListLeft' : 'tcFileListRight');
    if (list) list.focus();
  },

  _getDisplayEntries(p) {
    const entries = [];
    // Up-nav entry
    if (p.path) {
      const sep = p.path.includes('/') ? '/' : '\\';
      const parts = p.path.split(/[/\\]/).filter(Boolean);
      let parentPath = '';
      if (parts.length > 1) {
        parts.pop();
        parentPath = parts.join(sep) + (parts.length === 1 && parts[0].endsWith(':') ? sep : '');
      }
      entries.push({ name: '[..]', path: parentPath, dir: true, _upnav: true });
    }

    // Sort entries: dirs first, then by sortCol
    const sorted = [...p.entries].sort((a, b) => {
      if (a.dir !== b.dir) return a.dir ? -1 : 1;
      let va, vb;
      switch (p.sortCol) {
        case 'ext':
          va = (a.name.includes('.') ? a.name.split('.').pop() : '').toLowerCase();
          vb = (b.name.includes('.') ? b.name.split('.').pop() : '').toLowerCase();
          break;
        case 'size': va = a.size || 0; vb = b.size || 0; break;
        case 'date': va = a.modified || ''; vb = b.modified || ''; break;
        default: va = a.name.toLowerCase(); vb = b.name.toLowerCase();
      }
      if (va < vb) return p.sortAsc ? -1 : 1;
      if (va > vb) return p.sortAsc ? 1 : -1;
      return 0;
    });

    entries.push(...sorted);
    return entries;
  },

  _renderPanel(side) {
    const p = this._panels[side];
    const listEl = document.getElementById(side === 'left' ? 'tcFileListLeft' : 'tcFileListRight');
    const pathEl = document.getElementById(side === 'left' ? 'tcPathLeft' : 'tcPathRight');
    const statusEl = document.getElementById(side === 'left' ? 'tcStatusLeft' : 'tcStatusRight');
    if (!listEl) return;

    // Path bar with drive buttons
    if (pathEl) {
      pathEl.innerHTML = '';
      const homeBtn = document.createElement('span');
      homeBtn.className = 'drive-btn';
      homeBtn.textContent = '\u{1F4BB}';
      homeBtn.title = 'Vis drev';
      homeBtn.addEventListener('click', () => this.listDrives(side));
      pathEl.appendChild(homeBtn);

      if (p.path) {
        const pathText = document.createElement('span');
        pathText.textContent = p.path;
        pathText.style.cssText = 'overflow:hidden; text-overflow:ellipsis; flex:1;';
        pathEl.appendChild(pathText);
      } else {
        const pathText = document.createElement('span');
        pathText.textContent = 'Drev';
        pathText.style.color = '#8b949e';
        pathEl.appendChild(pathText);
      }
    }

    // File list
    const allEntries = this._getDisplayEntries(p);
    listEl.innerHTML = '';

    for (let i = 0; i < allEntries.length; i++) {
      const entry = allEntries[i];
      const row = document.createElement('div');
      row.className = 'tc-row' + (entry.dir ? ' dir' : '') + (entry._upnav ? ' upnav' : '');
      if (p.selected.has(i)) row.classList.add('selected');
      if (i === p.focusIdx) row.classList.add('focused');

      // Name column
      const nameCol = document.createElement('span');
      nameCol.className = 'tc-col-name';
      const icon = document.createElement('span');
      icon.className = 'tc-icon';
      if (entry._upnav) icon.innerHTML = '<i class="fas fa-level-up-alt"></i>';
      else if (entry.dir) icon.innerHTML = '<i class="fas fa-folder"></i>';
      else icon.innerHTML = '<i class="fas fa-file"></i>';
      const fname = document.createElement('span');
      fname.className = 'tc-fname';
      fname.textContent = entry._upnav ? '..' : entry.name;
      nameCol.append(icon, fname);

      // Extension
      const extCol = document.createElement('span');
      extCol.className = 'tc-col-ext';
      if (!entry.dir && !entry._upnav && entry.name.includes('.')) {
        extCol.textContent = entry.name.split('.').pop().toUpperCase();
      }

      // Size
      const sizeCol = document.createElement('span');
      sizeCol.className = 'tc-col-size';
      if (entry.dir && !entry._upnav) sizeCol.textContent = '<DIR>';
      else if (!entry._upnav) sizeCol.textContent = this._formatSize(entry.size);

      // Date
      const dateCol = document.createElement('span');
      dateCol.className = 'tc-col-date';
      if (!entry._upnav && entry.modified) {
        dateCol.textContent = entry.modified.substring(0, 16).replace('T', ' ');
      }

      row.append(nameCol, extCol, sizeCol, dateCol);

      // Click handlers
      row.addEventListener('click', (e) => {
        this._setActivePanel(side);
        if (e.ctrlKey) {
          // Toggle selection
          if (p.selected.has(i)) p.selected.delete(i);
          else p.selected.add(i);
          p.focusIdx = i;
        } else if (e.shiftKey) {
          // Range selection
          const start = Math.min(p.focusIdx, i);
          const end = Math.max(p.focusIdx, i);
          for (let j = start; j <= end; j++) p.selected.add(j);
        } else {
          p.selected.clear();
          p.focusIdx = i;
        }
        this._renderPanel(side);
      });

      row.addEventListener('dblclick', () => {
        this._setActivePanel(side);
        if (entry.dir) {
          if (entry._upnav && !entry.path) this.listDrives(side);
          else this.listDirectory(entry.path, side);
        } else {
          this.downloadFile(entry.path, entry.name, entry.size);
        }
      });

      listEl.appendChild(row);
    }

    // Status bar
    if (statusEl) {
      const fileCount = allEntries.filter(e => !e.dir && !e._upnav).length;
      const dirCount = allEntries.filter(e => e.dir && !e._upnav).length;
      const selCount = p.selected.size;
      let totalSize = 0;
      for (const idx of p.selected) {
        const e = allEntries[idx];
        if (e && !e.dir) totalSize += e.size || 0;
      }
      let status = `${fileCount} filer, ${dirCount} mapper`;
      if (selCount > 0) status += ` | Valgt: ${selCount} (${this._formatSize(totalSize)})`;
      statusEl.textContent = status;
    }

    // Sort header highlighting
    const panelEl = document.getElementById(side === 'left' ? 'tcPanelLeft' : 'tcPanelRight');
    if (panelEl) {
      panelEl.querySelectorAll('.tc-header span').forEach(span => {
        const col = span.dataset.sort;
        span.style.color = col === p.sortCol ? '#58a6ff' : '';
        span.textContent = span.textContent.replace(/ [▲▼]$/, '');
        if (col === p.sortCol) span.textContent += p.sortAsc ? ' ▲' : ' ▼';
        if (!span._wired) {
          span._wired = true;
          span.addEventListener('click', () => {
            if (p.sortCol === col) p.sortAsc = !p.sortAsc;
            else { p.sortCol = col; p.sortAsc = true; }
            this._renderPanel(side);
          });
        }
      });
    }
  },

  _handleKeyboard(e) {
    if (!this._isOpen) return;
    const p = this._panels[this._activePanel];
    const allEntries = this._getDisplayEntries(p);
    const side = this._activePanel;

    switch (e.key) {
      case 'Tab':
        e.preventDefault();
        this._setActivePanel(this._activePanel === 'left' ? 'right' : 'left');
        break;
      case 'ArrowUp':
        e.preventDefault();
        if (p.focusIdx > 0) { p.focusIdx--; if (!e.shiftKey) p.selected.clear(); this._renderPanel(side); }
        break;
      case 'ArrowDown':
        e.preventDefault();
        if (p.focusIdx < allEntries.length - 1) { p.focusIdx++; if (!e.shiftKey) p.selected.clear(); this._renderPanel(side); }
        break;
      case 'Enter': {
        const entry = allEntries[p.focusIdx];
        if (entry) {
          if (entry.dir) {
            if (entry._upnav && !entry.path) this.listDrives(side);
            else this.listDirectory(entry.path, side);
          } else {
            this.downloadFile(entry.path, entry.name, entry.size);
          }
        }
        break;
      }
      case 'Insert':
        e.preventDefault();
        if (p.selected.has(p.focusIdx)) p.selected.delete(p.focusIdx);
        else p.selected.add(p.focusIdx);
        if (p.focusIdx < allEntries.length - 1) p.focusIdx++;
        this._renderPanel(side);
        break;
      case 'Backspace':
        e.preventDefault();
        if (allEntries[0] && allEntries[0]._upnav) {
          if (!allEntries[0].path) this.listDrives(side);
          else this.listDirectory(allEntries[0].path, side);
        }
        break;
      case 'F3': e.preventDefault(); this.viewFile(); break;
      case 'F5': e.preventDefault(); document.getElementById('fileUploadInput')?.click(); break;
      case 'F6': e.preventDefault(); this.downloadSelected(); break;
      case 'F7': e.preventDefault(); this.createDirectoryPrompt(); break;
      case 'F8': case 'Delete': e.preventDefault(); this.deleteSelected(); break;
      case 'Escape': this.close(); break;
    }
  },

  _formatSize(bytes) {
    if (!bytes || bytes === 0) return '';
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1048576) return (bytes / 1024).toFixed(0) + ' KB';
    if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + ' MB';
    return (bytes / 1073741824).toFixed(2) + ' GB';
  },

  _updateProgress(fid, percent, label) {
    const container = document.getElementById('fileProgressContainer');
    const bar = document.getElementById('fileProgressBar');
    const labelEl = document.getElementById('fileProgressLabel');
    if (!container) return;
    container.style.display = '';
    if (bar) bar.style.width = Math.round(percent) + '%';
    if (labelEl) labelEl.textContent = label || '';
    if (percent >= 100) setTimeout(() => { container.style.display = 'none'; }, 2000);
  },

  _hideProgress() {
    const container = document.getElementById('fileProgressContainer');
    if (container) container.style.display = 'none';
  }
};

window.FileTransfer = FileTransfer;
