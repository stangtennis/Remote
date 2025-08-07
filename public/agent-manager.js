// Agent Management System - Inspired by MeshCentral
// Allows admins to generate and download pre-configured client agents

class AgentManager {
    constructor(supabase) {
        this.supabase = supabase;
        this.generatedAgents = new Map();
        this.setupEventListeners();
    }

    setupEventListeners() {
        // Agent generation form
        document.getElementById('generate-agent-form')?.addEventListener('submit', (e) => {
            e.preventDefault();
            this.generateAgent();
        });

        // Platform selection
        document.getElementById('platform-select')?.addEventListener('change', (e) => {
            this.updatePlatformInfo(e.target.value);
        });

        // Refresh agents list
        document.getElementById('refresh-agents')?.addEventListener('click', () => {
            this.loadGeneratedAgents();
        });
    }

    async generateAgent() {
        try {
            const platform = document.getElementById('platform-select').value;
            const deviceName = document.getElementById('device-name').value || 'Remote Device';
            const autoStart = document.getElementById('auto-start').checked;
            const hideWindow = document.getElementById('hide-window').checked;

            this.updateStatus('Generating agent...', 'info');
            
            // Show loading state
            const generateBtn = document.getElementById('generate-btn');
            const originalText = generateBtn.textContent;
            generateBtn.textContent = 'Generating...';
            generateBtn.disabled = true;

            // Call the Supabase Edge Function directly
            const queryParams = new URLSearchParams({
                platform,
                deviceName,
                autoStart: autoStart.toString(),
                hideWindow: hideWindow.toString(),
                orgId: 'default'
            });
            
            const response = await fetch(`https://ptrtibzwokjcjjxvjpin.supabase.co/functions/v1/agent-builder?${queryParams}`, {
                method: 'GET',
                headers: {
                    'Authorization': `Bearer ${this.supabase.supabaseKey}`,
                    'Content-Type': 'application/json'
                }
            });

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || 'Failed to generate agent');
            }

            // Get the generated agent file
            const agentBlob = await response.blob();
            const filename = this.getFilename(platform, deviceName);

            // Download the file
            this.downloadFile(agentBlob, filename);

            // Log the generation
            await this.logAgentGeneration(platform, deviceName, filename);

            this.updateStatus(`Agent generated successfully: ${filename}`, 'success');
            
