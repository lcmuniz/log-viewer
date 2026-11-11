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
            this.config = await this.fetchJSON('/api/config');
        } catch (error) {
            console.error('Error loading config:', error);
            this.config = { refreshInterval: 5, theme: 'light' };
        }
    }

    async fetchJSON(url) {
        const response = await fetch(url, { cache: 'no-store' });
        if (!response.ok) {
            const text = (await response.text()).trim();
            throw new Error(text || `HTTP ${response.status}`);
        }
        return response.json();
    }

    // DOM ids are derived from file names; keep them free of special chars.
    domId(prefix, fileName) {
        return `${prefix}-${encodeURIComponent(fileName).replace(/%/g, '_')}`;
    }

    el(tag, className, text) {
        const node = document.createElement(tag);
        if (className) node.className = className;
        if (text !== undefined) node.textContent = text;
        return node;
    }

    async loadFiles() {
        try {
            const files = await this.fetchJSON('/api/logs');
            this.renderFileButtons(files);
        } catch (error) {
            console.error('Error loading files:', error);
            this.showError('Erro ao carregar arquivos de log');
        }
    }

    renderFileButtons(files) {
        const container = document.getElementById('file-buttons');
        container.replaceChildren();

        if (files.length === 0) {
            container.appendChild(this.el('p', 'log-info', 'Nenhum arquivo .log/.txt encontrado na pasta.'));
            return;
        }

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
        try {
            const files = await this.fetchJSON('/api/logs');
            this.renderFileButtons(files);
        } catch (error) {
            console.error('Error refreshing files:', error);
        }
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
            const empty = this.el('div', 'empty-state');
            empty.appendChild(this.el('p', null, 'Selecione um arquivo de log acima para visualizar'));
            container.replaceChildren(empty);
            return;
        }

        const emptyState = container.querySelector('.empty-state');
        if (emptyState) {
            emptyState.remove();
        }

        const existingView = Array.from(container.querySelectorAll('.log-view'))
            .find(view => view.dataset.file === this.activeTab);

        container.querySelectorAll('.log-view').forEach(view => {
            view.classList.remove('active');
        });

        if (existingView) {
            existingView.classList.add('active');
        } else {
            const view = document.createElement('div');
            view.className = 'log-view active';
            view.dataset.file = this.activeTab;
            const fileName = this.activeTab;

            const header = this.el('div', 'log-header');
            const titleBox = this.el('div');
            titleBox.appendChild(this.el('span', 'log-title', `[${fileName}]`));
            const info = this.el('span', 'log-info');
            info.id = this.domId('last-update', fileName);
            titleBox.appendChild(info);

            const controls = this.el('div', 'log-controls');
            const label = this.el('label');
            const checkbox = document.createElement('input');
            checkbox.type = 'checkbox';
            checkbox.checked = true;
            checkbox.onchange = (e) => {
                const tab = this.openTabs.get(fileName);
                if (tab) {
                    tab.autoScroll = e.target.checked;
                }
            };
            label.append(checkbox, ' Auto-scroll');
            controls.append(label, this.el('span', 'log-info', `Refresh: ${this.config.refreshInterval}s`));
            header.append(titleBox, controls);

            const body = this.el('div', 'log-body');
            body.id = this.domId('log-body', fileName);
            body.appendChild(this.el('div', 'loading', 'Carregando...'));

            view.append(header, body);
            container.appendChild(view);
        }
    }

    async loadLogContent(fileName) {
        try {
            const data = await this.fetchJSON(`/api/log?file=${encodeURIComponent(fileName)}`);

            const bodyElement = document.getElementById(this.domId('log-body', fileName));
            const updateElement = document.getElementById(this.domId('last-update', fileName));

            if (bodyElement) {
                let text = data.content || '(arquivo vazio)';
                if (data.truncated) {
                    text = `… (mostrando apenas o final do arquivo de ${data.size} bytes)\n` + text;
                }
                bodyElement.textContent = text;

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
            const bodyElement = document.getElementById(this.domId('log-body', fileName));
            if (bodyElement) {
                bodyElement.replaceChildren(this.el('div', 'error-message', `Erro ao carregar o log: ${error.message}`));
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
        container.replaceChildren(this.el('div', 'error-message', message));
    }
}

document.addEventListener('DOMContentLoaded', () => {
    new LogViewer();
});
