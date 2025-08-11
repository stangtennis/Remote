// File Transfer Module for WebRTC Remote Desktop
console.log('📁 Loading File Transfer Module...');

// File transfer state
let isTransferActive = false;
let currentTransfers = new Map();
let dragCounter = 0;

// Event handlers
let onFileUpload = null;
let onFileDownload = null;
let onTransferProgress = null;
let onTransferComplete = null;
let onTransferError = null;

// Initialize file transfer functionality
function initializeFileTransfer(deviceId) {
    console.log(`📁 Initializing file transfer for device: ${deviceId}`);
    
    // Set up drag and drop
    setupDragAndDrop();
    
    // Set up file input handlers
    setupFileInputHandlers();
    
    // Set up transfer UI
    setupTransferUI();
    
    console.log('✅ File transfer initialized');
}

// Set up drag and drop functionality
function setupDragAndDrop() {
    console.log('🔧 Setting up drag and drop...');
    
    const dropZone = document.getElementById('fileDropZone') || createDropZone();
    
    // Prevent default drag behaviors
    ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
        dropZone.addEventListener(eventName, preventDefaults, false);
        document.body.addEventListener(eventName, preventDefaults, false);
    });
    
    // Highlight drop zone when item is dragged over it
    ['dragenter', 'dragover'].forEach(eventName => {
        dropZone.addEventListener(eventName, highlight, false);
    });
    
    ['dragleave', 'drop'].forEach(eventName => {
        dropZone.addEventListener(eventName, unhighlight, false);
    });
    
    // Handle dropped files
    dropZone.addEventListener('drop', handleDrop, false);
    
    console.log('✅ Drag and drop set up');
}

// Create drop zone if it doesn't exist
function createDropZone() {
    console.log('🎯 Creating file drop zone...');
    
    const dropZone = document.createElement('div');
    dropZone.id = 'fileDropZone';
    dropZone.className = 'file-drop-zone';
    dropZone.innerHTML = `
        <div class="drop-zone-content">
            <i class="fas fa-cloud-upload-alt"></i>
            <p>Drag and drop files here or <button id="selectFilesBtn" class="btn-link">click to select</button></p>
            <p class="drop-zone-hint">Supported formats: All file types</p>
        </div>
    `;
    
    // Add styles
    const style = document.createElement('style');
    style.textContent = `
        .file-drop-zone {
            border: 2px dashed #ccc;
            border-radius: 8px;
            padding: 40px;
            text-align: center;
            margin: 20px 0;
            background: #f9f9f9;
            transition: all 0.3s ease;
            cursor: pointer;
        }
        
        .file-drop-zone.drag-over {
            border-color: #007bff;
            background: #e3f2fd;
            transform: scale(1.02);
        }
        
        .drop-zone-content i {
            font-size: 48px;
            color: #ccc;
            margin-bottom: 16px;
        }
        
        .file-drop-zone.drag-over .drop-zone-content i {
            color: #007bff;
        }
        
        .btn-link {
            background: none;
            border: none;
            color: #007bff;
            text-decoration: underline;
            cursor: pointer;
            font-size: inherit;
        }
        
        .drop-zone-hint {
            font-size: 12px;
            color: #666;
            margin-top: 8px;
        }
        
        .transfer-progress {
            margin: 10px 0;
            padding: 10px;
            border: 1px solid #ddd;
            border-radius: 4px;
            background: #f8f9fa;
        }
        
        .progress-bar {
            width: 100%;
            height: 20px;
            background: #e9ecef;
            border-radius: 10px;
            overflow: hidden;
            margin: 8px 0;
        }
        
        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, #007bff, #0056b3);
            transition: width 0.3s ease;
            border-radius: 10px;
        }
        
        .transfer-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 8px 0;
            border-bottom: 1px solid #eee;
        }
        
        .transfer-item:last-child {
            border-bottom: none;
        }
        
        .transfer-info {
            flex: 1;
        }
        
        .transfer-status {
            font-size: 12px;
            color: #666;
        }
        
        .transfer-actions {
            display: flex;
            gap: 8px;
        }
        
        .btn-sm {
            padding: 4px 8px;
            font-size: 12px;
            border-radius: 4px;
            border: 1px solid #ddd;
            background: white;
            cursor: pointer;
        }
        
        .btn-sm:hover {
            background: #f8f9fa;
        }
        
        .btn-danger {
            border-color: #dc3545;
            color: #dc3545;
        }
        
        .btn-danger:hover {
            background: #f5c6cb;
        }
    `;
    
    document.head.appendChild(style);
    
    // Find a good place to insert the drop zone
    const container = document.getElementById('fileTransferContainer') || 
                     document.querySelector('.controls') || 
                     document.body;
    
    container.appendChild(dropZone);
    
    return dropZone;
}