            // Refresh the agents list
            await this.loadGeneratedAgents();

        } catch (error) {
            console.error('❌ Failed to generate agent:', error);
            this.updateStatus(`Failed to generate agent: ${error.message}`, 'error');
        } finally {
            // Reset button state
            const generateBtn = document.getElementById('generate-btn');
            generateBtn.textContent = 'Generate & Download Agent';
            generateBtn.disabled = false;
        }
    }

    // Removed buildQueryParams - using direct URL construction instead

    downloadFile(blob, filename) {
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.style.display = 'none';
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
    }

    getFilename(platform, deviceName) {
        const safeName = deviceName.replace(/[^a-zA-Z0-9]/g, '_');
        const timestamp = new Date().toISOString().slice(0, 10);
        
        switch (platform) {
            case 'windows':
                return `RemoteAgent_${safeName}_${timestamp}.bat`;
            case 'macos':
                return `RemoteAgent_${safeName}_${timestamp}.sh`;
            case 'linux':
                return `RemoteAgent_${safeName}_${timestamp}.sh`;
            default:
                return `RemoteAgent_${safeName}_${timestamp}.txt`;
        }
    }

    async logAgentGeneration(platform, deviceName, filename) {
        try {
            await this.supabase
                .from('agent_generations')
                .insert({
                    platform,
                    device_name: deviceName,
                    filename,
                    generated_at: new Date().toISOString(),
                    generated_by: this.supabase.auth.user()?.id
                });
        } catch (error) {
            console.error('⚠️ Failed to log agent generation:', error);
            // Don't throw - this is not critical
        }
    }

    async loadGeneratedAgents() {
        try {
            const { data: agents, error } = await this.supabase
                .from('agent_generations')
                .select('*')
                .order('generated_at', { ascending: false })
                .limit(50);

            if (error) throw error;

            this.renderAgentsList(agents);
            
        } catch (error) {
            console.error('❌ Failed to load agents:', error);
            this.updateStatus('Failed to load agents list', 'error');
        }
    }

    renderAgentsList(agents) {
        const agentsList = document.getElementById('generated-agents-list');
        if (!agentsList) return;

        if (!agents || agents.length === 0) {
            agentsList.innerHTML = '<p class="no-data">No agents generated yet</p>';
            return;
        }

        agentsList.innerHTML = agents.map(agent => `
            <div class="agent-item">
                <div class="agent-info">
                    <h4>${agent.filename}</h4>
                    <p><strong>Platform:</strong> ${this.getPlatformIcon(agent.platform)} ${agent.platform}</p>
                    <p><strong>Device:</strong> ${agent.device_name}</p>
                    <p><strong>Generated:</strong> ${new Date(agent.generated_at).toLocaleString()}</p>
                </div>
                <div class="agent-actions">
                    <button onclick="agentManager.regenerateAgent('${agent.platform}', '${agent.device_name}')" 
                            class="btn-secondary">
                        🔄 Regenerate
                    </button>
                </div>
            </div>
        `).join('');
    }

    getPlatformIcon(platform) {
        switch (platform) {
            case 'windows': return '🪟';
            case 'macos': return '🍎';
            case 'linux': return '🐧';
            default: return '💻';
        }
    }

    async regenerateAgent(platform, deviceName) {
        // Pre-fill the form and generate
        document.getElementById('platform-select').value = platform;
        document.getElementById('device-name').value = deviceName;
        await this.generateAgent();
    }

    updatePlatformInfo(platform) {
        const infoElement = document.getElementById('platform-info');
        if (!infoElement) return;

        const platformInfo = {
            windows: {
                icon: '🪟',
                name: 'Windows',
                description: 'Generates a .bat file that downloads and installs the agent',
                requirements: 'Windows 10 or later, Internet connection'
            },
            macos: {
                icon: '🍎',
                name: 'macOS',
                description: 'Generates a .sh script that installs the agent',
                requirements: 'macOS 10.14 or later, Internet connection'
            },
            linux: {
                icon: '🐧',
                name: 'Linux',
                description: 'Generates a .sh script that installs the agent',
                requirements: 'Ubuntu 18.04+ or equivalent, Internet connection'
            }
        };

        const info = platformInfo[platform];
        if (info) {
            infoElement.innerHTML = `
                <div class="platform-info">
                    <h4>${info.icon} ${info.name}</h4>
                    <p>${info.description}</p>
                    <p><strong>Requirements:</strong> ${info.requirements}</p>
                </div>
            `;
        }
    }

    updateStatus(message, type = 'info') {
        const statusElement = document.getElementById('agent-status');
        if (!statusElement) return;

        statusElement.textContent = message;
        statusElement.className = `status ${type}`;
        
        // Auto-clear after 5 seconds for non-error messages
        if (type !== 'error') {
            setTimeout(() => {
                statusElement.textContent = '';
                statusElement.className = 'status';
            }, 5000);
        }
    }

    // Initialize agent statistics
    async loadAgentStatistics() {
        try {
            // Get total agents generated
            const { count: totalAgents } = await this.supabase
                .from('agent_generations')
                .select('*', { count: 'exact', head: true });

            // Get agents by platform
            const { data: platformStats } = await this.supabase
                .from('agent_generations')
                .select('platform')
                .then(result => {
                    const stats = {};
                    result.data?.forEach(agent => {
                        stats[agent.platform] = (stats[agent.platform] || 0) + 1;
                    });
                    return { data: stats };
                });

            // Get currently online devices
            const { count: onlineDevices } = await this.supabase
                .from('device_presence')
                .select('*', { count: 'exact', head: true })
                .eq('status', 'online');

            this.renderStatistics({
                totalAgents: totalAgents || 0,
                platformStats: platformStats || {},
                onlineDevices: onlineDevices || 0
            });

        } catch (error) {
            console.error('❌ Failed to load statistics:', error);
        }
    }

    renderStatistics(stats) {
        const statsContainer = document.getElementById('agent-statistics');
        if (!statsContainer) return;

        statsContainer.innerHTML = `
            <div class="stats-grid">
                <div class="stat-card">
                    <h3>${stats.totalAgents}</h3>
                    <p>Total Agents Generated</p>
                </div>
                <div class="stat-card">
                    <h3>${stats.onlineDevices}</h3>
                    <p>Currently Online</p>
                </div>
                <div class="stat-card">
                    <h3>${Object.keys(stats.platformStats).length}</h3>
                    <p>Supported Platforms</p>
                </div>
            </div>
            <div class="platform-breakdown">
                <h4>Platform Distribution</h4>
                ${Object.entries(stats.platformStats).map(([platform, count]) => `
                    <div class="platform-stat">
                        <span>${this.getPlatformIcon(platform)} ${platform}</span>
                        <span>${count} agents</span>
                    </div>
                `).join('')}
            </div>
        `;
    }
}

// Export for use in main dashboard
window.AgentManager = AgentManager;
