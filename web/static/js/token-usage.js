// Token detail data belongs to the settings section; the dashboard keeps a
// separate fixed seven-day summary.
var tokenUsageState = { days: 7, controller: null, requestSequence: 0, result: null, trendMetric: 'totalTokens' };

const TOKEN_USAGE_TEXT = {
    "tokenUsageUI.chartHint": "悬停或聚焦柱形查看精确用量",
    "tokenUsageUI.scopeProject": "当前项目",
    "tokenUsageUI.scopeAccessible": "可访问会话",
    "tokenUsageUI.subtitle": "查看模型消耗、缓存利用与使用趋势",
    "tokenUsageUI.scopeNote": "统计口径与说明",
    "tokenUsageUI.trendMetric": "趋势指标",
    "tokenUsageUI.noDailyData": "暂无每日用量记录",
    "tokenUsageUI.cacheComposition": "输入缓存构成",
    "tokenUsageUI.cacheHit": "缓存命中率",
    "tokenUsageUI.uncachedInput": "未命中缓存的输入",
    "tokenUsageUI.cacheNote": "缓存读取是输入的一部分，不重复计入总量。缓存写入单独列示。",
    "dashboard.title": "仪表盘",
    "dashboard.refresh": "刷新",
    "dashboard.refreshData": "刷新数据",
    "dashboard.runningTasks": "运行中任务",
    "dashboard.runningConversations": "运行中对话",
    "dashboard.vulnTotal": "漏洞总数",
    "dashboard.toolCalls": "工具调用次数",
    "dashboard.successRate": "工具执行成功率",
    "dashboard.tokenUsage": "Token 用量",
    "dashboard.usageCacheHitRate": "缓存命中 {{rate}}",
    "dashboard.tokenUsageSub": "缓存命中 {{rate}}",
    "dashboard.noTokenUsageYet": "暂无用量",
    "dashboard.clickToViewTasks": "点击查看任务管理",
    "dashboard.clickToViewChat": "点击查看对话",
    "dashboard.clickToViewVuln": "点击查看漏洞管理",
    "dashboard.clickToViewMCP": "点击查看 MCP 监控",
    "dashboard.accessOverviewTitle": "接入概览",
    "dashboard.accessTabsAria": "C2 与 WebShell",
    "dashboard.c2OverviewTitle": "C2 概览",
    "dashboard.c2GoManage": "进入 C2 →",
    "dashboard.webshellGoManage": "进入 WebShell →",
    "dashboard.webshellConnections": "活跃连接",
    "dashboard.webshellClickConnections": "查看连接",
    "dashboard.c2ListenersRunning": "运行中监听器",
    "dashboard.c2SessionsOnline": "在线会话",
    "dashboard.c2TasksPending": "待审 / 排队任务",
    "dashboard.c2ClickListeners": "查看监听器",
    "dashboard.c2ClickSessions": "查看会话",
    "dashboard.c2ClickTasks": "查看任务",
    "dashboard.severityDistribution": "漏洞严重程度分布",
    "dashboard.severityCritical": "严重",
    "dashboard.severityHigh": "高危",
    "dashboard.severityMedium": "中危",
    "dashboard.severityLow": "低危",
    "dashboard.severityInfo": "信息",
    "dashboard.totalVulns": "总漏洞数",
    "dashboard.riskLevel": "风险等级",
    "dashboard.riskScore": "加权风险分",
    "dashboard.riskSafe": "安全",
    "dashboard.riskLow": "低",
    "dashboard.riskMedium": "中",
    "dashboard.riskHigh": "高",
    "dashboard.riskSevere": "极高",
    "dashboard.latestFound": "最近发现",
    "dashboard.noneYet": "暂无",
    "dashboard.runOverview": "运行概览",
    "dashboard.batchQueues": "批量任务队列",
    "dashboard.pending": "待执行",
    "dashboard.executing": "执行中",
    "dashboard.completed": "已完成",
    "dashboard.toolInvocations": "工具调用",
    "dashboard.callsUnit": "次调用",
    "dashboard.toolsUnit": "个工具",
    "dashboard.knowledgeLabel": "知识",
    "dashboard.knowledgeItems": "项知识",
    "dashboard.categoriesUnit": "个分类",
    "dashboard.skillsLabel": "Skills",
    "dashboard.skillUnit": "个 Skill",
    "dashboard.quickLinks": "快捷入口",
    "dashboard.toolsExecCount": "工具执行次数",
    "dashboard.ctaTitle": "开始你的安全之旅",
    "dashboard.ctaSub": "在对话中描述目标，AI 将协助执行扫描与漏洞分析",
    "dashboard.goToChat": "前往对话",
    "dashboard.noTasks": "暂无任务",
    "dashboard.totalCount": "共 {{count}} 个",
    "dashboard.notEnabled": "未启用",
    "dashboard.enabled": "已启用",
    "dashboard.toConfigure": "待配置",
    "dashboard.toUse": "待使用",
    "dashboard.active": "活跃",
    "dashboard.highFreq": "高频",
    "dashboard.noCallData": "暂无调用数据",
    "dashboard.severityClickHint": "点击查看",
    "dashboard.lastUpdated": "上次更新",
    "dashboard.viewAll": "查看全部 →",
    "dashboard.recentVulns": "最近漏洞",
    "dashboard.recentFacts": "近期事实",
    "dashboard.noVulnYet": "暂无最近漏洞",
    "dashboard.noFactsYet": "暂无近期事实",
    "dashboard.noFactsDesc": "在绑定项目的对话中，Agent 会自动记录目标、漏洞、攻击链等事实",
    "dashboard.createFirstProjectBtn": "创建第一个项目",
    "dashboard.factProjectMeta": "{{project}} · {{key}}",
    "dashboard.factsAcrossProjects": "{{count}} 个活跃项目 · {{facts}} 条事实",
    "dashboard.capabilities": "能力总览",
    "dashboard.mcpTools": "MCP 工具",
    "dashboard.rolesLabel": "角色",
    "dashboard.agentsLabel": "Agents",
    "dashboard.webshellLabel": "WebShell",
    "dashboard.pendingCountLabel": "{{count}} 待执行",
    "dashboard.highCountLabel": "高危 {{count}}",
    "dashboard.toolsCountLabel": "{{count}} 个工具",
    "dashboard.failedNCalls": "{{count}} 次失败",
    "dashboard.noCompletedYet": "暂无有效完成",
    "dashboard.noCallYet": "暂无调用",
    "dashboard.allClear": "暂无新增风险",
    "dashboard.allIdle": "系统空闲",
    "dashboard.executingNow": "正在执行",
    "dashboard.healthyStatus": "运行平稳",
    "dashboard.normalStatus": "基本正常",
    "dashboard.degradedStatus": "需要关注",
    "dashboard.alertTitle": "需要关注",
    "dashboard.alertWarningTitle": "需要关注",
    "dashboard.alertDangerTitle": "需要立即处理",
    "dashboard.alertCriticalReason": "存在 {{count}} 个待处理的严重漏洞，建议立即处置",
    "dashboard.alertFailedReason": "工具调用成功率偏低（{{count}} 次失败），请检查 MCP 监控",
    "dashboard.alertHitlReason": "有 {{count}} 个待审批的人机协同请求，Agent 正在等待你的决策",
    "dashboard.alertMcpDownReason": "External MCP 服务器有 {{count}} 个未运行，相关工具不可用",
    "dashboard.alertDismiss": "忽略此提醒（仅本次会话）",
    "dashboard.openHighCountLabel": "待处理高危 {{count}}",
    "dashboard.allHandled": "高严重度已全部处置",
    "dashboard.viewVulns": "查看漏洞",
    "dashboard.viewMonitor": "查看监控",
    "dashboard.viewHitl": "前往审批",
    "dashboard.viewMcpManagement": "管理 MCP",
    "dashboard.statusOpen": "待处理",
    "dashboard.statusConfirmed": "已确认",
    "dashboard.statusFixed": "已修复",
    "dashboard.statusFalsePositive": "误报",
    "dashboard.statusIgnored": "已忽略",
    "dashboard.statusFilter": "状态筛选",
    "dashboard.allStatuses": "全部状态",
    "dashboard.fixRate": "修复率",
    "dashboard.dataStale": "数据可能已过期，请手动刷新",
    "dashboard.recommendedActions": "推荐操作",
    "dashboard.recommendedActionsHint": "基于当前状态自动生成",
    "dashboard.recoFixCritical": "修复 {{count}} 个待处理严重漏洞",
    "dashboard.recoFixCriticalDesc": "严重等级的漏洞应优先处置",
    "dashboard.recoApproveHitl": "审批 {{count}} 个 HITL 请求",
    "dashboard.recoApproveHitlDesc": "Agent 正在等待你的决策才能继续",
    "dashboard.recoRestartMcp": "检查 {{count}} 个未运行的 External MCP",
    "dashboard.recoRestartMcpDesc": "相关工具在 MCP 服务恢复前不可用",
    "dashboard.recoCheckMonitor": "排查 {{count}} 次工具调用失败",
    "dashboard.recoCheckMonitorDesc": "在 MCP 监控中查看失败的请求详情",
    "dashboard.recoSetupMcp": "配置首个 MCP 工具",
    "dashboard.recoSetupMcpDesc": "安装 MCP 服务后 Agent 才能调用具体能力",
    "dashboard.recoStartScan": "在对话中发起扫描",
    "dashboard.recoStartScanDesc": "在对话中描述目标，让 AI 协助执行",
    "dashboard.recentEvents": "最近事件",
    "dashboard.eventUntitled": "事件",
    "dashboard.externalMcpServers": "External MCP",
    "dashboard.mcpAllRunning": "全部运行",
    "dashboard.mcpPartialDown": "{{count}} 个未运行",
    "dashboard.mcpAllDown": "全部未运行",
    "dashboard.noVulnDesc": "此处展示近期漏洞记录；在对话中完成检测后，新结果会出现在这里",
    "dashboard.startScanBtn": "前往对话发起扫描",
    "dashboard.usageView": "查看用量详情",
    "dashboard.usagePeriod": "统计周期",
    "dashboard.usage7Days": "近 7 天",
    "dashboard.usage30Days": "近 30 天",
    "dashboard.usage90Days": "近 90 天",
    "dashboard.usageTotal": "总 Token",
    "dashboard.usageInput": "输入 Token",
    "dashboard.usageOutput": "输出 Token",
    "dashboard.usageCacheRead": "缓存读取",
    "dashboard.usageCacheWrite": "缓存写入",
    "dashboard.usageReasoning": "推理 Token",
    "dashboard.usageCalls": "模型调用",
    "dashboard.usageCacheRate": "缓存读取占输入",
    "dashboard.usageByModel": "模型用量（最多 20 项）",
    "dashboard.usageByDay": "每日用量",
    "dashboard.usageRecent": "最近运行（最多 20 项）",
    "dashboard.usageDate": "日期",
    "dashboard.usageTime": "时间",
    "dashboard.usageModel": "模型",
    "dashboard.usageStatus": "结束原因",
    "dashboard.usageUnavailable": "用量统计暂不可用",
    "dashboard.usageScope": "统计已记录的 Agent 运行中模型返回的用量，包括子 Agent。缓存读取和写入属于输入，推理属于输出，不重复加入总量。未返回或未采集的用量不在统计内；这不是供应商账单。 模型按运行配置归类，切换模型的运行可能混合用量。",
    "dashboard.usageShare": "用量占比",
    "dashboard.usageTrend": "每日用量趋势",
    "dashboard.usageFinal": "已完成",
    "dashboard.usagePartial": "部分用量",
};