// Set up file input handlers
function setupFileInputHandlers() {
    console.log('🔧 Setting up file input handlers...');
    
    // Create hidden file input
    const fileInput = document.createElement('input');
    fileInput.type = 'file';
    fileInput.id = 'fileInput';
    fileInput.multiple = true;
    fileInput.style.display = 'none';
    document.body.appendChild(fileInput);
    
    // Handle file selection
    fileInput.addEventListener('change', (event) => {
        handleFiles(event.target.files);
    });
    
    // Connect select button to file input
    document.addEventListener('click', (event) => {
        if (event.target.id === 'selectFilesBtn') {
            fileInput.click();
        }
    });
    
    console.log('✅ File input handlers set up');
}

// Set up transfer UI
function setupTransferUI() {
    console.log('🔧 Setting up transfer UI...');
    
    // Create transfer list container
    let transferList = document.getElementById('transferList');
    if (!transferList) {
        transferList = document.createElement('div');
        transferList.id = 'transferList';
        transferList.className = 'transfer-list';
        
        const container = document.getElementById('fileTransferContainer') || 
                         document.querySelector('.controls') || 
                         document.body;
        
        container.appendChild(transferList);
    }
    
    console.log('✅ Transfer UI set up');
}

// Prevent default drag behaviors
function preventDefaults(e) {
    e.preventDefault();
    e.stopPropagation();
}

// Highlight drop zone
function highlight(e) {
    dragCounter++;
    const dropZone = document.getElementById('fileDropZone');
    if (dropZone) {
        dropZone.classList.add('drag-over');
    }
}

// Unhighlight drop zone
function unhighlight(e) {
    dragCounter--;
    if (dragCounter === 0) {
        const dropZone = document.getElementById('fileDropZone');
        if (dropZone) {
            dropZone.classList.remove('drag-over');
        }
    }
}

// Handle dropped files
function handleDrop(e) {
    const dt = e.dataTransfer;
    const files = dt.files;
    handleFiles(files);
}

// Handle selected files
function handleFiles(files) {
    console.log(`📁 Handling ${files.length} files`);
    
    Array.from(files).forEach(file => {
        uploadFile(file);
    });
}

