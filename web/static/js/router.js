// 页面路由管理
let currentPage = null;

/** 允许直接通过 hash 进入的页面白名单。 */
const ROUTABLE_PAGES = [
    'dashboard', 'chat', 'tool-guard', 'mcp-monitor', 'mcp-management',
    'vulnerabilities', 'roles-management', 'skills-monitor', 'skills-management', 'settings',
];

/** 对话、漏洞管理和设置页切换时保留查询串（会话 ID 或设置分区）。 */
function buildHashForPage(pageId) {
    if (pageId !== 'chat' && pageId !== 'vulnerabilities' && pageId !== 'settings') {
        return pageId;
    }
    const full = window.location.hash.slice(1);
    const parts = full.split('?');
    const curPage = parts[0];
    const q = parts.length > 1 ? parts.slice(1).join('?') : '';
    if (curPage === pageId && q) {
        return pageId + '?' + q;
    }
    return pageId;
}

let chatConversationFromHashSeq = 0;

function cancelScheduledChatConversationFromHash() {
    chatConversationFromHashSeq++;
    setChatConversationRestorePending('', false);
}
window.cancelScheduledChatConversationFromHash = cancelScheduledChatConversationFromHash;

function setChatConversationRestorePending(conversationId, pending) {
    const container = document.querySelector('.chat-container');
    if (!container) return;
    const id = String(conversationId || '').trim();
    if (pending && id) {
        container.classList.add('is-conversation-restoring');
        container.dataset.restoringConversationId = id;
        container.setAttribute('aria-busy', 'true');
        return;
    }
    container.classList.remove('is-conversation-restoring');
    delete container.dataset.restoringConversationId;
    container.removeAttribute('aria-busy');
}

function finishChatConversationRestore(conversationId) {
    setChatConversationRestorePending(conversationId, false);
}
window.finishChatConversationRestore = finishChatConversationRestore;

function scheduleChatConversationFromHash(delayMs) {
    const hash = window.location.hash.slice(1);
    const hashParts = hash.split('?');
    if (hashParts[0] !== 'chat' || hashParts.length < 2) {
        return;
    }
    const params = new URLSearchParams(hashParts.slice(1).join('?'));
    const conversationId = params.get('conversation');
    if (!conversationId) {
        return;
    }
    // 同一事件循环内先遮住默认新对话状态，避免网络请求返回前闪出空对话。
    setChatConversationRestorePending(conversationId, true);
    const token = ++chatConversationFromHashSeq;
    setTimeout(() => {
        if (token !== chatConversationFromHashSeq) {
            return;
        }
        if (typeof loadConversation === 'function') {
            loadConversation(conversationId);
        } else if (typeof window.loadConversation === 'function') {
            window.loadConversation(conversationId);
        } else {
            console.warn('loadConversation function not found');
        }
    }, delayMs);
}

/** 跳转到指定对话：单次切页 + 单次加载，避免 hashchange 与手动 load 重复触发导致闪烁 */
function navigateToConversation(conversationId) {
    const cid = String(conversationId || '').trim();
    if (!cid) return;
    const targetHash = 'chat?conversation=' + encodeURIComponent(cid);
    const alreadyOnChat = currentPage === 'chat';

    if (window.location.hash.slice(1) !== targetHash) {
        history.replaceState(null, '', '#' + targetHash);
    }

    if (!alreadyOnChat) {
        switchPage('chat');
    }

    if (typeof loadConversation === 'function') {
        return loadConversation(cid);
    } else if (typeof window.loadConversation === 'function') {
        return window.loadConversation(cid);
    }
}
window.navigateToConversation = navigateToConversation;