function tokenUsageText(key, opts, fallback) {
    let text = TOKEN_USAGE_TEXT[key];
    if (text) {
        if (opts) {
            text = text.replace(/\{\{(\w+)\}\}/g, (m, name) => (opts[name] !== undefined ? String(opts[name]) : m));
        }
        return text;
    }
    return fallback == null ? key : fallback;
}

function openDashboardTokenUsage() {
    if (window.location.hash === '#settings?section=token-usage') {
        switchPage('settings');
        switchSettingsSection('token-usage');
    } else {
        window.location.hash = 'settings?section=token-usage';
    }
}

function stopTokenUsageRequests() {
    tokenUsageState.requestSequence++;
    if (tokenUsageState.controller) tokenUsageState.controller.abort();
    tokenUsageState.controller = null;
}

async function refreshTokenUsage() {
    const host = document.getElementById('settings-token-analysis');
    if (!host) return;
    stopTokenUsageRequests();
    const sequence = tokenUsageState.requestSequence;
    const controller = new AbortController();
    tokenUsageState.controller = controller;
    host.setAttribute('aria-busy', 'true');
    host.textContent = tokenUsageText('common.loading', null, '加载中…');
    const select = document.getElementById('settings-token-days');
    if (select) select.value = String(tokenUsageState.days);
    let url = '/api/usage/tokens?days=' + tokenUsageState.days + '&limit=20';
    const scope = document.getElementById('settings-token-scope');
    if (scope) scope.textContent = tokenUsageText('tokenUsageUI.scopeAccessible', null, '可访问会话');
    try {
        const response = await apiFetch(url, { signal: controller.signal });
        const result = response && response.ok ? await response.json() : null;
        if (sequence !== tokenUsageState.requestSequence) return;
        tokenUsageState.result = result;
        renderTokenUsageAnalysis(result);
    } catch (_) {
        if (sequence !== tokenUsageState.requestSequence) return;
        tokenUsageState.result = null;
        renderTokenUsageAnalysis(null);
    } finally {
        if (sequence === tokenUsageState.requestSequence) {
            tokenUsageState.controller = null;
            host.removeAttribute('aria-busy');
        }
    }
}