// Upload file
async function uploadFile(file) {
    const transferId = `upload_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    
    console.log(`📤 Starting upload: ${file.name} (${formatFileSize(file.size)})`);
    
    // Add to current transfers
    currentTransfers.set(transferId, {
        id: transferId,
        type: 'upload',
        fileName: file.name,
        fileSize: file.size,
        progress: 0,
        status: 'starting',
        startTime: Date.now()
    });
    
    // Update UI
    updateTransferUI();
    
    try {
        // Call upload handler if available
        if (onFileUpload) {
            await onFileUpload(file, (progress) => {
                updateTransferProgress(transferId, progress);
            });
        } else {
            // Simulate upload for demo
            await simulateTransfer(transferId);
        }
        
        // Mark as completed
        const transfer = currentTransfers.get(transferId);
        if (transfer) {
            transfer.status = 'completed';
            transfer.progress = 100;
            transfer.endTime = Date.now();
            updateTransferUI();
            
            if (onTransferComplete) {
                onTransferComplete(transfer);
            }
        }
        
        console.log(`✅ Upload completed: ${file.name}`);
        
    } catch (error) {
        console.error(`❌ Upload failed: ${file.name}`, error);
        
        const transfer = currentTransfers.get(transferId);
        if (transfer) {
            transfer.status = 'error';
            transfer.error = error.message;
            updateTransferUI();
            
            if (onTransferError) {
                onTransferError(transfer, error);
            }
        }
    }
}

// Download file
async function downloadFile(fileName, fileSize = null) {
    const transferId = `download_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    
    console.log(`📥 Starting download: ${fileName}`);
    
    // Add to current transfers
    currentTransfers.set(transferId, {
        id: transferId,
        type: 'download',
        fileName: fileName,
        fileSize: fileSize,
        progress: 0,
        status: 'starting',
        startTime: Date.now()
    });
    
    // Update UI
    updateTransferUI();
    
    try {
        // Call download handler if available
        if (onFileDownload) {
            const blob = await onFileDownload(fileName, (progress) => {
                updateTransferProgress(transferId, progress);
            });
            
            // Trigger browser download
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = fileName;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
        } else {
            // Simulate download for demo
            await simulateTransfer(transferId);
        }
        
        // Mark as completed
        const transfer = currentTransfers.get(transferId);
        if (transfer) {
            transfer.status = 'completed';
            transfer.progress = 100;
            transfer.endTime = Date.now();
            updateTransferUI();
            
            if (onTransferComplete) {
                onTransferComplete(transfer);
            }
        }
        
        console.log(`✅ Download completed: ${fileName}`);
        
    } catch (error) {
        console.error(`❌ Download failed: ${fileName}`, error);
        
        const transfer = currentTransfers.get(transferId);
        if (transfer) {
            transfer.status = 'error';
            transfer.error = error.message;
            updateTransferUI();
            
            if (onTransferError) {
                onTransferError(transfer, error);
            }
        }
    }
}

// Update transfer progress
function updateTransferProgress(transferId, progress) {
    const transfer = currentTransfers.get(transferId);
    if (transfer) {
        transfer.progress = Math.min(100, Math.max(0, progress));
        transfer.status = 'transferring';
        updateTransferUI();
        
        if (onTransferProgress) {
            onTransferProgress(transfer);
        }
    }
}

// Update transfer UI
function updateTransferUI() {
    const transferList = document.getElementById('transferList');
    if (!transferList) return;
    
    transferList.innerHTML = '';
    
    if (currentTransfers.size === 0) {
        transferList.style.display = 'none';
        return;
    }
    
    transferList.style.display = 'block';
    
    // Add header
    const header = document.createElement('h4');
    header.textContent = 'File Transfers';
    header.style.marginBottom = '16px';
    transferList.appendChild(header);
    
    // Add transfers
    currentTransfers.forEach(transfer => {
        const transferItem = createTransferItem(transfer);
        transferList.appendChild(transferItem);
    });
}

// Create transfer item UI
function createTransferItem(transfer) {
    const item = document.createElement('div');
    item.className = 'transfer-item';
    
    const statusIcon = getStatusIcon(transfer.status);
    const progressBar = transfer.status === 'transferring' ? 
        `<div class="progress-bar">
            <div class="progress-fill" style="width: ${transfer.progress}%"></div>
         </div>` : '';
    
    const duration = transfer.endTime ? 
        `${((transfer.endTime - transfer.startTime) / 1000).toFixed(1)}s` : 
        `${((Date.now() - transfer.startTime) / 1000).toFixed(1)}s`;
    
    item.innerHTML = `
        <div class="transfer-info">
            <div style="display: flex; align-items: center; gap: 8px;">
                <span>${statusIcon}</span>
                <strong>${transfer.fileName}</strong>
                <span class="transfer-status">${transfer.type}</span>
            </div>
            ${progressBar}
            <div class="transfer-status">
                ${transfer.fileSize ? formatFileSize(transfer.fileSize) : ''} • 
                ${transfer.status} • ${duration}
                ${transfer.error ? ` • ${transfer.error}` : ''}
            </div>
        </div>
        <div class="transfer-actions">
            ${transfer.status === 'transferring' ? 
                `<button class="btn-sm btn-danger" onclick="cancelTransfer('${transfer.id}')">Cancel</button>` :
                `<button class="btn-sm" onclick="removeTransfer('${transfer.id}')">Remove</button>`
            }
        </div>
    `;
    
    return item;
}

