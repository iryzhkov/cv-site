// ============================================
// Terminal UI — Vim Nav, Commands, Themes
// ============================================

(function() {
    'use strict';

    // --- Theme Management ---
    const THEMES = ['modern', 'green', 'amber', 'purple', 'dracula', 'solarized', 'nord', 'gruvbox', 'catppuccin', 'cyberpunk', 'solarized-light', 'github-light', 'paper'];
    let currentTheme = localStorage.getItem('cv-theme') || 'paper';

    function applyTheme(name) {
        if (!THEMES.includes(name)) name = 'modern';
        currentTheme = name;
        document.documentElement.className = name === 'modern' ? '' : 'theme-' + name;
        localStorage.setItem('cv-theme', name);
        updateStatusBar();
    }

    function cycleTheme() {
        const idx = THEMES.indexOf(currentTheme);
        applyTheme(THEMES[(idx + 1) % THEMES.length]);
    }

    // --- Tab Navigation ---
    const TABS = [
        { name: 'home', href: '/', aliases: ['home', 'h'] },
        { name: 'about', href: '/about', aliases: ['about', 'a'] },
        { name: 'projects', href: '/projects', aliases: ['projects', 'proj'] },
        { name: 'play', href: '/playground', aliases: ['play', 'playground', 'chat', 'p'] },
        { name: 'bench', href: '/benchmarks', aliases: ['bench', 'benchmarks', 'b'] },
    ];

    function getActiveTabIndex() {
        const path = window.location.pathname;
        for (let i = 0; i < TABS.length; i++) {
            if (TABS[i].href === path) return i;
            if (i > 0 && path.startsWith(TABS[i].href + '/')) return i;
        }
        // Default to home for unknown paths
        if (path === '/') return 0;
        return -1;
    }

    function switchTab(index) {
        if (index < 0) index = TABS.length - 1;
        if (index >= TABS.length) index = 0;
        window.location.href = TABS[index].href;
    }

    function highlightActiveTab() {
        const idx = getActiveTabIndex();
        document.querySelectorAll('.terminal-tabs .tab').forEach((tab, i) => {
            tab.classList.toggle('active', i === idx);
        });
        // Update title bar path
        const titleEl = document.querySelector('.terminal-title');
        if (titleEl) {
            const page = idx >= 0 ? TABS[idx].name : window.location.pathname;
            titleEl.textContent = 'igor@cv-site:~/' + page;
        }
    }

    // --- Mode Management ---
    let mode = 'normal'; // 'normal' | 'command'

    function enterCommandMode() {
        mode = 'command';
        const input = document.getElementById('command-input');
        if (input) {
            input.classList.remove('hidden');
            input.value = ':';
            input.focus();
            input.setSelectionRange(1, 1);
        }
        updateStatusBar();
    }

    function exitCommandMode() {
        mode = 'normal';
        const input = document.getElementById('command-input');
        if (input) {
            input.classList.add('hidden');
            input.value = '';
            input.blur();
        }
        // Also blur any focused input
        if (document.activeElement && document.activeElement.tagName !== 'BODY') {
            document.activeElement.blur();
        }
        updateStatusBar();
    }

    // --- Command System ---
    function executeCommand(str) {
        str = str.trim();
        if (str.startsWith(':')) str = str.substring(1);
        const parts = str.split(/\s+/);
        const cmd = parts[0].toLowerCase();
        const args = parts.slice(1).join(' ');

        // Navigation commands
        for (const tab of TABS) {
            if (tab.aliases.includes(cmd)) {
                window.location.href = tab.href;
                return;
            }
        }

        switch (cmd) {
            case 'admin':
                exitCommandMode();
                window.location.href = '/admin';
                return;

            case 'q':
            case 'quit':
                history.back();
                return;

            case 'theme':
                if (args && THEMES.includes(args)) {
                    applyTheme(args);
                } else {
                    cycleTheme();
                }
                break;

            case 'help':
                showHelp();
                break;

            case 'model':
                if (args) setModel(args);
                break;

            case 'system':
                if (args) setSystemPrompt(args);
                break;

            case 'models':
                listModels();
                break;

            case 'clear':
                if (typeof window.clearChat === 'function') window.clearChat();
                break;

            default:
                // Unknown command — flash status bar
                const hints = document.getElementById('status-hints');
                if (hints) {
                    const orig = hints.textContent;
                    hints.textContent = 'Unknown command: ' + cmd;
                    hints.style.color = 'var(--term-error)';
                    setTimeout(() => { hints.textContent = orig; hints.style.color = ''; }, 2000);
                }
        }

        exitCommandMode();
    }

    // --- Model/System helpers (playground integration) ---
    function setModel(name) {
        const select = document.getElementById('model-select');
        if (select) {
            // Try to find matching option
            for (const opt of select.options) {
                if (opt.value === name || opt.value.startsWith(name)) {
                    select.value = opt.value;
                    select.dispatchEvent(new Event('change'));
                    return;
                }
            }
        }
        const input = document.getElementById('model-input');
        if (input) input.value = name;
    }

    function setSystemPrompt(name) {
        const select = document.getElementById('system-select');
        if (select) {
            for (const opt of select.options) {
                if (opt.textContent.toLowerCase().includes(name.toLowerCase())) {
                    select.value = opt.value;
                    select.dispatchEvent(new Event('change'));
                    return;
                }
            }
        }
    }

    function listModels() {
        fetch('/api/models')
            .then(r => r.text())
            .then(html => {
                // Parse option elements to get model names
                const parser = new DOMParser();
                const doc = parser.parseFromString('<select>' + html + '</select>', 'text/html');
                const options = doc.querySelectorAll('option');
                const names = Array.from(options).map(o => o.value);

                // Display in chat area if on playground
                const chat = document.getElementById('chat-messages');
                if (chat) {
                    const div = document.createElement('div');
                    div.className = 'term-line dim';
                    div.textContent = 'Available models: ' + names.join(', ');
                    chat.appendChild(div);
                    chat.scrollTop = chat.scrollHeight;
                }
            });
    }

    // --- Help Overlay ---
    function showHelp() {
        // Remove existing
        document.getElementById('help-overlay')?.remove();

        const overlay = document.createElement('div');
        overlay.id = 'help-overlay';
        overlay.className = 'help-overlay';
        overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

        overlay.innerHTML = `
            <div class="help-content">
                <h2>:help — Keyboard Shortcuts</h2>
                <div class="help-row"><span class="help-key">h / l</span><span class="help-desc">Previous / next tab</span></div>
                <div class="help-row"><span class="help-key">j / k</span><span class="help-desc">Scroll down / up</span></div>
                <div class="help-row"><span class="help-key">g / G</span><span class="help-desc">Top / bottom</span></div>
                <div class="help-row"><span class="help-key">1-7</span><span class="help-desc">Jump to tab</span></div>
                <div class="help-row"><span class="help-key">i / /</span><span class="help-desc">Focus input (insert mode)</span></div>
                <div class="help-row"><span class="help-key">:</span><span class="help-desc">Command mode</span></div>
                <div class="help-row"><span class="help-key">Esc</span><span class="help-desc">Exit mode / close</span></div>
                <br>
                <h2>Commands</h2>
                <div class="help-row"><span class="help-key">:theme [name]</span><span class="help-desc">Switch color scheme</span></div>
                <div class="help-row"><span class="help-key">:model [name]</span><span class="help-desc">Set LLM model</span></div>
                <div class="help-row"><span class="help-key">:models</span><span class="help-desc">List available models</span></div>
                <div class="help-row"><span class="help-key">:system [name]</span><span class="help-desc">Set system prompt</span></div>
                <div class="help-row"><span class="help-key">:clear</span><span class="help-desc">Clear chat</span></div>
                <div class="help-row"><span class="help-key">:q</span><span class="help-desc">Go back</span></div>
                <br>
                <h2>Themes</h2>
                <div class="help-row"><span class="help-desc">${THEMES.join(', ')}</span></div>
                <br>
                <div style="color:var(--term-fg-dim);font-size:11px">Press Esc to close</div>
            </div>
        `;
        document.body.appendChild(overlay);
    }

    // --- Keyboard Handler ---
    function isInputFocused() {
        const el = document.activeElement;
        if (!el) return false;
        const tag = el.tagName;
        return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable;
    }

    function handleKeyDown(e) {
        // Help overlay: Esc to close
        const helpOverlay = document.getElementById('help-overlay');
        if (helpOverlay && e.key === 'Escape') {
            helpOverlay.remove();
            return;
        }

        // Command mode
        if (mode === 'command') {
            if (e.key === 'Escape') {
                exitCommandMode();
                e.preventDefault();
            } else if (e.key === 'Enter') {
                const input = document.getElementById('command-input');
                if (input) executeCommand(input.value);
                e.preventDefault();
            }
            return;
        }

        // Normal mode — skip if in input
        if (isInputFocused()) {
            if (e.key === 'Escape') {
                exitCommandMode();
                e.preventDefault();
            }
            return;
        }

        const content = document.getElementById('terminal-content');

        switch (e.key) {
            case 'h':
                switchTab(getActiveTabIndex() - 1);
                e.preventDefault();
                break;
            case 'l':
                switchTab(getActiveTabIndex() + 1);
                e.preventDefault();
                break;
            case 'j':
                if (content) content.scrollBy({ top: 60 });
                e.preventDefault();
                break;
            case 'k':
                if (content) content.scrollBy({ top: -60 });
                e.preventDefault();
                break;
            case 'g':
                if (content) content.scrollTo({ top: 0 });
                e.preventDefault();
                break;
            case 'G':
                if (content) content.scrollTo({ top: content.scrollHeight });
                e.preventDefault();
                break;
            case ':':
                enterCommandMode();
                e.preventDefault();
                break;
            case 'i':
            case '/':
                // Focus the main input on the page (chat prompt, search, etc.)
                const mainInput = document.getElementById('prompt-input')
                    || content?.querySelector('input[type="text"]:not(.hidden):not(.command-input), textarea');
                if (mainInput) {
                    mainInput.focus();
                    e.preventDefault();
                }
                break;
            case 'Escape':
                exitCommandMode();
                e.preventDefault();
                break;
            default:
                // Number keys 1-7 for tab switching (1-based)
                const num = parseInt(e.key);
                if (num >= 1 && num <= 7) {
                    switchTab(num - 1);
                    e.preventDefault();
                }
        }
    }

    // --- Status Bar ---
    function updateStatusBar() {
        const modeEl = document.getElementById('status-mode');
        const hintsEl = document.getElementById('status-hints');
        if (modeEl) {
            modeEl.textContent = mode === 'command' ? 'COMMAND' : 'NORMAL';
            modeEl.classList.toggle('command', mode === 'command');
        }
        if (hintsEl) {
            hintsEl.textContent = ':help | h/l tabs | j/k scroll | i insert | theme: ' + currentTheme;
        }
    }

    // --- Typewriter Animation (first visit per tab) ---
    function getVisitedTabs() {
        try { return JSON.parse(sessionStorage.getItem('cv-visited-tabs') || '{}'); } catch { return {}; }
    }

    function markTabVisited(path) {
        const visited = getVisitedTabs();
        visited[path] = true;
        sessionStorage.setItem('cv-visited-tabs', JSON.stringify(visited));
    }

    function typewriterIntro() {
        const path = window.location.pathname;
        if (getVisitedTabs()[path]) return;
        markTabVisited(path);

        const content = document.getElementById('terminal-content');
        if (!content) return;

        const children = Array.from(content.children);
        if (children.length === 0) return;

        // Hide all children
        children.forEach(el => {
            el.style.opacity = '0';
            el.style.transform = 'translateY(2px)';
        });

        // Reveal line by line
        let delay = 100;
        children.forEach((el) => {
            const tag = el.tagName.toLowerCase();
            if (tag === 'pre' || el.classList.contains('neofetch-box')) {
                typewriterPre(el, delay);
                delay += Math.min(el.textContent.length * 6, 2000);
            } else {
                setTimeout(() => {
                    el.style.transition = 'opacity 0.15s ease, transform 0.15s ease';
                    el.style.opacity = '1';
                    el.style.transform = 'translateY(0)';
                }, delay);
                delay += 40;
            }
        });
    }

    function typewriterPre(el, startDelay) {
        const fullText = el.innerHTML;
        el.innerHTML = '';
        el.style.opacity = '1';
        el.style.transform = 'translateY(0)';

        let charIndex = 0;
        const speed = 6; // ms per character — fast typing
        const maxChars = fullText.length;

        setTimeout(() => {
            function type() {
                if (charIndex < maxChars) {
                    const chunk = Math.min(4, maxChars - charIndex);
                    el.innerHTML = fullText.substring(0, charIndex + chunk);
                    charIndex += chunk;
                    setTimeout(type, speed);
                }
            }
            type();
        }, startDelay);
    }

    // --- Init ---
    function init() {
        applyTheme(currentTheme);
        highlightActiveTab();
        updateStatusBar();
        document.addEventListener('keydown', handleKeyDown);
        typewriterIntro();

        // Re-highlight tabs after HTMX navigation
        document.body.addEventListener('htmx:afterSettle', () => {
            highlightActiveTab();
            updateStatusBar();
        });
    }

    // Expose for page-level scripts
    window.Terminal = {
        cycleTheme,
        applyTheme,
        getThemeColors: () => {
            const s = getComputedStyle(document.documentElement);
            return {
                accent: s.getPropertyValue('--term-accent').trim(),
                fg: s.getPropertyValue('--term-fg').trim(),
                fgDim: s.getPropertyValue('--term-fg-dim').trim(),
                border: s.getPropertyValue('--term-border').trim(),
                bg: s.getPropertyValue('--term-bg').trim(),
            };
        }
    };

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