function changeTokenUsagePeriod(value) {
    const days = Number(value);
    if (![7, 30, 90].includes(days)) return;
    tokenUsageState.days = days;
    refreshTokenUsage();
}

function tokenUsageCacheRate(summary) {
    const input = tokenUsageCount(summary && summary.promptTokens);
    const cached = tokenUsageCount(summary && summary.cachedTokens);
    return input > 0 ? (Math.min(cached, input) / input * 100).toFixed(1) + '%' : '—';
}

function tokenUsageCount(value) {
    const n = Number(value);
    return Number.isFinite(n) && n > 0 ? Math.trunc(n) : 0;
}

function tokenUsageMetrics(summary) {
    const input = tokenUsageCount(summary.promptTokens);
    const cached = tokenUsageCount(summary.cachedTokens);
    return [
        ['usageTotal', summary.totalTokens],
        ['usageInput', input],
        ['usageOutput', summary.completionTokens],
        ['usageCacheRead', cached],
        ['usageCacheWrite', summary.cacheWriteTokens],
        ['usageReasoning', summary.reasoningTokens],
        ['usageCalls', summary.modelCalls],
        ['usageCacheRate', tokenUsageCacheRate(summary)]
    ];
}

document.addEventListener('languagechange', function () {
    if (tokenUsageState.controller) return;
    const page = document.getElementById('page-settings');
    const section = document.getElementById('settings-section-token-usage');
    if (page && page.classList.contains('active') && section && section.classList.contains('active')) {
        renderTokenUsageAnalysis(tokenUsageState.result);
    }
});