// Get status icon
function getStatusIcon(status) {
    switch (status) {
        case 'starting': return '⏳';
        case 'transferring': return '📡';
        case 'completed': return '✅';
        case 'error': return '❌';
        case 'cancelled': return '🚫';
        default: return '❓';
    }
}

// Format file size
function formatFileSize(bytes) {
    if (bytes === 0) return '0 Bytes';
    
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Simulate transfer for demo purposes
function simulateTransfer(transferId) {
    return new Promise((resolve) => {
        let progress = 0;
        const interval = setInterval(() => {
            progress += Math.random() * 10;
            if (progress >= 100) {
                progress = 100;
                clearInterval(interval);
                resolve();
            }
            updateTransferProgress(transferId, progress);
        }, 200);
    });
}

// Cancel transfer
function cancelTransfer(transferId) {
    console.log(`🚫 Cancelling transfer: ${transferId}`);
    
    const transfer = currentTransfers.get(transferId);
    if (transfer) {
        transfer.status = 'cancelled';
        transfer.endTime = Date.now();
        updateTransferUI();
    }
}

// Remove transfer from list
function removeTransfer(transferId) {
    console.log(`🗑️ Removing transfer: ${transferId}`);
    currentTransfers.delete(transferId);
    updateTransferUI();
}

// Clear completed transfers
function clearCompleted() {
    console.log('🧹 Clearing completed transfers...');
    
    for (const [id, transfer] of currentTransfers) {
        if (transfer.status === 'completed' || transfer.status === 'error' || transfer.status === 'cancelled') {
            currentTransfers.delete(id);
        }
    }
    
    updateTransferUI();
}

// Get transfer statistics
function getTransferStats() {
    const stats = {
        total: currentTransfers.size,
        active: 0,
        completed: 0,
        failed: 0,
        totalBytes: 0,
        transferredBytes: 0
    };
    
    currentTransfers.forEach(transfer => {
        if (transfer.status === 'transferring') stats.active++;
        if (transfer.status === 'completed') stats.completed++;
        if (transfer.status === 'error') stats.failed++;
        
        if (transfer.fileSize) {
            stats.totalBytes += transfer.fileSize;
            stats.transferredBytes += (transfer.fileSize * transfer.progress / 100);
        }
    });
    
    return stats;
}

// Make functions available globally for onclick handlers
window.cancelTransfer = cancelTransfer;
window.removeTransfer = removeTransfer;

// Export functions for use in other modules
window.FileTransfer = {
    initializeFileTransfer,
    uploadFile,
    downloadFile,
    clearCompleted,
    getTransferStats,
    
    // Event handler setters
    setFileUploadHandler: (handler) => { onFileUpload = handler; },
    setFileDownloadHandler: (handler) => { onFileDownload = handler; },
    setTransferProgressHandler: (handler) => { onTransferProgress = handler; },
    setTransferCompleteHandler: (handler) => { onTransferComplete = handler; },
    setTransferErrorHandler: (handler) => { onTransferError = handler; },
    
    // State getters
    getCurrentTransfers: () => Array.from(currentTransfers.values()),
    isTransferActive: () => isTransferActive
};

console.log('✅ File Transfer Module loaded successfully');
