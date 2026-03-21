// File Transfer Module for Controller
// Port of dashboard's file-transfer.js — browse, upload, download over WebRTC file channel
// Protocol matches agent/internal/filetransfer/handler.go

const FileTransfer = {
  _channel: null,
  _pendingCallbacks: {},
  _nextFid: 1,
  _currentPath: '',
  _isOpen: false,

  setChannel(dc) {
    this._channel = dc;
    if (dc) {
      dc.onmessage = (event) => this._handleMessage(event);
    }
  },

  isReady() {
    return this._channel && this._channel.readyState === 'open';
  },

  _send(obj) {
    if (!this.isReady()) {
      console.warn('File channel not ready');
      return false;
    }
    this._channel.send(JSON.stringify(obj));
    return true;
  },

  _handleMessage(event) {
    let msg;
    if (typeof event.data === 'string') {
      try { msg = JSON.parse(event.data); } catch (e) { return; }
    } else if (event.data instanceof ArrayBuffer) {
      try { msg = JSON.parse(new TextDecoder().decode(event.data)); } catch (e) { return; }
    } else {
      return;
    }

    const op = msg.op;
    if (!op) return;

    switch (op) {
      case 'list': this._handleList(msg); break;
      case 'drives': this._handleDrives(msg); break;
      case 'put': this._handleDownloadChunk(msg); break;
      case 'ack': this._handleAck(msg); break;
      case 'err': this._handleError(msg); break;
    }
  },

  // ==================== DIRECTORY LISTING ====================

  async listDrives() { this._send({ op: 'drives' }); },

  async listDirectory(path) { this._send({ op: 'list', path: path || '' }); },

  _handleList(msg) {
    this._currentPath = msg.path || '';
    this._renderDirectoryListing(msg.entries || [], msg.path);
  },

  _handleDrives(msg) {
    this._currentPath = '';
    this._renderDirectoryListing(msg.entries || [], '');
  },

  // ==================== DOWNLOAD ====================

  downloadFile(path, filename, size) {
    const fid = this._nextFid++;
    this._pendingCallbacks[fid] = {
      type: 'download', filename, size: size || 0,
      chunks: [], totalChunks: 0, receivedChunks: 0, data: []
    };
    this._send({ op: 'get', path, fid });
    this._updateProgress(fid, 0, 'Downloader...');
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
      } else if (Array.isArray(msg.data)) {
        bytes = new Uint8Array(msg.data);
      }
      if (bytes) pending.data.push(bytes);
    }

    const pct = Math.round((pending.receivedChunks / pending.totalChunks) * 100);
    this._updateProgress(msg.fid, pct, `Downloader... ${pct}%`);

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
      this._updateProgress(msg.fid, 100, 'Download fuldført!');
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
      const b64 = btoa(binary);

      this._send({ op: 'put', path: remotePath, fid, c, t: totalChunks, size: file.size, data: b64 });

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
      this._updateProgress(msg.fid, 100, 'Upload fuldført!');
      setTimeout(() => { this._hideProgress(); if (this._currentPath) this.listDirectory(this._currentPath); }, 1500);
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
    setTimeout(() => this.listDirectory(this._currentPath), 500);
  },

  createDirectoryPrompt() {
    const name = prompt('Mappenavn:');
    if (!name) return;
    const sep = this._currentPath.includes('/') ? '/' : '\\';
    this.createDirectory(this._currentPath + sep + name);
  },

  deleteItem(path) {
    if (!confirm('Slet ' + path + '?')) return;
    this._send({ op: 'rm', path });
    setTimeout(() => this.listDirectory(this._currentPath), 500);
  },

  // ==================== UI ====================

  open() {
    const modal = document.getElementById('fileTransferModal');
    if (!modal) return;
    this._isOpen = true;
    modal.style.display = 'flex';

    if (!this.isReady()) {
      if (typeof showToast === 'function') showToast('Filkanal ikke tilgængelig. Sørg for at være forbundet.', 'error');
      return;
    }

    // Setup upload input
    const uploadInput = document.getElementById('fileUploadInput');
    if (uploadInput && !uploadInput._wired) {
      uploadInput._wired = true;
      uploadInput.addEventListener('change', async (e) => {
        if (!this.isReady() || !e.target.files.length) return;
        const sep = this._currentPath.includes('/') ? '/' : '\\';
        for (const file of e.target.files) {
          await this.uploadFile(file, this._currentPath + sep + file.name);
        }
        e.target.value = '';
      });
    }

    this._setupDragDrop();
    this.listDrives();
  },

  _setupDragDrop() {
    const zone = document.getElementById('fileDropZone');
    if (!zone || zone._dragSetup) return;
    zone._dragSetup = true;

    zone.addEventListener('dragover', (e) => {
      e.preventDefault();
      e.stopPropagation();
      zone.classList.add('drag-over');
    });
    zone.addEventListener('dragleave', (e) => {
      e.preventDefault();
      zone.classList.remove('drag-over');
    });
    zone.addEventListener('drop', (e) => {
      e.preventDefault();
      zone.classList.remove('drag-over');
      const files = e.dataTransfer.files;
      if (files.length > 0) {
        const sep = this._currentPath.includes('/') ? '/' : '\\';
        for (const file of files) {
          this.uploadFile(file, this._currentPath + sep + file.name);
        }
      }
    });
  },

  close() {
    const modal = document.getElementById('fileTransferModal');
    if (modal) modal.style.display = 'none';
    this._isOpen = false;
  },

  _renderDirectoryListing(entries, path) {
    const list = document.getElementById('fileList');
    const breadcrumb = document.getElementById('fileBreadcrumb');
    if (!list) return;

    list.innerHTML = '';

    // Breadcrumb
    if (breadcrumb) {
      breadcrumb.innerHTML = '';
      const homeBtn = document.createElement('span');
      homeBtn.innerHTML = '<i class="fas fa-hdd"></i> Drev';
      homeBtn.style.cssText = 'cursor:pointer; color:var(--primary);';
      homeBtn.addEventListener('click', () => this.listDrives());
      breadcrumb.appendChild(homeBtn);

      if (path) {
        const sep = path.includes('/') ? '/' : '\\';
        const parts = path.split(/[/\\]/).filter(Boolean);
        let accumulated = '';
        for (const part of parts) {
          accumulated += (accumulated && !accumulated.endsWith(sep) ? sep : '') + part;
          const arrow = document.createElement('span');
          arrow.textContent = ' › ';
          arrow.style.color = 'var(--text-muted)';
          breadcrumb.appendChild(arrow);

          const partBtn = document.createElement('span');
          partBtn.textContent = part;
          partBtn.style.cssText = 'cursor:pointer; color:var(--primary);';
          const navPath = accumulated + (part.endsWith(':') ? sep : '');
          partBtn.addEventListener('click', () => this.listDirectory(navPath));
          breadcrumb.appendChild(partBtn);
        }
      }
    }

    // "Go up" entry
    if (path) {
      const sep = path.includes('/') ? '/' : '\\';
      const parts = path.split(/[/\\]/).filter(Boolean);
      if (parts.length > 1) {
        parts.pop();
        const parentPath = parts.join(sep) + (parts.length === 1 && parts[0].endsWith(':') ? sep : '');
        list.appendChild(this._createFileRow({ name: '..', path: parentPath, dir: true }, true));
      } else {
        const upRow = this._createFileRow({ name: '..', path: '', dir: true }, true);
        upRow.addEventListener('click', () => this.listDrives());
        list.appendChild(upRow);
      }
    }

    for (const entry of entries) {
      list.appendChild(this._createFileRow(entry, false));
    }

    if (entries.length === 0 && !path) {
      const empty = document.createElement('div');
      empty.style.cssText = 'text-align:center; padding:2rem; color:var(--text-muted);';
      empty.textContent = 'Ingen drev fundet';
      list.appendChild(empty);
    }
  },

  _createFileRow(entry, isUpNav) {
    const row = document.createElement('div');
    row.style.cssText = 'display:flex; align-items:center; gap:0.5rem; padding:0.45rem 0.75rem; border-bottom:1px solid var(--border); cursor:pointer; font-size:0.85rem; transition:background 0.15s;';
    row.addEventListener('mouseenter', () => row.style.background = 'rgba(255,255,255,0.05)');
    row.addEventListener('mouseleave', () => row.style.background = '');

    const icon = document.createElement('span');
    icon.style.cssText = 'flex-shrink:0; width:1.5em; text-align:center;';
    if (isUpNav) { icon.innerHTML = '<i class="fas fa-level-up-alt"></i>'; }
    else if (entry.dir) { icon.innerHTML = '<i class="fas fa-folder" style="color:#f59e0b;"></i>'; }
    else { icon.innerHTML = '<i class="fas fa-file" style="color:var(--text-muted);"></i>'; }

    const name = document.createElement('span');
    name.style.cssText = 'flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;';
    name.textContent = entry.name;

    row.append(icon, name);

    if (!isUpNav && !entry.dir) {
      const size = document.createElement('span');
      size.style.cssText = 'color:var(--text-muted); font-size:0.75rem; flex-shrink:0;';
      size.textContent = this._formatSize(entry.size);
      row.appendChild(size);

      const dlBtn = document.createElement('button');
      dlBtn.className = 'btn btn-sm btn-icon';
      dlBtn.style.cssText = 'font-size:0.75rem; padding:0.1rem 0.3rem;';
      dlBtn.innerHTML = '<i class="fas fa-download"></i>';
      dlBtn.title = 'Download';
      dlBtn.addEventListener('click', (e) => { e.stopPropagation(); this.downloadFile(entry.path, entry.name, entry.size); });
      row.appendChild(dlBtn);
    }

    if (!isUpNav && entry.path) {
      const delBtn = document.createElement('button');
      delBtn.className = 'btn btn-sm btn-icon';
      delBtn.style.cssText = 'font-size:0.75rem; padding:0.1rem 0.3rem; color:var(--danger);';
      delBtn.innerHTML = '<i class="fas fa-trash"></i>';
      delBtn.title = 'Slet';
      delBtn.addEventListener('click', (e) => { e.stopPropagation(); this.deleteItem(entry.path); });
      row.appendChild(delBtn);
    }

    if (entry.dir) {
      row.addEventListener('click', () => {
        if (isUpNav && !entry.path) this.listDrives();
        else this.listDirectory(entry.path);
      });
    }

    return row;
  },

  _formatSize(bytes) {
    if (!bytes || bytes === 0) return '';
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB';
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
    if (percent >= 100) {
      setTimeout(() => { container.style.display = 'none'; }, 2000);
    }
  },

  _hideProgress() {
    const container = document.getElementById('fileProgressContainer');
    if (container) container.style.display = 'none';
  }
};

window.FileTransfer = FileTransfer;