function tokenUsageNode(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text != null) node.textContent = text;
    return node;
}

// Fill only gaps between returned dates. The API's server-local day keys are
// authoritative; do not invent a browser-timezone endpoint for the period.
function tokenUsageTimeline(rows) {
    const dated = (rows || []).filter(row => /^\d{4}-\d{2}-\d{2}$/.test(row.key || '')).sort((a, b) => a.key.localeCompare(b.key));
    if (!dated.length) return [];
    const byDay = new Map(dated.map(row => [row.key, row]));
    const start = Date.parse(dated[0].key + 'T00:00:00Z');
    const end = Date.parse(dated[dated.length - 1].key + 'T00:00:00Z');
    if (!Number.isFinite(start) || !Number.isFinite(end)) return dated;
    const days = [];
    for (let time = start; time <= end && days.length < 365; time += 86400000) {
        const key = new Date(time).toISOString().slice(0, 10);
        days.push(byDay.get(key) || { key, events: 0, totalTokens: 0, modelCalls: 0, promptTokens: 0 });
    }
    return days;
}

function setTokenUsageTrend(metric) {
    if (!['totalTokens', 'modelCalls', 'cacheRate'].includes(metric)) return;
    tokenUsageState.trendMetric = metric;
    renderTokenUsageAnalysis(tokenUsageState.result);
    const button = document.querySelector('[data-token-trend="' + metric + '"]');
    if (button) button.focus();
}

