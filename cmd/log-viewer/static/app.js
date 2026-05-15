class LogViewer {
    constructor() {
        this.config = null;
        this.openTabs = new Map();
        this.activeTab = null;
        this.refreshIntervals = new Map();
        this.currentTheme = 'dark';
        this.init();
    }

    async init() {
        await this.loadConfig();
        this.initTheme();
        await this.loadFiles();
    }

    initTheme() {
        this.currentTheme = this.config.theme || 'dark';
        this.applyTheme(this.currentTheme);

        const themeToggle = document.getElementById('theme-toggle');
        themeToggle.addEventListener('click', () => {
            this.currentTheme = this.currentTheme === 'dark' ? 'light' : 'dark';
            this.applyTheme(this.currentTheme);
        });
    }

    applyTheme(theme) {
        const body = document.body;
        const themeIcon = document.querySelector('.theme-icon');

        if (theme === 'light') {
            body.classList.add('light-theme');
            themeIcon.textContent = '☀️';
        } else {
            body.classList.remove('light-theme');
            themeIcon.textContent = '🌙';
        }
    }

    async loadConfig() {
        try {
            const response = await fetch('/api/config');
            this.config = await response.json();
        } catch (error) {
            console.error('Error loading config:', error);
        }
    }

    async loadFiles() {
        try {
            const response = await fetch('/api/logs');
            const files = await response.json();
            this.renderFileButtons(files);
        } catch (error) {
            console.error('Error loading files:', error);
            this.showError('Erro ao carregar arquivos de log');
        }
    }

    renderFileButtons(files) {
        const container = document.getElementById('file-buttons');
        container.innerHTML = '';

        files.forEach(file => {
            const button = document.createElement('button');
            button.className = 'file-button';
            button.dataset.file = file.name;

            if (this.openTabs.has(file.name)) {
                button.classList.add('active');
            }

            const fileName = document.createElement('span');
            fileName.textContent = file.name;
            button.appendChild(fileName);

            if (this.openTabs.has(file.name)) {
                const closeBtn = document.createElement('button');
                closeBtn.className = 'file-button-close';
                closeBtn.textContent = '×';
                closeBtn.onclick = (e) => {
                    e.stopPropagation();
                    this.closeTab(file.name);
                };
                button.appendChild(closeBtn);
            }

            button.onclick = () => {
                if (!this.openTabs.has(file.name)) {
                    this.openTab(file.name);
                } else {
                    this.switchTab(file.name);
                }
            };

            container.appendChild(button);
        });
    }

    openTab(fileName) {
        if (this.openTabs.has(fileName)) {
            this.switchTab(fileName);
            return;
        }

        this.openTabs.set(fileName, {
            name: fileName,
            autoScroll: true,
            lastUpdate: null
        });

        this.switchTab(fileName);
        this.loadLogContent(fileName);
        this.startAutoRefresh(fileName);
        this.refreshFileButtons();
    }

    async refreshFileButtons() {
        const response = await fetch('/api/logs');
        const files = await response.json();
        this.renderFileButtons(files);
    }

    closeTab(fileName) {
        this.stopAutoRefresh(fileName);
        this.openTabs.delete(fileName);

        if (this.activeTab === fileName) {
            const tabs = Array.from(this.openTabs.keys());
            if (tabs.length > 0) {
                this.switchTab(tabs[tabs.length - 1]);
            } else {
                this.activeTab = null;
                this.renderContent();
            }
        }

        this.refreshFileButtons();
    }

    switchTab(fileName) {
        this.activeTab = fileName;
        this.renderContent();
        this.refreshFileButtons();
    }

    renderContent() {
        const container = document.getElementById('log-content');

        if (!this.activeTab) {
            container.innerHTML = `
                <div class="empty-state">
                    <p>Selecione um arquivo de log acima para visualizar</p>
                </div>
            `;
            return;
        }

        const emptyState = container.querySelector('.empty-state');
        if (emptyState) {
            emptyState.remove();
        }

        const existingView = container.querySelector(`[data-file="${this.activeTab}"]`);

        container.querySelectorAll('.log-view').forEach(view => {
            view.classList.remove('active');
        });

        if (existingView) {
            existingView.classList.add('active');
        } else {
            const view = document.createElement('div');
            view.className = 'log-view active';
            view.dataset.file = this.activeTab;
            view.innerHTML = `
                <div class="log-header">
                    <div>
                        <span class="log-title">[${this.activeTab}]</span>
                        <span class="log-info" id="last-update-${this.activeTab}"></span>
                    </div>
                    <div class="log-controls">
                        <label>
                            <input type="checkbox" id="autoscroll-${this.activeTab}" checked>
                            Auto-scroll
                        </label>
                        <span class="log-info">Refresh: ${this.config.refreshInterval}s</span>
                    </div>
                </div>
                <div class="log-body" id="log-body-${this.activeTab}">
                    <div class="loading">Carregando...</div>
                </div>
            `;
            container.appendChild(view);

            document.getElementById(`autoscroll-${this.activeTab}`).onchange = (e) => {
                const tab = this.openTabs.get(this.activeTab);
                if (tab) {
                    tab.autoScroll = e.target.checked;
                }
            };
        }
    }

    async loadLogContent(fileName) {
        try {
            const response = await fetch(`/api/log?file=${encodeURIComponent(fileName)}`);
            const data = await response.json();

            const bodyElement = document.getElementById(`log-body-${fileName}`);
            const updateElement = document.getElementById(`last-update-${fileName}`);

            if (bodyElement) {
                bodyElement.textContent = data.content || '(arquivo vazio)';

                const tab = this.openTabs.get(fileName);
                if (tab && tab.autoScroll) {
                    bodyElement.scrollTop = bodyElement.scrollHeight;
                }
            }

            if (updateElement) {
                const now = new Date();
                updateElement.textContent = ` - Atualizado às ${now.toLocaleTimeString()}`;
            }

            const tab = this.openTabs.get(fileName);
            if (tab) {
                tab.lastUpdate = new Date();
            }

        } catch (error) {
            console.error(`Error loading log ${fileName}:`, error);
            const bodyElement = document.getElementById(`log-body-${fileName}`);
            if (bodyElement) {
                bodyElement.innerHTML = `<div class="error-message">Erro ao carregar o log: ${error.message}</div>`;
            }
        }
    }

    startAutoRefresh(fileName) {
        if (this.refreshIntervals.has(fileName)) {
            return;
        }

        const intervalId = setInterval(() => {
            if (this.openTabs.has(fileName)) {
                this.loadLogContent(fileName);
            }
        }, this.config.refreshInterval * 1000);

        this.refreshIntervals.set(fileName, intervalId);
    }

    stopAutoRefresh(fileName) {
        if (this.refreshIntervals.has(fileName)) {
            clearInterval(this.refreshIntervals.get(fileName));
            this.refreshIntervals.delete(fileName);
        }
    }

    showError(message) {
        const container = document.getElementById('log-content');
        container.innerHTML = `
            <div class="error-message">${message}</div>
        `;
    }
}

document.addEventListener('DOMContentLoaded', () => {
    new LogViewer();
});
