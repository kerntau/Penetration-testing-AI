(function () {
    'use strict';

    const STORAGE_KEY = 'ptai-theme';
    const THEMES = ['light', 'dark'];
    const FALLBACK_LABELS = {
        light: '浅色',
        dark: '暗色'
    };
    const FALLBACK_TITLES = {
        light: '当前：浅色主题。',
        dark: '当前：暗色主题。'
    };
    const TITLE_KEYS = {
        light: 'titleLight',
        dark: 'titleDark'
    };

    function themeText(key, fallback) {
        if (typeof window.t === 'function') {
            const value = window.t('theme.' + key);
            if (value && value !== 'theme.' + key) {
                return value;
            }
        }
        return fallback;
    }

    function getLabel(preference) {
        return themeText(preference, FALLBACK_LABELS[preference] || FALLBACK_LABELS.light);
    }

    function getTitle(preference) {
        const titleKey = TITLE_KEYS[preference] || TITLE_KEYS.light;
        return themeText(titleKey, FALLBACK_TITLES[preference] || FALLBACK_TITLES.light);
    }

    function normalizePreference(value) {
        return THEMES.includes(value) ? value : 'light';
    }

    function readPreference() {
        try {
            return normalizePreference(localStorage.getItem(STORAGE_KEY));
        } catch (err) {
            return 'light';
        }
    }

    function resolveTheme(preference) {
        return preference === 'dark' ? 'dark' : 'light';
    }

    function updateButton(preference, resolved) {
        document.querySelectorAll('.theme-switch-btn').forEach(function (btn) {
            const active = btn.dataset.themeValue === preference;
            btn.classList.toggle('is-active', active);
            btn.setAttribute('aria-pressed', active ? 'true' : 'false');
        });
        const legacy = document.getElementById('theme-toggle-btn');
        if (legacy) {
            legacy.dataset.themePreference = preference;
            legacy.dataset.theme = resolved;
            legacy.title = getTitle(preference);
            legacy.setAttribute('aria-label', getTitle(preference));
        }
    }

    function applyTheme(preference) {
        const normalized = normalizePreference(preference);
        const resolved = resolveTheme(normalized);
        const root = document.documentElement;
        root.setAttribute('data-theme-preference', normalized);
        root.setAttribute('data-theme', resolved);
        root.style.colorScheme = resolved;
        updateButton(normalized, resolved);
        document.dispatchEvent(
            new CustomEvent('ptai-themechange', {
                detail: { preference: normalized, resolved: resolved },
            }),
        );
    }

    function savePreference(preference) {
        const normalized = normalizePreference(preference);
        try {
            localStorage.setItem(STORAGE_KEY, normalized);
        } catch (err) {
            // Ignore storage failures; the current page can still apply the theme.
        }
        applyTheme(normalized);
    }

    window.setThemePreference = savePreference;
    window.getThemePreference = readPreference;
    window.cycleThemePreference = function () {
        const current = readPreference();
        const next = THEMES[(THEMES.indexOf(current) + 1) % THEMES.length];
        savePreference(next);
    };
    window.refreshThemeToggleLabel = function () {
        applyTheme(readPreference());
    };

    document.addEventListener('languagechange', function () {
        applyTheme(readPreference());
    });

    function initTheme() {
        applyTheme(readPreference());
        if (window.i18nReady && typeof window.i18nReady.then === 'function') {
            window.i18nReady.then(function () {
                applyTheme(readPreference());
            });
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initTheme);
    } else {
        initTheme();
    }
})();