function renderTokenUsageAnalysis(res) {
    const host = document.getElementById('settings-token-analysis');
    if (!host) return;
    host.replaceChildren();
    const label = key => tokenUsageText('dashboard.' + key, null, key);
    const ui = key => tokenUsageText('tokenUsageUI.' + key, null, key);
    const exact = value => tokenUsageCount(value).toLocaleString(window.__locale || 'zh-CN');
    if (!res || !res.summary) {
        const empty = tokenUsageNode('div', 'token-usage-empty', label('usageUnavailable'));
        host.append(empty);
        return;
    }
    const summary = res.summary;
    const metrics = new Map(tokenUsageMetrics(summary));
    const primary = tokenUsageNode('div', 'token-usage-metrics token-usage-primary');
    const secondary = tokenUsageNode('div', 'token-usage-metrics token-usage-secondary');
    const primaryKeys = ['usageTotal', 'usageInput', 'usageOutput', 'usageCacheRate'];
    [...primaryKeys, 'usageCalls', 'usageCacheRead', 'usageCacheWrite', 'usageReasoning'].forEach(key => {
        const value = metrics.get(key);
        const card = tokenUsageNode('div', 'token-usage-metric');
        card.dataset.metric = key;
        card.append(tokenUsageNode('span', '', label(key)), tokenUsageNode('strong', '', typeof value === 'string' ? value : exact(value)));
        (primaryKeys.includes(key) ? primary : secondary).append(card);
    });
    host.append(primary, secondary);
    if (!tokenUsageCount(summary.events)) {
        host.append(tokenUsageNode('div', 'token-usage-empty', label('noTokenUsageYet')));
        return;
    }

    const chartLayout = tokenUsageNode('div', 'token-usage-charts');
    const chart = tokenUsageNode('section', 'token-usage-chart-card');
    const chartHead = tokenUsageNode('div', 'token-usage-chart-head');
    chartHead.append(tokenUsageNode('h4', '', label('usageTrend')));
    const tabs = tokenUsageNode('div', 'token-usage-chart-tabs');
    tabs.setAttribute('role', 'group');
    tabs.setAttribute('aria-label', ui('trendMetric'));
    for (const [metric, key] of [['totalTokens', 'usageTotal'], ['modelCalls', 'usageCalls'], ['cacheRate', 'usageCacheRate']]) {
        const button = tokenUsageNode('button', '', label(key));
        button.type = 'button';
        button.dataset.tokenTrend = metric;
        button.setAttribute('aria-pressed', String(tokenUsageState.trendMetric === metric));
        button.addEventListener('click', () => setTokenUsageTrend(metric));
        tabs.append(button);
    }
    chartHead.append(tabs);
    chart.append(chartHead);
    const tooltip = tokenUsageNode('p', 'token-usage-chart-tooltip', ui('chartHint'));
    tooltip.setAttribute('aria-live', 'polite');
    const days = tokenUsageTimeline(res.byDay);
    const metric = tokenUsageState.trendMetric;
    const valueOf = row => metric === 'cacheRate' ? (tokenUsageCount(row.promptTokens) ? Math.min(tokenUsageCount(row.cachedTokens), tokenUsageCount(row.promptTokens)) / tokenUsageCount(row.promptTokens) * 100 : 0) : tokenUsageCount(row[metric]);
    const peak = metric === 'cacheRate' ? 100 : Math.max(1, ...days.map(valueOf));
    const svgNS = 'http://www.w3.org/2000/svg';
    const svg = document.createElementNS(svgNS, 'svg');
    svg.setAttribute('viewBox', '0 0 640 230');
    svg.setAttribute('role', 'group');
    svg.setAttribute('aria-label', label('usageTrend'));
    svg.classList.add('token-usage-trend-svg');
    const element = (tag, attrs, text) => {
        const node = document.createElementNS(svgNS, tag);
        Object.entries(attrs).forEach(([key, value]) => node.setAttribute(key, String(value)));
        if (text != null) node.textContent = text;
        return node;
    };
    for (let tick = 0; tick <= 4; tick++) {
        const y = 18 + tick * 44;
        svg.append(element('line', { x1: 54, x2: 624, y1: y, y2: y, class: 'token-usage-chart-grid' }));
        svg.append(element('text', { x: 46, y: y + 4, 'text-anchor': 'end', class: 'token-usage-chart-label' }, metric === 'cacheRate' ? (100 - tick * 25) + '%' : formatTokenUsageCompact(peak * (4 - tick) / 4)));
    }
    const slot = 570 / Math.max(days.length, 1);
    const every = Math.max(1, Math.ceil(days.length / 7));
    days.forEach((row, index) => {
        const x = 54 + slot * (index + .5);
        const height = valueOf(row) / peak * 176;
        const detail = row.key + ' · ' + label('usageTotal') + ' ' + exact(row.totalTokens) + ' · ' + label('usageCalls') + ' ' + exact(row.modelCalls) + ' · ' + label('usageCacheRate') + ' ' + tokenUsageCacheRate(row);
        const bar = element('rect', { x: x - Math.min(24, slot * .55) / 2, y: 194 - height, width: Math.min(24, slot * .55), height: Math.max(height, 1), rx: 3, class: 'token-usage-chart-bar', tabindex: 0, 'aria-label': detail });
        bar.append(element('title', {}, detail));
        for (const event of ['mouseenter', 'focus']) bar.addEventListener(event, () => { tooltip.textContent = detail; });
        for (const event of ['mouseleave', 'blur']) bar.addEventListener(event, () => { tooltip.textContent = ui('chartHint'); });
        svg.append(bar);
        if (index % every === 0 || index === days.length - 1) {
            svg.append(element('text', { x, y: 218, 'text-anchor': 'middle', class: 'token-usage-chart-label' }, row.key.slice(5)));
        }
    });
    const plot = tokenUsageNode('div', 'token-usage-svg-wrap');
    plot.append(svg);
    chart.append(plot);
    chart.append(tooltip);
    if (!days.length) chart.append(tokenUsageNode('p', 'token-usage-empty', ui('noDailyData')));
    chartLayout.append(chart);

    const cache = tokenUsageNode('section', 'token-usage-chart-card token-usage-cache-card');
    cache.append(tokenUsageNode('h4', '', ui('cacheComposition')));
    const input = tokenUsageCount(summary.promptTokens);
    const cached = Math.min(input, tokenUsageCount(summary.cachedTokens));
    const ratio = input ? cached / input * 100 : 0;
    const ring = tokenUsageNode('div', 'token-usage-cache-ring');
    ring.style.setProperty('--cache-ratio', ratio + '%');
    ring.append(tokenUsageNode('strong', '', tokenUsageCacheRate(summary)), tokenUsageNode('span', '', ui('cacheHit')));
    cache.append(ring);
    for (const [key, value, className] of [['usageCacheRead', cached, 'is-cached'], ['uncachedInput', input - cached, 'is-uncached']]) {
        const line = tokenUsageNode('div', 'token-usage-cache-legend ' + className);
        line.append(tokenUsageNode('span', '', key === 'uncachedInput' ? ui(key) : label(key)), tokenUsageNode('strong', '', exact(value)));
        cache.append(line);
    }
    cache.append(tokenUsageNode('p', 'token-usage-chart-note', ui('cacheNote')));
    chartLayout.append(cache);
    host.append(chartLayout);

    // Provider labels are untrusted: render all values as text or constructed
    // nodes, never as HTML. Full precision is retained in every table.
    const table = (titleKey, headings, rows, collapsed = false) => {
        const section = tokenUsageNode(collapsed ? 'details' : 'section', 'token-usage-data-card');
        const title = tokenUsageNode(collapsed ? 'summary' : 'h4', '', label(titleKey));
        const wrap = tokenUsageNode('div', 'token-usage-table-wrap');
        const grid = tokenUsageNode('table');
        const caption = tokenUsageNode('caption', 'sr-only', label(titleKey));
        const head = tokenUsageNode('thead');
        const hr = tokenUsageNode('tr');
        headings.forEach(key => { const th = tokenUsageNode('th', '', label(key)); th.scope = 'col'; hr.append(th); });
        head.append(hr);
        const body = tokenUsageNode('tbody');
        rows.forEach(values => {
            const tr = tokenUsageNode('tr');
            values.forEach(value => {
                const td = tokenUsageNode('td');
                if (value && value.nodeType) td.append(value);
                else td.textContent = String(value == null ? '' : value);
                tr.append(td);
            });
            body.append(tr);
        });
        grid.append(caption, head, body);
        wrap.append(grid);
        section.append(title, wrap);
        host.append(section);
    };
    const counts = row => [exact(row.promptTokens), exact(row.completionTokens), exact(row.cachedTokens), exact(row.cacheWriteTokens), exact(row.reasoningTokens), exact(row.totalTokens), exact(row.modelCalls)];
    const fields = ['usageInput', 'usageOutput', 'usageCacheRead', 'usageCacheWrite', 'usageReasoning', 'usageTotal', 'usageCalls'];
    table('usageByModel', ['usageModel', 'usageShare', ...fields], (res.byModel || []).map(row => {
        const ratio = tokenUsageCount(summary.totalTokens) ? tokenUsageCount(row.totalTokens) / tokenUsageCount(summary.totalTokens) * 100 : 0;
        const share = tokenUsageNode('div', 'token-usage-share');
        share.append(tokenUsageNode('span', '', ratio.toFixed(1) + '%'));
        const track = tokenUsageNode('span', 'token-usage-share-track');
        const fill = tokenUsageNode('span'); fill.style.width = Math.min(100, ratio) + '%'; track.append(fill); share.append(track);
        return [row.label || row.key, share, ...counts(row)];
    }));
    table('usageByDay', ['usageDate', ...fields], (res.byDay || []).map(row => [row.key, ...counts(row)]), true);
    table('usageRecent', ['usageTime', 'usageModel', 'usageStatus', 'usageTotal', 'usageCalls'], (res.recent || []).map(row => {
        const time = new Date(row.createdAt);
        const status = tokenUsageNode('span', 'token-usage-status', row.reason === 'final' ? label('usageFinal') : row.reason === 'partial' ? label('usagePartial') : (row.reason || '—'));
        if (row.reason === 'final' || row.reason === 'partial') status.classList.add('is-' + row.reason);
        return [Number.isFinite(time.getTime()) ? time.toLocaleString(window.__locale || 'zh-CN') : '—', row.model || '—', status, exact(row.totalTokens), exact(row.modelCalls)];
    }));
}