/** 解析 hash 中的页面与设置分区。 */
function resolveRoute(hash) {
    const hashParts = String(hash || '').split('?');
    let pageId = hashParts[0].replace(/^#?\//, '');
    const query = new URLSearchParams(hashParts.slice(1).join('?'));
    const settingsSection = pageId === 'settings' && query.get('section') === 'token-usage' ? 'token-usage' : '';
    if (!ROUTABLE_PAGES.includes(pageId)) {
        pageId = '';
    }
    return { pageId, settingsSection };
}

// 初始化路由
function initRouter() {
    // 从URL hash读取页面（如果有）
    const { pageId, settingsSection } = resolveRoute(window.location.hash.slice(1));
    if (pageId) {
        switchPage(pageId);
        if (settingsSection && typeof switchSettingsSection === 'function') {
            switchSettingsSection(settingsSection);
        }
        if (pageId === 'chat') {
            scheduleChatConversationFromHash(0);
        }
        return;
    }

    // 默认显示仪表盘
    switchPage('dashboard');
}

// 切换页面
function switchPage(pageId) {
    const targetPage = document.getElementById(`page-${pageId}`);
    if (!targetPage) return;
    if (pageId !== 'chat') {
        setChatConversationRestorePending('', false);
        if (currentPage === 'chat' && typeof window.abandonChatConversationForPageNavigation === 'function') {
            window.abandonChatConversationForPageNavigation();
        }
    }

    // 导航点击会修改 hash，随后浏览器还会触发 hashchange。
    // 同一页面已经激活时不再重复初始化，避免接口重复请求和页面二次重绘。
    if (currentPage === pageId && targetPage.classList.contains('active')) {
        const currentHash = buildHashForPage(pageId);
        if (window.location.hash.slice(1) !== currentHash) {
            window.location.hash = currentHash;
        }
        return;
    }

    // 隐藏所有页面
    document.querySelectorAll('.page').forEach(page => {
        page.classList.remove('active');
    });

    // 显示目标页面
    targetPage.classList.add('active');
    currentPage = pageId;

    const newHash = buildHashForPage(pageId);
    if (window.location.hash.slice(1) !== newHash) {
        window.location.hash = newHash;
    }

    // 更新导航状态
    updateNavState(pageId);

    // 页面特定的初始化
    initPage(pageId);

    if (typeof applyRBACToUI === 'function') {
        applyRBACToUI(targetPage);
    }
}
window.switchPage = switchPage;

// 更新导航状态（扁平导航：每个页面直接对应一个导航项）
function updateNavState(pageId) {
    const isTokenSection = String(window.location.hash || '').includes('section=token-usage');
    document.querySelectorAll('.nav-item').forEach((item) => {
        if (item.dataset.page !== pageId) {
            item.classList.remove('active');
            return;
        }
        // 设置页拆成两个导航项，按当前分区决定高亮哪一个
        if (pageId === 'settings' && item.dataset.settingsSection) {
            const wanted = isTokenSection ? 'token-usage' : 'basic';
            item.classList.toggle('active', item.dataset.settingsSection === wanted);
            return;
        }
        item.classList.add('active');
    });
}

/** 打开设置页并定位到指定分区（侧边栏两个导航项共用 settings 页） */
function openSettingsSection(section) {
    const target = section === 'token-usage' ? 'token-usage' : 'basic';
    window.__pendingSettingsSection = target;
    switchPage('settings');
    if (window.currentPage && window.currentPage() === 'settings') {
        if (typeof switchSettingsSection === 'function') switchSettingsSection(target);
    }
}
window.openSettingsSection = openSettingsSection;

// 初始化页面
async function initPage(pageId) {
    // 等待 i18n 就绪，避免快速刷新时翻译函数未初始化导致页面显示原始占位符 key
    if (window.i18nReady) await window.i18nReady;
    if (typeof stopExternalMcpPoll === 'function') {
        stopExternalMcpPoll();
    }
    switch(pageId) {
        case 'dashboard':
            if (typeof refreshDashboard === 'function') {
                refreshDashboard();
            }
            break;
        case 'chat':
            // 恢复对话列表折叠状态（从其他页返回时保持用户选择）
            initConversationSidebarState();
            break;
        case 'tool-guard':
            if (typeof loadToolGuardConfig === 'function') loadToolGuardConfig();
            break;
        case 'mcp-monitor':
            // 初始化监控面板
            if (typeof refreshMonitorPanel === 'function') {
                refreshMonitorPanel();
            }
            if (typeof startMonitorPoll === 'function') {
                startMonitorPoll();
            }
            break;
        case 'mcp-management':
            // 初始化MCP管理
            const startLoadMcpTools = () => {
                // 加载工具列表（MCP工具配置已移到MCP管理页面）
                // 使用异步加载，避免阻塞页面渲染
                if (typeof loadToolsList === 'function') {
                    // 确保工具分页设置已初始化
                    if (typeof getToolsPageSize === 'function' && typeof toolsPagination !== 'undefined') {
                        toolsPagination.pageSize = getToolsPageSize();
                    }
                    // 延迟加载，让页面先渲染
                    setTimeout(() => {
                        loadToolsList(1, '').catch(err => {
                            console.error('加载工具列表失败:', err);
                        });
                    }, 100);
                }
            };
            const afterMcpConfigReady = () => {
                startLoadMcpTools();
                if (typeof loadExternalMCPs === 'function') {
                    loadExternalMCPs().catch(err => {
                        console.warn('加载外部MCP列表失败:', err);
                    });
                }
                if (typeof startExternalMcpPoll === 'function') {
                    startExternalMcpPoll();
                }
            };
            // 先拉取配置（含 tool_search 常驻列表），再加载工具与外部 MCP。
            // 仅有 mcp:read 的用户无需拉全量 /api/config（需 config:read）。
            const canLoadFullConfig = typeof hasPermission !== 'function' || hasPermission('config:read');
            if (typeof loadConfig === 'function' && canLoadFullConfig) {
                loadConfig(false, { silent: true })
                    .catch(err => {
                        console.warn('加载配置失败（将继续加载 MCP 列表）:', err);
                    })
                    .finally(afterMcpConfigReady);
            } else {
                afterMcpConfigReady();
            }
            break;
        case 'vulnerabilities':
            // 初始化漏洞管理页面
            if (typeof initVulnerabilityPage === 'function') {
                initVulnerabilityPage();
            }
            break;
        case 'settings':
            // 初始化设置页面（不需要加载工具列表）
            if (typeof loadConfig === 'function') {
                loadConfig(false);
            }
            // Returning through the regular settings nav reloads a retained usage panel.
            // Deep links are loaded by switchSettingsSection after switchPage returns.
            const tokenSection = document.getElementById('settings-section-token-usage');
            if (tokenSection && tokenSection.classList.contains('active') && window.location.hash !== '#settings?section=token-usage' && typeof refreshTokenUsage === 'function') {
                refreshTokenUsage();
            }
            break;
        case 'roles-management':
            // 初始化角色管理页面
            // 重置搜索UI（变量会在下次搜索时自动更新）
            const rolesSearchInput = document.getElementById('roles-search');
            if (rolesSearchInput) {
                rolesSearchInput.value = '';
            }
            const rolesSearchClear = document.getElementById('roles-search-clear');
            if (rolesSearchClear) {
                rolesSearchClear.style.display = 'none';
            }
            if (typeof loadRoles === 'function') {
                loadRoles().then(() => {
                    if (typeof renderRolesList === 'function') {
                        renderRolesList();
                    }
                });
            }
            break;
        case 'skills-monitor':
            // 初始化Skills状态监控页面
            if (typeof loadSkillsMonitor === 'function') {
                loadSkillsMonitor();
            }
            break;
        case 'skills-management':
            // 初始化Skills管理页面
            // 重置搜索UI（变量会在下次搜索时自动更新）
            const skillsSearchInput = document.getElementById('skills-search');
            if (skillsSearchInput) {
                skillsSearchInput.value = '';
            }
            const skillsSearchClear = document.getElementById('skills-search-clear');
            if (skillsSearchClear) {
                skillsSearchClear.style.display = 'none';
            }
            if (typeof initSkillsPagination === 'function') {
                initSkillsPagination();
            }
            if (typeof loadSkills === 'function') {
                loadSkills();
            }
            break;
    }

    if (pageId !== 'settings' && typeof stopTokenUsageRequests === 'function') {
        stopTokenUsageRequests();
    }
}

// 页面加载完成后初始化路由
document.addEventListener('DOMContentLoaded', function() {
    initRouter();
    document.documentElement.classList.remove('initial-route-pending');
    initSidebarState();

    // 监听hash变化
    window.addEventListener('hashchange', function() {
        const { pageId, settingsSection } = resolveRoute(window.location.hash.slice(1));
        if (pageId) {
            switchPage(pageId);
            if (settingsSection && typeof switchSettingsSection === 'function') {
                switchSettingsSection(settingsSection);
            }
            if (pageId === 'chat') {
                scheduleChatConversationFromHash(0);
            }
        }
    });
});

// 切换侧边栏折叠/展开
function toggleSidebar() {
    const sidebar = document.getElementById('main-sidebar');
    if (sidebar) {
        sidebar.classList.toggle('collapsed');
        // 保存折叠状态到localStorage
        const isCollapsed = sidebar.classList.contains('collapsed');
        localStorage.setItem('sidebarCollapsed', isCollapsed ? 'true' : 'false');
    }
}
window.toggleSidebar = toggleSidebar;

// 初始化侧边栏状态
function initSidebarState() {
    const sidebar = document.getElementById('main-sidebar');
    if (sidebar) {
        const savedState = localStorage.getItem('sidebarCollapsed');
        if (savedState === 'true') {
            sidebar.classList.add('collapsed');
        }
    }
    initConversationSidebarState();
}

// 切换对话页左侧列表折叠/展开
function toggleConversationSidebar() {
    const sidebar = document.getElementById('conversation-sidebar');
    if (sidebar) {
        sidebar.classList.toggle('collapsed');
        const isCollapsed = sidebar.classList.contains('collapsed');
        localStorage.setItem('conversationSidebarCollapsed', isCollapsed ? 'true' : 'false');
    }
}
window.toggleConversationSidebar = toggleConversationSidebar;

// 恢复对话列表折叠状态（进入对话页时生效）
function initConversationSidebarState() {
    const sidebar = document.getElementById('conversation-sidebar');
    if (sidebar) {
        const savedState = localStorage.getItem('conversationSidebarCollapsed');
        if (savedState === 'true') {
            sidebar.classList.add('collapsed');
        } else {
            sidebar.classList.remove('collapsed');
        }
    }
}

// 导出函数供其他脚本使用（与上方尽早绑定保持一致，便于外部脚本探测）
window.currentPage = function() { return currentPage; };
