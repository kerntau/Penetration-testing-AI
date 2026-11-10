// 认证与通用前端工具。
// 平台改为免登录直进后，这里不再维护登录态：apiFetch 直接请求，权限判定恒为放行。
let isAppInitialized = false;

/** 带统一错误处理的 fetch 包装（401 视为请求失败，不再跳转登录页）。 */
async function apiFetch(url, options = {}) {
    const response = await fetch(url, options);
    if (response.status === 401) {
        const msg = (typeof window !== 'undefined' && typeof window.t === 'function')
            ? window.t('auth.unauthorized')
            : '未授权访问';
        notifyApiError(msg);
        throw new Error(msg);
    }
    // 403 属于可预期的拒绝，返回 Response 供调用方通过 res.ok / ensureApiOk 处理。
    return response;
}

function hasPermission(permission) {
    return true;
}

function hasAnyPermission(permissions) {
    return true;
}

async function readApiError(response, fallback) {
    if (!response) {
        return fallback || authT('auth.requestFailed', '请求失败');
    }
    try {
        const body = await response.clone().json();
        return body.error || body.message || fallback || authT('auth.requestFailed', '请求失败');
    } catch (error) {
        return fallback || authT('auth.requestFailed', '请求失败');
    }
}

function notifyApiError(message, type = 'error') {
    const text = (message || '').trim() || authT('auth.requestFailed', '请求失败');
    if (typeof showNotification === 'function') {
        showNotification(text, type);
        return;
    }
    if (typeof showToast === 'function') {
        showToast(text, type);
        return;
    }
    alert(text);
}

async function notifyApiResponseError(response, fallback) {
    notifyApiError(await readApiError(response, fallback));
}

async function ensureApiOk(response, fallback) {
    if (response && response.ok) return true;
    await notifyApiResponseError(response, fallback);
    return false;
}

function requirePermission(permission, customMessage) {
    const allowed = Array.isArray(permission)
        ? hasAnyPermission(permission)
        : hasPermission(permission);
    if (allowed) return true;
    notifyApiError(customMessage || authT('auth.forbidden', '权限不足'));
    return false;
}

function permissionAllowedForElement(el) {
    if (!el) return true;
    const anyOf = el.getAttribute('data-require-permission-any');
    const permission = el.getAttribute('data-require-permission');
    if (anyOf) {
        return hasAnyPermission(anyOf.split(/[\s,|]+/).map((item) => item.trim()).filter(Boolean));
    }
    if (permission) {
        return hasPermission(permission);
    }
    return true;
}

function applyPermissionElement(el) {
    const anyOf = el.getAttribute('data-require-permission-any');
    const permission = el.getAttribute('data-require-permission');
    if (!anyOf && !permission) return;
    const allowed = permissionAllowedForElement(el);
    el.hidden = !allowed;
    el.classList.toggle('rbac-permission-denied', !allowed);
    // Permission decides whether this user may ever use the control; it does not decide
    // whether the control is ready. A page that disables one for a reason of its own marks
    // it with data-state-disabled, because writing el.disabled = !allowed here otherwise
    // re-enables it for exactly the users who are allowed - and a button whose preconditions
    // are unmet is then pressed, and fails.
    const blockedByState = el.dataset && el.dataset.stateDisabled === 'true';
    const disabled = !allowed || blockedByState;
    if ('disabled' in el) {
        el.disabled = disabled;
    }
    el.setAttribute('aria-hidden', allowed ? 'false' : 'true');
    el.setAttribute('aria-disabled', disabled ? 'true' : 'false');
}

function installPermissionClickGuard() {
    // 免登录后不再有权限拦截：保留空实现，避免调用方需要感知机制变化。
}

function applyRBACToUI(root) {
    const permissionRoot = root instanceof Element ? root : document;
    permissionRoot.querySelectorAll('[data-require-permission], [data-require-permission-any]').forEach(applyPermissionElement);
    if (permissionRoot instanceof Element && permissionRoot.matches('[data-require-permission], [data-require-permission-any]')) {
        applyPermissionElement(permissionRoot);
    }
}