// 运行日志：读取 log/ 目录下日志文件的末尾内容（系统设置 → 用量与运维）。
function refreshRuntimeLogs() {
    const output = document.getElementById('runtime-log-output');
    const picker = document.getElementById('runtime-log-file');
    if (!output) return;
    const file = picker && picker.value ? '&file=' + encodeURIComponent(picker.value) : '';
    output.textContent = '加载中…';
    fetch('/api/system/logs?lines=300' + file, { credentials: 'same-origin' })
        .then((resp) => resp.json().then((body) => ({ ok: resp.ok, body })))
        .then(({ ok, body }) => {
            if (!ok) {
                output.textContent = '读取失败：' + ((body && body.error) || '未知错误');
                return;
            }
            if (picker && Array.isArray(body.files)) {
                picker.innerHTML = body.files
                    .map((name) => '<option value="' + name + '"' + (name === body.file ? ' selected' : '') + '>' + name + '</option>')
                    .join('');
            }
            const lines = Array.isArray(body.lines) ? body.lines : [];
            output.textContent = lines.length
                ? (body.truncated ? '仅显示最近 ' + lines.length + ' 行\n' : '') + lines.join('\n')
                : '暂无日志';
        })
        .catch((err) => {
            output.textContent = '读取失败：' + (err && err.message ? err.message : err);
        });
}

document.addEventListener('DOMContentLoaded', function () {
    refreshRuntimeLogs();
});