function authT(key, fallback, opts = {}) {
    if (typeof window !== 'undefined' && typeof window.t === 'function') {
        const translated = window.t(key, opts);
        if (translated && translated !== key) {
            return translated;
        }
    }
    return fallback.replace(/\{\{\s*(\w+)\s*\}\}/g, function (_, name) {
        return Object.prototype.hasOwnProperty.call(opts, name) ? String(opts[name]) : '';
    });
}

// 通用通知：优先使用页面自定义 toast，便于统一风格与自动消失。
function showNotification(message, type = 'info') {
    showToastNotification(message, type);
}

function showToastNotification(message, type = 'info') {
    // 创建通知容器（如果不存在）
    let container = document.getElementById('toast-notification-container');
    if (!container) {
        container = document.createElement('div');
        container.id = 'toast-notification-container';
        container.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            z-index: 10100;
            display: flex;
            flex-direction: column;
            gap: 12px;
            pointer-events: none;
        `;
        document.body.appendChild(container);
    }

    // 创建通知元素
    const toast = document.createElement('div');
    toast.className = `toast-notification toast-${type}`;

    const typeStyles = {
        success: {
            background: '#f4fbf6',
            border: '1px solid #cce8d4',
            color: '#3d6654',
            iconColor: '#52a06a',
            icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2"/><path d="M5 8l2 2 4-4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>'
        },
        error: {
            background: '#fef7f7',
            border: '1px solid #f3d0d0',
            color: '#8b4444',
            iconColor: '#c96a6a',
            icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2"/><path d="M6 6l4 4M10 6l-4 4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>'
        },
        info: {
            background: '#f5f9ff',
            border: '1px solid #cfe0f5',
            color: '#4a6078',
            iconColor: '#6b8fbf',
            icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2"/><path d="M8 7v4M8 5.5v.01" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>'
        },
        warning: {
            background: '#fffbf3',
            border: '1px solid #f0dfc0',
            color: '#7a6535',
            iconColor: '#c4a04a',
            icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="M8 2.5l6 10.5H2L8 2.5z" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/><path d="M8 7v2.5M8 11v.01" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>'
        }
    };

    const style = typeStyles[type] || typeStyles.info;

    toast.style.cssText = `
        background: ${style.background};
        border: ${style.border};
        color: ${style.color};
        padding: 10px 14px;
        border-radius: 10px;
        box-shadow: 0 2px 8px rgba(15, 23, 42, 0.06);
        min-width: 220px;
        max-width: 420px;
        pointer-events: auto;
        animation: slideInRight 0.25s ease-out;
        display: flex;
        align-items: center;
        gap: 10px;
        font-size: 0.875rem;
        line-height: 1.45;
        word-wrap: break-word;
        backdrop-filter: none;
        -webkit-backdrop-filter: none;
    `;

    toast.innerHTML = `
        <span style="color: ${style.iconColor}; flex-shrink: 0; display: flex; align-items: center;">${style.icon}</span>
        <span style="flex: 1;">${escapeHtml(message)}</span>
        <button onclick="this.parentElement.remove()" style="
            background: transparent;
            border: none;
            color: ${style.color};
            cursor: pointer;
            font-size: 1rem;
            padding: 0;
            margin-left: 4px;
            opacity: 0.45;
            flex-shrink: 0;
            width: 20px;
            height: 20px;
            display: flex;
            align-items: center;
            justify-content: center;
        " onmouseover="this.style.opacity='0.75'" onmouseout="this.style.opacity='0.45'">×</button>
    `;

    container.appendChild(toast);

    // 自动移除（成功消息显示5秒，错误消息显示7秒，其他显示4秒）
    const duration = type === 'success' ? 5000 : type === 'error' ? 7000 : 4000;
    setTimeout(() => {
        if (toast.parentElement) {
            toast.style.animation = 'slideOutRight 0.3s ease-out';
            setTimeout(() => {
                if (toast.parentElement) {
                    toast.remove();
                }
            }, 300);
        }
    }, duration);
}

// 添加CSS动画（如果不存在）
if (!document.getElementById('toast-notification-styles')) {
    const style = document.createElement('style');
    style.id = 'toast-notification-styles';
    style.textContent = `
        @keyframes slideInRight {
            from {
                transform: translateX(16px);
                opacity: 0;
            }
            to {
                transform: translateX(0);
                opacity: 1;
            }
        }
        @keyframes slideOutRight {
            from {
                transform: translateX(0);
                opacity: 1;
            }
            to {
                transform: translateX(16px);
                opacity: 0;
            }
        }
    `;
    document.head.appendChild(style);
}

// 通用工具函数
function getStatusText(status) {
    const s = (status && String(status).toLowerCase()) || '';
    if (typeof window.t !== 'function') {
        const fallback = { pending: '等待中', queued: '排队中', running: '执行中', background_running: '后台执行中', completed: '已完成', failed: '失败', blocked: '已拦截', cancelled: '已终止', hard_timeout: '硬超时', orphaned: '孤儿记录' };
        return fallback[s] || status;
    }
    const keyMap = { pending: 'mcpDetailModal.statusPending', queued: 'mcpDetailModal.statusQueued', running: 'mcpDetailModal.statusRunning', background_running: 'timeline.backgroundRunning', completed: 'mcpDetailModal.statusCompleted', failed: 'mcpDetailModal.statusFailed', blocked: 'mcpMonitor.statusBlocked', cancelled: 'mcpDetailModal.statusCancelled', hard_timeout: 'mcpMonitor.statusHardTimeout', orphaned: 'mcpMonitor.statusOrphaned' };
    const key = keyMap[s];
    return key ? window.t(key) : status;
}

function formatDuration(ms) {
    const seconds = Math.floor(ms / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);

    if (hours > 0) {
        return `${hours}小时${minutes % 60}分钟`;
    } else if (minutes > 0) {
        return `${minutes}分钟${seconds % 60}秒`;
    } else {
        return `${seconds}秒`;
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function escapeJsString(text) {
    return JSON.stringify(String(text == null ? '' : text));
}

function escapeAttr(text) {
    return escapeHtml(text).replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

function escapeJsStringAttr(text) {
    return escapeAttr(escapeJsString(text));
}

/** @param {string} text @param {{ profile?: 'chat'|'timeline' }} [options] */
function formatMarkdown(text, options) {
    if (typeof window.csMarkdownSanitize !== 'undefined') {
        return window.csMarkdownSanitize.formatMarkdownToHtml(text, options);
    }
    const raw = text == null ? '' : String(text);
    return escapeHtml(raw).replace(/\n/g, '<br>');
}

async function refreshAppData(showTaskErrors = false) {
    await Promise.allSettled([
        loadConversations(),
        loadActiveTasks(showTaskErrors),
    ]);
}

async function bootstrapApp() {
    if (!isAppInitialized) {
        // 等待 i18n 首包加载完成后再插系统就绪消息，避免清除缓存后语言显示 English 气泡仍是中文
        try {
            if (window.i18nReady && typeof window.i18nReady.then === 'function') {
                await window.i18nReady;
            }
        } catch (e) {
            console.warn('等待 i18n 就绪失败，继续初始化聊天', e);
        }
        initializeChatUI();
        isAppInitialized = true;
    }
    applyRBACToUI();
    await refreshAppData();
}

async function initializeApp() {
    applyRBACToUI();
    await bootstrapApp();
}

// 导出函数供HTML使用
window.hasPermission = hasPermission;
window.hasAnyPermission = hasAnyPermission;
window.readApiError = readApiError;
window.notifyApiError = notifyApiError;
window.notifyApiResponseError = notifyApiResponseError;
window.ensureApiOk = ensureApiOk;
window.requirePermission = requirePermission;
window.permissionAllowedForElement = permissionAllowedForElement;
window.applyRBACToUI = applyRBACToUI;
window.showNotification = showNotification;

function rbacAfterDynamicRender(root) {
    applyRBACToUI(root);
}

window.rbacAfterDynamicRender = rbacAfterDynamicRender;

document.addEventListener('DOMContentLoaded', initializeApp);
