import './style.css';
import { api } from './api';
import { t, setLang, getLang } from './i18n';
import { WindowCenter, WindowIsMaximised, WindowSetMinSize, WindowSetSize, WindowUnmaximise } from '../wailsjs/runtime/runtime';
import type { AppState, LogEntry, ProjectPreset, ShareMode, StackServiceStatus, TunnelStatus } from './types';

type NoticeTone = 'info' | 'success' | 'error';

interface Notice {
  tone: NoticeTone;
  message: string;
}

interface UIState {
  appState: AppState | null;
  fatalError: string | null;
  selectedProjectId: string | null;
  activeProjectId: string | null;
  projectUrls: Record<string, string>;
  copiedProjectId: string | null;
  activeTab: 'overview' | 'settings' | 'logs' | 'about';
  projectMenuOpen: boolean;
  editorOpen: boolean;
  editorMode: 'create' | 'edit';
  editorProject: ProjectPreset;
  notice: Notice | null;
  busy: string | null;
  licenseDraft: string;
  projectSearch: string;
  stackStatuses: StackServiceStatus[];
  stackDraft: {
    nginxBinaryPath: string;
    phpCgiBinaryPath: string;
    mysqldBinaryPath: string;
    nginxPort: number;
    phpPort: number;
    mysqlPort: number;
  };
}

const blankProject = (): ProjectPreset => ({
  id: '',
  displayName: '',
  localHost: '',
  originURL: '',
  subdomain: '',
  publicURL: '',
  projectPath: '',
  localURL: '',
  startCommand: '',
  shareMode: 'auto',
});

const blankStackDraft = () => ({
  nginxBinaryPath: '',
  phpCgiBinaryPath: '',
  mysqldBinaryPath: '',
  nginxPort: 8090,
  phpPort: 9000,
  mysqlPort: 3306,
});

const state: UIState = {
  appState: null,
  fatalError: null,
  selectedProjectId: null,
  activeProjectId: null,
  projectUrls: {},
  copiedProjectId: null,
  activeTab: 'overview',
  projectMenuOpen: false,
  editorOpen: false,
  editorMode: 'create',
  editorProject: blankProject(),
  notice: null,
  busy: null,
  licenseDraft: '',
  projectSearch: '',
  stackStatuses: [],
  stackDraft: blankStackDraft(),
};

const rootElement = document.querySelector<HTMLDivElement>('#app');
if (!rootElement) {
  throw new Error('App root not found');
}
const root = rootElement;
const COMPACT_WINDOW_WIDTH = 1040;
const COMPACT_WINDOW_HEIGHT = 700;
const COMPACT_WINDOW_MIN_WIDTH = 900;
const COMPACT_WINDOW_MIN_HEIGHT = 620;
let noticeTimer: number | null = null;
let copyLabelTimer: number | null = null;
let currentTheme: 'dark' | 'light' = 'dark';

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;');
}

function languageFlagIcon(lang: string): string {
  if (lang === 'km') {
    return `
      <svg class="flag-icon" viewBox="0 0 36 24" role="img" aria-label="Khmer flag" xmlns="http://www.w3.org/2000/svg">
        <rect width="36" height="24" rx="3" fill="#032ea1"/>
        <rect y="6" width="36" height="12" fill="#e00025"/>
        <path d="M12 16.4h12v1.3H12v-1.3Zm1-1.5h10v1.1H13v-1.1Zm1.2-2.6h7.6v2.2h-7.6v-2.2Zm1.1-2h1.3v1.7h-1.3v-1.7Zm2.1-.9h1.2V12h-1.2V9.4Zm2 .9h1.3V12h-1.3v-1.7Zm-4.8 1.2h6.8l-3.4-2.7-3.4 2.7Z" fill="#fff"/>
      </svg>
    `;
  }

  return `
    <svg class="flag-icon" viewBox="0 0 36 24" role="img" aria-label="English flag" xmlns="http://www.w3.org/2000/svg">
      <rect width="36" height="24" rx="3" fill="#012169"/>
      <path d="M0 0h4.1L36 20.2V24h-4.1L0 3.8V0Zm36 0v3.8L4.1 24H0v-3.8L31.9 0H36Z" fill="#fff"/>
      <path d="M36 0 0 24h2.8L36 1.8V0ZM0 0l36 24h-2.8L0 1.8V0Z" fill="#c8102e"/>
      <path d="M15 0h6v24h-6V0ZM0 9h36v6H0V9Z" fill="#fff"/>
      <path d="M16.2 0h3.6v24h-3.6V0ZM0 10.2h36v3.6H0v-3.6Z" fill="#c8102e"/>
    </svg>
  `;
}

function nextLanguage(): 'en' | 'km' {
  return getLang() === 'en' ? 'km' : 'en';
}

function setTheme(theme: string) {
  currentTheme = theme === 'light' ? 'light' : 'dark';
  document.documentElement.dataset.theme = currentTheme;
}

function nextTheme(): 'dark' | 'light' {
  return currentTheme === 'dark' ? 'light' : 'dark';
}

function themeIcon(theme: 'dark' | 'light'): string {
  if (theme === 'dark') {
    return `
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true" xmlns="http://www.w3.org/2000/svg">
        <path d="M21 14.6A7.5 7.5 0 0 1 9.4 3 9 9 0 1 0 21 14.6Z" fill="currentColor"/>
      </svg>
    `;
  }

  return `
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true" xmlns="http://www.w3.org/2000/svg">
      <circle cx="12" cy="12" r="4" fill="currentColor"/>
      <path d="M12 2v2M12 20v2M4.93 4.93l1.42 1.42M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.42-1.41M17.66 6.34l1.41-1.41" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
    </svg>
  `;
}

function formatProjectURL(project: ProjectPreset, domain: string): string {
  if (project.publicURL) return project.publicURL;
  if (project.subdomain && isMeaningfulDomain(domain)) return `https://${project.subdomain}.${domain}`;
  return '';
}

function usesEphemeralURL(project: ProjectPreset): boolean {
  return project.shareMode === 'auto' || project.shareMode === 'quick' || project.shareMode === 'host-html';
}

function isMeaningfulDomain(domain: string): boolean {
  const normalized = domain.trim().toLowerCase();
  return normalized !== '' && normalized !== 'example.com';
}

function resolvedProjectURL(project: ProjectPreset, appState: AppState): string {
  const isActiveProject = state.activeProjectId === project.id;
  const liveUrl = isActiveProject
    ? (appState.status.activeUrl || appState.status.quickUrl || '').trim()
    : '';
  if (liveUrl) return liveUrl;

  if (usesEphemeralURL(project)) {
    return '';
  }

  const cachedUrl = state.projectUrls[project.id]?.trim() || '';
  if (cachedUrl) return cachedUrl;

  return formatProjectURL(project, appState.settings.defaultDomain).trim();
}

function shareActionForProject(project: ProjectPreset): { action: string; label: string } {
  switch (project.shareMode) {
    case 'auto':
      return { action: 'share-quick', label: t('startAutoShare') };
    case 'quick':
      return { action: 'share-quick', label: t('createPublicUrl') };
    case 'host-html':
      return { action: 'share-quick', label: t('createHtmlUrl') };
    default:
      return { action: 'share-quick', label: t('createPublicUrl') };
  }
}

function looksLikeURL(value: string): boolean {
  return /^https?:\/\//i.test(value.trim());
}

function inferLocalHostFromPath(projectPath: string): string {
  const normalized = projectPath.trim().replace(/[/\\]+$/, '');
  if (!normalized || looksLikeURL(normalized)) return '';

  const segments = normalized.split(/[/\\]+/);
  const folderName = segments[segments.length - 1]?.trim() ?? '';
  if (!folderName) return '';

  const hostname = folderName
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

  if (!hostname) return '';
  return `${hostname}.test`;
}

function projectTypeLabel(project: ProjectPreset): string {
  switch (project.shareMode) {
    case 'auto':
      return t('autoProject');
    case 'host-html':
      return t('htmlProject');
    default:
      return t('localHostProject');
  }
}

function projectPrimaryTarget(project: ProjectPreset): string {
  if (project.localURL.trim()) return project.localURL.trim();
  if (looksLikeURL(project.projectPath)) return project.projectPath.trim();
  if (project.shareMode === 'host-html') return t('htmlSiteFolder');
  if (project.localHost.trim()) return project.localHost.trim();
  if (project.startCommand.trim()) return `${t('autoVia')} ${project.startCommand.trim()}`;
  return project.projectPath.trim() || t('projectSource');
}

function randomSubdomainValue(): string {
  return Math.random().toString(36).slice(2, 8);
}

function selectedProject(): ProjectPreset | null {
  const projects = state.appState?.settings.projects ?? [];
  if (projects.length === 0) return null;
  if (!state.selectedProjectId) return projects[0];
  return projects.find((project) => project.id === state.selectedProjectId) ?? projects[0];
}

function syncProjectUrlsFromState(appState: AppState) {
  for (const project of appState.settings.projects) {
    if (usesEphemeralURL(project)) {
      delete state.projectUrls[project.id];
      continue;
    }
    if (project.publicURL?.trim()) {
      state.projectUrls[project.id] = project.publicURL.trim();
    }
  }
}

function inferActiveProjectId(appState: AppState): string | null {
  const liveUrl = (appState.status.activeUrl || appState.status.quickUrl || '').trim();
  if (!appState.status.running) return null;

  if (liveUrl) {
    const matchedProject = appState.settings.projects.find((project) => {
      const knownUrl = (state.projectUrls[project.id] || project.publicURL || '').trim();
      return knownUrl === liveUrl;
    });
    if (matchedProject) return matchedProject.id;
  }

  return state.selectedProjectId || appState.settings.projects[0]?.id || null;
}

function setNotice(tone: NoticeTone, message: string) {
  state.notice = { tone, message };
  if (noticeTimer !== null) {
    window.clearTimeout(noticeTimer);
  }
  noticeTimer = window.setTimeout(() => {
    state.notice = null;
    noticeTimer = null;
    render();
  }, 3000);
  render();
}

function setBusy(label: string | null) {
  state.busy = label;
  render();
}

function shareModeLabel(mode: ShareMode): string {
  switch (mode) {
    case 'auto':
      return 'Auto';
    case 'host-html':
      return t('htmlSite');
    default:
      return t('publicUrl');
  }
}

function statusTone(status: TunnelStatus): string {
  if (status.running) return 'success';
  if (status.lastError) return 'error';
  return 'idle';
}

function formatUptime(seconds: number): string {
  if (!isFinite(seconds) || seconds <= 0) return '0s';
  const s = Math.floor(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  if (h > 0) return `${h}h ${m}m ${sec}s`;
  if (m > 0) return `${m}m ${sec}s`;
  return `${sec}s`;
}

function formatNumber(n: number): string {
  if (!isFinite(n) || n === 0) return '0';
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
  return Math.round(n).toString();
}

function renderUsagePanel(status: TunnelStatus): string {
  if (!status.running) return '';

  const usage = status.usage;
  if (!usage || !usage.available) {
    return `
      <div class="usage-panel usage-panel-pending">
        <div class="usage-panel-header">
          <span class="eyebrow">${t('liveTunnelUsage')}</span>
          <span class="usage-badge">${t('warmingUp')}</span>
        </div>
        <p class="usage-hint">${t('warmingUpHint')}</p>
      </div>
    `;
  }

  const codes = usage.responsesByCode || {};
  const codeEntries = Object.entries(codes)
    .filter(([, v]) => (v || 0) > 0)
    .sort((a, b) => a[0].localeCompare(b[0]));

  const codesHtml = codeEntries.length
    ? codeEntries
        .map(([code, count]) => {
          const tone = code.startsWith('2')
            ? 'ok'
            : code.startsWith('3')
              ? 'info'
              : code.startsWith('4')
                ? 'warn'
                : code.startsWith('5')
                  ? 'err'
                  : 'muted';
          return `<span class="usage-code usage-code-${tone}" title="HTTP ${escapeHtml(code)}">${escapeHtml(code)} · ${formatNumber(count)}</span>`;
        })
        .join('')
    : `<span class="usage-code usage-code-muted">${t('noResponsesYet')}</span>`;

  const edge = (usage.edgeLocations || []).filter(Boolean);
  const edgeLabel = edge.length ? edge.join(', ') : '—';

  return `
    <div class="usage-panel">
      <div class="usage-panel-header">
        <span class="eyebrow">${t('liveTunnelUsage')}</span>
        <span class="usage-badge usage-badge-live">${t('live')}</span>
      </div>
      <div class="usage-grid">
        <div class="usage-cell">
          <span class="usage-label">${t('requests')}</span>
          <strong>${formatNumber(usage.totalRequests)}</strong>
          <span class="usage-sub">${formatNumber(usage.requestsPerMin)}${t('perMin')}</span>
        </div>
        <div class="usage-cell">
          <span class="usage-label">${t('edgeConnections')}</span>
          <strong>${formatNumber(usage.haConnections)}</strong>
          <span class="usage-sub">${t('activeToCloudflare')}</span>
        </div>
        <div class="usage-cell">
          <span class="usage-label">${t('inFlight')}</span>
          <strong>${formatNumber(usage.activeConns)}</strong>
          <span class="usage-sub">${t('concurrentRequests')}</span>
        </div>
        <div class="usage-cell">
          <span class="usage-label">${t('uptime')}</span>
          <strong>${escapeHtml(formatUptime(usage.uptimeSeconds))}</strong>
          <span class="usage-sub">${escapeHtml(edgeLabel)}</span>
        </div>
      </div>
      <div class="usage-codes">${codesHtml}</div>
      <p class="usage-hint">${t('usageHint')}</p>
    </div>
  `;
}

function displayVersionLabel(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) return '';
  return trimmed.startsWith('v') ? trimmed : `v${trimmed}`;
}

function updateBanner(appState: AppState): string {
  const update = appState.update;
  if (!update.available) return '';
  return `
    <section class="install-banner" data-action="tab-about">
      <div class="install-banner-copy">
        <span class="install-banner-label">${t('updateAvailable')}</span>
        <strong>Exposely ${escapeHtml(displayVersionLabel(update.latestVersion))} ${t('isReady')}</strong>
        <p>${escapeHtml(update.message || `${t('updateCurrentVersion')} v${update.currentVersion}.`)}</p>
      </div>
      <div class="install-banner-action">
        <button type="button" data-action="install-latest-update">${t('updateNow')}</button>
        <button type="button" data-action="open-latest-release">${t('openRelease')}</button>
      </div>
    </section>
  `;
}

function logRows(logs: LogEntry[]): string {
  return logs
    .slice(-12)
    .reverse()
    .map(
      (entry) => `
        <div class="log-entry log-${escapeHtml(entry.level)}">
          <span>${escapeHtml(new Date(entry.timestamp).toLocaleTimeString())}</span>
          <strong>${escapeHtml(entry.source)}</strong>
          <p>${escapeHtml(entry.message)}</p>
        </div>
      `,
    )
    .join('');
}

function projectRows(appState: AppState): string {
  const query = state.projectSearch.trim().toLowerCase();
  const projects = appState.settings.projects.filter((project) => {
    if (!query) return true;
    return [project.displayName, projectTypeLabel(project), project.localHost, project.localURL, project.projectPath, project.publicURL, project.subdomain]
      .some((value) => value.toLowerCase().includes(query));
  });

  if (projects.length === 0) {
    return `<div class="empty-list">${escapeHtml(query ? t('noMatchingProjects') : t('noProject'))}</div>`;
  }

  return projects
    .map((project) => {
      const isSelected = selectedProject()?.id === project.id;
      const showRunning = state.activeProjectId === project.id && appState.status.running;
      return `
        <button type="button" class="project-row ${isSelected ? 'selected' : ''}" data-action="select-project" data-id="${escapeHtml(project.id)}">
          <div class="project-copy">
            <strong>${escapeHtml(project.displayName)}</strong>
            <span class="project-type-tag">${escapeHtml(projectTypeLabel(project))}</span>
          </div>
          ${showRunning ? `<span class="project-running-badge">${t('running')}</span>` : ''}
        </button>
      `;
    })
    .join('');
}

// Tracks the active tab from the previous render so we can scroll to the top
// only on actual tab switches. Without this, every render() call rewrites
// root.innerHTML and the browser resets the scroll position, which means
// "Save Settings" on a scrolled-down page jumps you back to the top.
let lastRenderedTab: string | null = null;

// captureMainScroll returns the current scrollTop of the .main-content
// container, or 0 if the page is still loading or hasn't been laid out yet.
// We use this to restore the user's scroll position across re-renders.
function captureMainScroll(): number {
  const scroller = root.querySelector<HTMLElement>('.main-content');
  return scroller ? scroller.scrollTop : 0;
}

// restoreMainScroll puts the .main-content scroller back where it was
// before render() blew away the DOM. Capped at scrollHeight - clientHeight
// so a render that shrinks the page does not leave the scroller in an
// out-of-range position.
function restoreMainScroll(scrollTop: number) {
  const scroller = root.querySelector<HTMLElement>('.main-content');
  if (!scroller) return;
  const max = scroller.scrollHeight - scroller.clientHeight;
  scroller.scrollTop = Math.max(0, Math.min(scrollTop, max));
}

function render() {
  if (state.fatalError) {
    root.innerHTML = `
      <main class="shell loading-state">
        <section class="hero-card" style="border-color: var(--danger);">
          <h1 style="color: var(--danger);">${t('initError')}</h1>
          <p>${escapeHtml(state.fatalError)}</p>
          <button type="button" onclick="window.location.reload()" style="margin-top: 20px;">${t('reloadApp')}</button>
        </section>
      </main>
    `;
    lastRenderedTab = null;
    return;
  }

  if (!state.appState) {
    root.innerHTML = `
      <main class="shell loading-state">
        <section class="hero-card">
          <h1>Exposely</h1>
          <p>${t('loadingBackend')}</p>
        </section>
      </main>
    `;
    lastRenderedTab = null;
    return;
  }

  const appState = state.appState;
  syncProjectUrlsFromState(appState);
  const project = selectedProject();
  state.selectedProjectId = project?.id ?? null;
  const tunnelStatus = appState.status;
  const projectUrl = project ? resolvedProjectURL(project, appState) : '';
  const hasProjects = appState.settings.projects.length > 0;
  const shareToolReady = appState.cloudflaredDetected;
  const stackConfigured = !!(appState.settings.stack && (appState.settings.stack.nginxBinaryPath || appState.settings.stack.phpCgiBinaryPath || appState.settings.stack.mysqldBinaryPath));
  const stackRunningCount = state.stackStatuses.filter((s) => s.running).length;
  const stackTotalCount = state.stackStatuses.length || 3;
  const canSetupTunnel = shareToolReady;
  const shareAction = project ? shareActionForProject(project) : null;
  const canShareSelectedProject = Boolean(
    project
      && shareAction
      && shareToolReady
      && (project.shareMode === 'auto' || project.shareMode === 'quick' || project.shareMode === 'host-html'),
  );
  const canStartTunnel = project ? canShareSelectedProject : shareToolReady && hasProjects;
  const canRunProjectBuild = Boolean(project?.projectPath.trim()) && !looksLikeURL(project?.projectPath ?? '') && appState.buildCommandDetected && !appState.buildRunning;
  const canTestProject = Boolean(project?.localHost.trim()) && project?.shareMode !== 'host-html' && project?.shareMode !== 'auto';
  const shareToolStatusLabel = shareToolReady ? t('installed') : t('notInstalled');
  const shareToolStatusBadgeClass = shareToolReady ? 'pill-success' : 'pill-outline';
  const shareToolStatusMessage = shareToolReady
    ? t('cloudflaredReady')
    : t('cloudflaredNotReady');
  const setupTunnelStatusLabel = canSetupTunnel ? t('available') : t('installRequired');
  const setupTunnelStatusBadgeClass = canSetupTunnel ? 'pill-success' : 'pill-outline';
  const setupTunnelStatusMessage = canSetupTunnel
    ? t('tunnelAvailable')
    : t('tunnelNotAvailable');
  const installBannerPath = appState.cloudflaredPath || 'cloudflared.exe (PATH)';
  // Prefer the new generic localStack* fields; fall back to the legacy
  // envkit* fields when the backend has not been refreshed yet so we
  // keep rendering correctly across version transitions. The name
  // fallback is intentionally staged so Exposely never shows the wrong
  // stack name:
  //   1. Use the backend's localStackName when set (covers EnvKit, Herd,
  //      Valet, Laragon, and the generic HTTP/HTTPS loopback kinds).
  //   2. Otherwise, if legacy EnvKit-only fields are populated the
  //      detected stack really was EnvKit — show "EnvKit".
  //   3. Otherwise fall back to a neutral label ("Local dev stack") so
  //      we don't misrepresent an unidentified stack as EnvKit.
  const localStackDetected = tunnelStatus.localStackDetected ?? tunnelStatus.envkitDetected;
  const rawStackName = (tunnelStatus.localStackName ?? '').trim();
  const looksLikeLegacyEnvKitPayload = !!(tunnelStatus.envkitDetected || tunnelStatus.envkitVersion || tunnelStatus.envkitPath);
  const localStackName = rawStackName
    || (looksLikeLegacyEnvKitPayload ? 'EnvKit' : t('localStackFallbackName'));
  const localStackVersion = tunnelStatus.localStackVersion || tunnelStatus.envkitVersion || '';
  const localStackPath = tunnelStatus.localStackPath || tunnelStatus.envkitPath;
  const localStackOrigin = tunnelStatus.localStackOriginUrl || tunnelStatus.envkitOriginUrl;
  const envkitHint = localStackDetected
    ? localStackVersion
      ? t('localStackDetected')
          .replace('{name}', escapeHtml(localStackName))
          .replace('{version}', escapeHtml(localStackVersion))
      : t('localStackDetectedNoVersion').replace('{name}', escapeHtml(localStackName))
    : t('localStackNotDetected');
  const envkitOriginHint = localStackDetected && localStackOrigin
    ? t('localStackDefaultOrigin')
        .replace('{name}', escapeHtml(localStackName))
        .replace('{origin}', escapeHtml(localStackOrigin))
    : '';
  const headerHint = !shareToolReady
    ? ''
    : !hasProjects
      ? t('createProjectFirst')
      : '';

  // Capture scroll position before we blow away the DOM. We only restore
  // when the tab did not change — explicit tab switches should land at the
  // top of the new tab so the user can see the page header.
  const previousScroll = captureMainScroll();
  const tabChanged = lastRenderedTab !== null && lastRenderedTab !== state.activeTab;
  lastRenderedTab = state.activeTab;

  root.innerHTML = `
    <main class="shell">
      <aside class="sidebar">
        <div class="sidebar-header">
          <div class="logo">
            <div class="logo-main">
              <img src="./logo.png?v=3" alt="App logo" />
              <div class="logo-text">
                <h1>Exposely</h1>
                <div class="status-indicator tone-${statusTone(tunnelStatus)}">
                  <span class="status-dot"></span>
                  <span>${escapeHtml(tunnelStatus.running ? t('running') : t('stopped'))}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="sidebar-section sidebar-create-section">
          <div class="section-header">
            <h3>${t('projects')}</h3>
            <button type="button" class="add-button" data-action="new-project" title="${t('newProject')}">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <line x1="12" y1="5" x2="12" y2="19"></line>
                <line x1="5" y1="12" x2="19" y2="12"></line>
              </svg>
            </button>
          </div>
        </div>

        <div class="sidebar-section sidebar-projects-section">
          <div class="project-search">
            <input id="project-search" type="search" value="${escapeHtml(state.projectSearch)}" placeholder="${t('searchProjects')}" aria-label="${t('searchProjects')}" autocomplete="off" />
          </div>
          <div class="project-list">${projectRows(appState)}</div>
        </div>

      </aside>

      <section class="main-content">
        <div class="app-navbar">
          <nav class="top-nav" aria-label="Primary">
            <button type="button" class="top-nav-item ${state.activeTab === 'overview' ? 'active' : ''}" data-action="tab-overview">
              <svg width="16" height="16" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                <path d="M10 2L2 7L10 12L18 7L10 2Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                <path d="M2 13L10 18L18 13" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
              <span>${t('overview')}</span>
            </button>
            <button type="button" class="top-nav-item ${state.activeTab === 'settings' ? 'active' : ''}" data-action="tab-settings">
              <svg width="16" height="16" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                <circle cx="10" cy="10" r="3" stroke="currentColor" stroke-width="2"/>
                <path d="M10 2V4M10 16V18M18 10H16M4 10H2M15.66 4.34L14.24 5.76M5.76 14.24L4.34 15.66M15.66 15.66L14.24 14.24M5.76 5.76L4.34 4.34" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
              <span>${t('settings')}</span>
            </button>
            <button type="button" class="top-nav-item ${state.activeTab === 'about' ? 'active' : ''}" data-action="tab-about">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="16" x2="12" y2="12"></line>
                <line x1="12" y1="8" x2="12.01" y2="8"></line>
              </svg>
              <span>${t('about')}</span>
            </button>
            <button type="button" class="top-nav-item ${state.activeTab === 'logs' ? 'active' : ''}" data-action="tab-logs">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
                <polyline points="14 2 14 8 20 8"></polyline>
                <line x1="8" y1="13" x2="16" y2="13"></line>
                <line x1="8" y1="17" x2="16" y2="17"></line>
              </svg>
              <span>${t('logs')}</span>
            </button>
          </nav>
          <div class="navbar-actions">
            <button type="button" class="stack-toggle ${stackRunningCount > 0 ? 'stack-toggle-on' : ''}" data-action="stack-toggle-all" title="${stackRunningCount > 0 ? t('stackStopAll') : t('stackStartAll')}" ${!stackConfigured ? 'disabled' : ''}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6h16M4 12h16M4 18h16"></path><circle cx="9" cy="6" r="1.6" fill="currentColor" stroke="none"></circle><circle cx="15" cy="12" r="1.6" fill="currentColor" stroke="none"></circle><circle cx="7" cy="18" r="1.6" fill="currentColor" stroke="none"></circle></svg>
              <span>Stack</span>
              <span class="stack-toggle-badge">${stackRunningCount}/${stackTotalCount}</span>
            </button>
            <button type="button" class="stack-toggle" data-action="open-database-manager" title="${t('dbManagerTitle')}" ${!stackConfigured ? 'disabled' : ''}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"></ellipse><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"></path><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"></path></svg>
              <span>${t('dbManager')}</span>
            </button>
            <button type="button" class="icon-switch" data-action="toggle-theme" aria-label="${nextTheme() === 'light' ? t('switchToLight') : t('switchToDark')}" title="${nextTheme() === 'light' ? t('switchToLight') : t('switchToDark')}">
              ${themeIcon(nextTheme())}
            </button>
            <button type="button" class="icon-switch lang-switch" data-action="toggle-lang" aria-label="${nextLanguage() === 'km' ? t('switchToKhmer') : t('switchToEnglish')}" title="${nextLanguage() === 'km' ? t('switchToKhmer') : t('switchToEnglish')}">
              <span aria-hidden="true">${languageFlagIcon(nextLanguage())}</span>
            </button>
          </div>
        </div>

        ${!shareToolReady ? `
          <section class="install-banner" data-action="tab-settings">
            <div class="install-banner-copy">
              <span class="install-banner-label">${t('requiredSetup')}</span>
              <strong>${t('installCloudflaredFirst')}</strong>
              <p>${t('tunnelBannerDesc')}</p>
              <span class="install-banner-path">${t('expectedPath')}: ${escapeHtml(installBannerPath)}</span>
            </div>
            <div class="install-banner-action">
              <button type="button" class="danger-button" data-action="tab-settings">${t('openSettings')}</button>
            </div>
          </section>
        ` : ''}
        ${updateBanner(appState)}

        <header class="content-header">
          <div class="header-info">
            <h2>${state.activeTab === 'overview' ? t('projects') : state.activeTab === 'settings' ? t('settings') : state.activeTab === 'logs' ? t('logs') : t('about')}</h2>
            ${state.busy ? `<p class="busy-indicator">${escapeHtml(state.busy)}</p>` : ''}
          </div>
          <div class="header-actions">
            <button type="button" data-action="start-tunnel" ${state.appState?.status.running || !canStartTunnel ? 'disabled' : ''}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"></polygon></svg>
              ${escapeHtml(shareAction?.label || t('startTunnel'))}
            </button>
            <button type="button" class="secondary" data-action="stop-tunnel">
               <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="6" width="12" height="12"></rect></svg>
               ${t('stop')}
            </button>
            <button type="button" class="secondary" data-action="refresh" title="${t('refreshState')}">
               <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"></polyline><polyline points="1 20 1 14 7 14"></polyline><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
            </button>
          </div>
        </header>

        ${headerHint ? `<p class="header-hint">${escapeHtml(headerHint)}</p>` : ''}

        ${state.notice ? `<section class="toast toast-${state.notice.tone}">${escapeHtml(state.notice.message)}</section>` : ''}

          ${
            state.editorOpen
              ? `
              <section class="panel editor-panel">
                <div class="panel-header">
                  <div>
                    <p class="eyebrow">${state.editorMode === 'create' ? t('newProject') : t('editProject')}</p>
                    <h2>${escapeHtml(state.editorProject.displayName || t('projectPreset'))}</h2>
                  </div>
                  <button type="button" class="secondary" data-action="close-editor">${t('closeEditor')}</button>
                </div>
                <form id="project-form" class="form-grid editor-grid">
                  <input type="hidden" name="id" value="${escapeHtml(state.editorProject.id)}" />
                  <label class="wide">${t('displayName')}<input name="displayName" value="${escapeHtml(state.editorProject.displayName)}" /></label>
                  <label class="wide">
                    ${state.editorProject.shareMode === 'quick' ? t('projectFolder') : t('projectFolderOptional')}
                    <div class="folder-picker">
                      <input
                        name="projectPath"
                        value="${escapeHtml(state.editorProject.projectPath)}"
                        placeholder="${state.editorProject.shareMode === 'quick' ? 'D:\\code\\hr-system' : 'D:\\code\\site'}"
                      />
                      <button type="button" class="secondary browse-button" data-action="browse-project-folder">${t('browse')}</button>
                    </div>
                  </label>
                  <label>
                    ${t('projectType')}
                    <select name="shareMode">
                      <option value="auto" ${state.editorProject.shareMode === 'auto' ? 'selected' : ''}>${t('autoDetectProject')}</option>
                      <option value="quick" ${state.editorProject.shareMode === 'quick' ? 'selected' : ''}>${t('localHostProjectOption')}</option>
                      <option value="host-html" ${state.editorProject.shareMode === 'host-html' ? 'selected' : ''}>${t('htmlProjectOption')}</option>
                    </select>
                  </label>
                  ${
                    state.editorProject.shareMode === 'quick' || state.editorProject.shareMode === 'auto'
                      ? `<label>${state.editorProject.shareMode === 'auto' ? t('localHostOptional') : t('localHostRequired')}<input name="localHost" value="${escapeHtml(state.editorProject.localHost)}" placeholder="app.test" /></label>`
                      : ''
                  }
                  ${
                    state.editorProject.shareMode === 'auto' || state.editorProject.shareMode === 'host-html'
                      ? `<label>${t('localUrl')}<input name="localURL" value="${escapeHtml(state.editorProject.localURL)}" placeholder="http://127.0.0.1:5500" /></label>`
                      : ''
                  }
                  ${
                    state.editorProject.shareMode === 'quick' || state.editorProject.shareMode === 'auto'
                      ? `<label>${t('originUrl')}<input name="originURL" value="${escapeHtml(state.editorProject.originURL)}" placeholder="http://127.0.0.1:80" /></label>`
                      : ''
                  }
                  ${
                    state.editorProject.shareMode === 'auto'
                      ? `<label class="wide">${t('startCommand')}<input name="startCommand" value="${escapeHtml(state.editorProject.startCommand)}" placeholder="npm run dev -- --port 4173" /></label>`
                      : ''
                  }
                   ${
                     state.editorProject.shareMode === 'host-html'
                       ? `
                        <div class="field-hint hint-info">
                          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                           <circle cx="7" cy="7" r="6" stroke="currentColor" stroke-width="1.2"/>
                           <path d="M7 10V7M7 4H7.01" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
                         </svg>
                         <span>${t('hintHtml')}</span>
                        </div>
                        `
                      : state.editorProject.shareMode === 'auto'
                        ? `
                        <div class="field-hint hint-info">
                          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                           <circle cx="7" cy="7" r="6" stroke="currentColor" stroke-width="1.2"/>
                           <path d="M7 10V7M7 4H7.01" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
                         </svg>
                         <span>${t('hintAuto')}</span>
                        </div>
                        `
                       : `
                        <div class="field-hint hint-success">
                          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                           <circle cx="7" cy="7" r="6" stroke="currentColor" stroke-width="1.2"/>
                           <path d="M4.5 7L6 8.5L9.5 5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
                         </svg>
                          <span>${t('hintLocalHost')}</span>
                       </div>
                     `
                   }
                  <div class="action-row wide"><button type="submit">${state.editorMode === 'create' ? t('saveProject') : t('updateProject')}</button></div>
                </form>
              </section>
            `
              : ''
          }

          ${
            state.activeTab === 'overview'
              ? `
                <article class="panel selected-panel">
                  <div class="panel-header">
                    <div>
                      <h2>${escapeHtml(project?.displayName || t('noProject'))}</h2>
                    </div>
                    <div class="selected-header-actions">
                      ${
                        project
                          ? `
                            <div class="dropdown">
                              <button type="button" class="secondary menu-button" data-action="toggle-project-menu">${t('manage')}</button>
                              ${
                                state.projectMenuOpen
                                  ? `
                                    <div class="dropdown-menu">
                                      <button type="button" class="dropdown-item" data-action="edit-project">
                                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                                          <path d="M11.333 2.667a.943.943 0 0 1 1.333 1.333l-6.4 6.4L3.333 11.333l.933-2.933 6.4-5.733Z" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
                                        </svg>
                                        <span>${t('editProject')}</span>
                                      </button>
                                      <button type="button" class="dropdown-item danger-item" data-action="delete-project">
                                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                                          <path d="M2.667 4h10.666M6 2.667h4M5.333 6v5.333M8 6v5.333M10.667 6v5.333M4.667 4l.4 8A1.333 1.333 0 0 0 6.4 13.333h3.2A1.333 1.333 0 0 0 10.933 12l.4-8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
                                        </svg>
                                        <span>${t('deleteProject')}</span>
                                      </button>
                                    </div>
                                  `
                                  : ''
                              }
                            </div>
                          `
                          : ''
                      }
                    </div>
                  </div>

                  ${
                    project
                      ? `
                        <div class="hero-project">
                          <div class="hero-project-main">
                            <strong>${escapeHtml(projectPrimaryTarget(project))}</strong>
                            <div class="inline-url-row">
                              <p>${escapeHtml(projectUrl || t('noPublicUrl'))}</p>
                              ${projectUrl ? `<button type="button" class="secondary inline-copy-button" data-action="open-url" aria-label="${t('open')}">
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>
                                ${t('open')}
                              </button>` : ''}
                              ${!projectUrl && project.shareMode !== 'stable' ? `<button type="button" class="secondary inline-copy-button" data-action="regenerate-url" aria-label="${t('createPublicUrl')}" ${!shareToolReady ? 'disabled' : ''}>
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M23 4v6h-6"></path><path d="M1 20v-6h6"></path><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
                                ${t('refreshUrl')}
                              </button>` : ''}
                            </div>
                          </div>
                        </div>

                        ${renderUsagePanel(tunnelStatus)}
                      `
                      : `<p class="empty-copy">${t('createOrSelect')}</p>`
                  }
                </article>
              `
                 : state.activeTab === 'settings'
                 ? `
                    <article class="panel compact-panel">
                      <div class="panel-header">
                        <div>
                          <p class="eyebrow">${t('setup')}</p>
                          <h2>${t('shareToolSetup')}</h2>
                        </div>
                      </div>
                      <div class="metric-grid">
                        <div class="metric-card metric-card-split">
                          <div class="metric-card-copy">
                            <span class="summary-label">${t('shareTool')}</span>
                            <strong>${escapeHtml(shareToolStatusLabel)} <span class="pill ${shareToolStatusBadgeClass}">${escapeHtml(shareToolStatusLabel)}</span></strong>
                            <p>${escapeHtml(shareToolStatusMessage)}</p>
                          </div>
                          <div class="action-row">
                            ${
                              shareToolReady
                                ? `<button type="button" class="secondary" data-action="ensure-cloudflared">${t('recheckCloudflared')}</button>`
                                : `
                                  <button type="button" data-action="install-cloudflared">${t('installCloudflared')}</button>
                                  <button type="button" class="secondary" data-action="ensure-cloudflared">${t('checkSetup')}</button>
                                `
                            }
                          </div>
                        </div>
                        <div class="metric-card metric-card-split">
                          <div class="metric-card-copy">
                            <span class="summary-label">${t('setupTunnel')}</span>
                            <strong>${escapeHtml(setupTunnelStatusLabel)} <span class="pill ${setupTunnelStatusBadgeClass}">${escapeHtml(setupTunnelStatusLabel)}</span></strong>
                            <p>${escapeHtml(setupTunnelStatusMessage)}</p>
                          </div>
                          <div class="action-row">
                            <button type="button" class="secondary" disabled>${t('notNeeded')}</button>
                          </div>
                        </div>
                      </div>
                      <div class="metric-grid metric-grid-section">
                        <div class="metric-card metric-card-split">
                          <div class="metric-card-copy">
                            <span class="summary-label">${t('envkitSectionTitle')}</span>
                            <strong>${escapeHtml(envkitHint)} <span class="pill ${localStackDetected ? 'pill-success' : 'pill-outline'}">${localStackDetected ? t('installed') : t('notInstalled')}</span></strong>
                            <p>${localStackDetected && localStackPath ? `${t('localStackInstallPath')}: ${escapeHtml(localStackPath)}` : ''}</p>
                            ${envkitOriginHint ? `<p>${escapeHtml(envkitOriginHint)}</p>` : ''}
                          </div>
                        </div>
                      </div>
                    </article>

                 <article class="panel compact-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">${t('defaults')}</p>
                        <h2>${t('appSettings')}</h2>
                      </div>
                    </div>
                    <form id="settings-form" class="form-grid">
                      <label>${t('cloudflaredPath')}<input name="cloudflaredPath" value="${escapeHtml(appState.settings.cloudflaredPath)}" placeholder="${t('leaveBlankDefault')}" /></label>
                      <label>${t('localServiceUrl')}<input name="defaultServiceURL" value="${escapeHtml(appState.settings.defaultServiceURL)}" /></label>
                      <label class="checkbox-row">
                        <input type="checkbox" name="insecureSkipOriginTls" ${appState.settings.insecureSkipOriginTls ? 'checked' : ''} />
                        <span>
                          <strong>${t('insecureSkipOriginTls')}</strong>
                          <small class="hint">${t('insecureSkipOriginTlsHint')}</small>
                        </span>
                      </label>
                      <label class="checkbox-row">
                        <input type="checkbox" name="devServerFallback" ${appState.settings.devServerFallback !== false ? 'checked' : ''} />
                        <span>
                          <strong>${t('devServerFallback')}</strong>
                          <small class="hint">${t('devServerFallbackHint')}</small>
                        </span>
                      </label>
                      <div class="action-row wide"><button type="submit">${t('save')}</button></div>
                     </form>
                   </article>

                   <article class="panel compact-panel">
                     <div class="panel-header">
                       <div>
                         <p class="eyebrow">${t('stackSectionTitle')}</p>
                         <h2>${t('stackSectionSubtitle')}</h2>
                       </div>
                       <div class="action-row">
                         <button type="button" class="secondary" data-action="stack-start-all">${t('stackStartAll')}</button>
                         <button type="button" class="secondary" data-action="stack-stop-all">${t('stackStopAll')}</button>
                       </div>
                     </div>
                     <form id="stack-form" class="form-grid">
                       <label>${t('stackNginxPath')}<input name="nginxBinaryPath" value="${escapeHtml(state.stackDraft.nginxBinaryPath)}" placeholder="C:\\nginx\\nginx.exe" /></label>
                       <label>${t('stackPhpPath')}<input name="phpCgiBinaryPath" value="${escapeHtml(state.stackDraft.phpCgiBinaryPath)}" placeholder="C:\\php\\php-cgi.exe" /></label>
                       <label>${t('stackMysqlPath')}<input name="mysqldBinaryPath" value="${escapeHtml(state.stackDraft.mysqldBinaryPath)}" placeholder="C:\\mysql\\bin\\mysqld.exe" /></label>
                       <label>${t('stackNginxPort')}<input type="number" min="1" max="65535" name="nginxPort" value="${escapeHtml(String(state.stackDraft.nginxPort ?? 8090))}" /></label>
                       <label>${t('stackPhpPort')}<input type="number" min="1" max="65535" name="phpPort" value="${escapeHtml(String(state.stackDraft.phpPort ?? 9000))}" /></label>
                       <label>${t('stackMysqlPort')}<input type="number" min="1" max="65535" name="mysqlPort" value="${escapeHtml(String(state.stackDraft.mysqlPort ?? 3306))}" /></label>
                       <div class="action-row wide"><button type="submit">${t('stackSave')}</button></div>
                     </form>
                     <div class="metric-grid metric-grid-section">
                       ${stackStatusCards()}
                     </div>
                     <p class="hint">${t('stackHint')}</p>
                   </article>
                  `
                 : state.activeTab === 'logs'
                ? `
                  <section class="panel logs-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">${t('logs')}</p>
                        <h2>${t('logsSubtitle')}</h2>
                      </div>
                      <span class="pill pill-outline">${escapeHtml(`${appState.status.lastLogs.length} ${t('entries')}`)}</span>
                    </div>
                    <div class="log-stream">${logRows(appState.status.lastLogs)}</div>
                  </section>
                `
               : `
                  <article class="panel compact-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">${t('application')}</p>
                        <h2>Exposely</h2>
                      </div>
                    </div>
                    <div class="metric-grid">
                      <div class="metric-card">
                         <span class="summary-label">${t('version')}</span>
                         <strong>${escapeHtml(displayVersionLabel(appState.productVersion))}</strong>
                      </div>
                      <div class="metric-card">
                         <span class="summary-label">${t('platform')}</span>
                         <strong>Windows</strong>
                      </div>
                    </div>
                    <div class="metric-grid" style="margin-top: 20px;">
                      <div class="metric-card">
                        <span class="summary-label">${t('updates')}</span>
                        <strong>${escapeHtml(appState.update.available ? `${displayVersionLabel(appState.update.latestVersion)} ${t('available')}` : appState.update.checked ? t('upToDate') : t('checkingBackground'))}</strong>
                        <p style="margin-top: 10px; color: var(--text-secondary);">${escapeHtml(appState.update.message || t('updateHint'))}</p>
                        <div class="action-row" style="margin-top: 14px;">
                          <button type="button" class="secondary" data-action="check-updates">${t('checkNow')}</button>
                          <button type="button" ${appState.update.available ? '' : 'disabled'} data-action="install-latest-update">${t('updateNow')}</button>
                          <button type="button" ${appState.update.releaseUrl ? '' : 'disabled'} data-action="open-latest-release">${t('openRelease')}</button>
                        </div>
                      </div>
                    </div>
                    <div style="padding: 24px; color: var(--text-secondary); line-height: 1.6;">
                      <p>${t('aboutDescription')}</p>
                      <p style="margin-top: 16px;">${t('aboutCreator')}</p>
                      <p style="margin-top: 16px; color: var(--text-primary); font-weight: 500;">${t('aboutLicense')}</p>
                    </div>
                  </article>
               `
          }

        </section>
      </section>

    </main>
  `;

  bindForms();

  if (!tabChanged) {
    // Re-rendering on the same tab (e.g. after Save Settings, Refresh, or
    // a tunnel-status update) should keep the user's scroll position so
    // they don't get yanked back to the top.
    restoreMainScroll(previousScroll);
  }
}

function formValue(form: HTMLFormElement, name: string): string {
  return (form.elements.namedItem(name) as HTMLInputElement | HTMLSelectElement | null)?.value?.trim() ?? '';
}

function syncStackDraftFromState(appState: AppState) {
  const stack = appState.settings.stack ?? {};
  state.stackDraft = {
    nginxBinaryPath: stack.nginxBinaryPath ?? '',
    phpCgiBinaryPath: stack.phpCgiBinaryPath ?? '',
    mysqldBinaryPath: stack.mysqldBinaryPath ?? '',
    nginxPort: stack.nginxPort ?? 8090,
    phpPort: stack.phpPort ?? 9000,
    mysqlPort: stack.mysqlPort ?? 3306,
  };
}

function stackStatusCards(): string {
  if (!state.stackStatuses.length) {
    return '';
  }
  return state.stackStatuses
    .map((st) => {
      const badge = st.running ? '<span class="pill pill-success">running</span>' : '<span class="pill pill-outline">stopped</span>';
      const detail = st.running ? `pid ${st.pid}` : escapeHtml(st.lastError || '');
      return `<div class="metric-card">
        <span class="summary-label">${escapeHtml(st.service)}</span>
        <strong>${badge}</strong>
        <p>${escapeHtml(detail)}</p>
      </div>`;
    })
    .join('');
}

function formChecked(form: HTMLFormElement, name: string): boolean {
  return (form.elements.namedItem(name) as HTMLInputElement | null)?.checked ?? false;
}

function syncEditorFromForm() {
  const projectForm = root.querySelector<HTMLFormElement>('#project-form');
  if (!projectForm) return;

  const shareModeValue = formValue(projectForm, 'shareMode') as ShareMode;
  const validShareModes: ShareMode[] = ['auto', 'quick', 'host-html'];
  const shareMode = validShareModes.includes(shareModeValue) ? shareModeValue : 'auto';
  const projectPath = formValue(projectForm, 'projectPath');
  const localURL = formValue(projectForm, 'localURL');
  const startCommand = formValue(projectForm, 'startCommand');
  const shouldInferLocalHost = shareMode === 'quick' && !looksLikeURL(projectPath);
  const localHost = formValue(projectForm, 'localHost') || (shouldInferLocalHost ? inferLocalHostFromPath(projectPath) : '');

  state.editorProject = {
    id: formValue(projectForm, 'id'),
    displayName: formValue(projectForm, 'displayName'),
    localHost,
    originURL: formValue(projectForm, 'originURL'),
    projectPath,
    localURL,
    startCommand,
    subdomain: formValue(projectForm, 'subdomain'),
    publicURL: state.editorProject.publicURL,
    shareMode,
  };
}

async function withAction<T>(label: string, fn: () => Promise<T>): Promise<T | undefined> {
  try {
    setBusy(label);
    const result = await fn();
    state.notice = null;
    return result;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setNotice('error', message);
    return undefined;
  } finally {
    setBusy(null);
  }
}

async function confirmProjectSwitch(nextProjectID: string): Promise<boolean> {
  if (!state.appState?.status.running) return true;
  if (!state.activeProjectId || state.activeProjectId === nextProjectID) return true;

  return window.confirm(t('confirmSwitchProject'));
}

function bindForms() {
  const projectSearch = root.querySelector<HTMLInputElement>('#project-search');
  projectSearch?.addEventListener('input', () => {
    state.projectSearch = projectSearch.value;
    render();
  });

  const licenseForm = root.querySelector<HTMLFormElement>('#license-form');
  licenseForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const token = formValue(licenseForm, 'licenseToken');
    state.licenseDraft = token;
    const next = await withAction(t('loadingBackend'), () => api.activateLicense(token));
    if (next) {
      state.appState = next;
      state.licenseDraft = '';
      if (next.license.isAdmin && state.activeTab !== 'settings') {
        state.activeTab = 'settings';
      }
      setNotice('success', next.license.message || t('settingsSaved'));
    }
  });

  const settingsForm = root.querySelector<HTMLFormElement>('#settings-form');
  settingsForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
      const payload = {
        ...state.appState!.settings,
        cloudflaredPath: formValue(settingsForm, 'cloudflaredPath'),
        defaultServiceURL: formValue(settingsForm, 'defaultServiceURL'),
        insecureSkipOriginTls: formChecked(settingsForm, 'insecureSkipOriginTls'),
        devServerFallback: formChecked(settingsForm, 'devServerFallback'),
      };
    const next = await withAction(t('save'), () => api.saveSettings(payload));
    if (next) {
      state.appState = next;
      setNotice('success', t('settingsSaved'));
    }
  });

  const stackForm = root.querySelector<HTMLFormElement>('#stack-form');
  stackForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const payload = {
      ...state.appState!.settings,
      stack: {
        nginxBinaryPath: formValue(stackForm, 'nginxBinaryPath'),
        phpCgiBinaryPath: formValue(stackForm, 'phpCgiBinaryPath'),
        mysqldBinaryPath: formValue(stackForm, 'mysqldBinaryPath'),
        nginxPort: Number(formValue(stackForm, 'nginxPort')) || 8090,
        phpPort: Number(formValue(stackForm, 'phpPort')) || 9000,
        mysqlPort: Number(formValue(stackForm, 'mysqlPort')) || 3306,
      },
    };
    const next = await withAction(t('save'), () => api.saveSettings(payload));
    if (next) {
      state.appState = next;
      syncStackDraftFromState(next);
      setNotice('success', t('settingsSaved'));
    }
  });

  const projectForm = root.querySelector<HTMLFormElement>('#project-form');
  projectForm?.addEventListener('change', (event) => {
    const target = event.target as HTMLInputElement | HTMLSelectElement | null;
    if (!target) return;

    if (target.name === 'shareMode' || target.name === 'projectPath' || target.name === 'localHost' || target.name === 'originURL' || target.name === 'localURL' || target.name === 'startCommand') {
      syncEditorFromForm();
      render();
    }
  });

  projectForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const payload: ProjectPreset = {
      id: formValue(projectForm, 'id'),
      displayName: formValue(projectForm, 'displayName'),
      localHost: formValue(projectForm, 'localHost'),
      originURL: formValue(projectForm, 'originURL'),
      projectPath: formValue(projectForm, 'projectPath'),
      localURL: formValue(projectForm, 'localURL'),
      startCommand: formValue(projectForm, 'startCommand'),
      subdomain: formValue(projectForm, 'subdomain'),
      publicURL: state.editorProject.publicURL,
      shareMode: state.editorProject.shareMode,
    };
    const next = await withAction(t('saveProject'), () => api.saveProject(payload));
    if (!next) return;

    state.appState = next;
    const savedProjectId = payload.id || (next.settings.projects[next.settings.projects.length - 1]?.id ?? null);
    state.selectedProjectId = savedProjectId;
    state.editorOpen = false;
    setNotice('success', t('projectSaved'));
  });
}

async function handleAction(action: string, id: string | null) {
  switch (action) {
    case 'toggle-lang': {
      const newLang = getLang() === 'en' ? 'km' : 'en';
      setLang(newLang);
      if (state.appState) {
        state.appState.settings.language = newLang;
        api.saveSettings(state.appState.settings);
      }
      render();
      return;
    }
    case 'toggle-theme': {
      const newTheme = nextTheme();
      setTheme(newTheme);
      if (state.appState) {
        state.appState.settings.theme = newTheme;
        api.saveSettings(state.appState.settings);
      }
      render();
      return;
    }
    case 'tab-overview':
      state.activeTab = 'overview';
      state.projectMenuOpen = false;
      render();
      return;
    case 'tab-settings':
      state.activeTab = 'settings';
      state.projectMenuOpen = false;
      render();
      return;
    case 'tab-logs':
      state.activeTab = 'logs';
      state.projectMenuOpen = false;
      render();
      return;
    case 'tab-about':
      state.activeTab = 'about';
      state.projectMenuOpen = false;
      render();
      return;
    case 'toggle-project-menu':
      state.projectMenuOpen = !state.projectMenuOpen;
      render();
      return;
    case 'select-project':
      state.selectedProjectId = id ?? null;
      state.projectMenuOpen = false;
      render();
      return;
    case 'new-project':
      state.editorOpen = true;
      state.editorMode = 'create';
      state.editorProject = blankProject();
      render();
      return;
    case 'edit-project':
      if (selectedProject()) {
        state.editorOpen = true;
        state.editorMode = 'edit';
        state.editorProject = { ...selectedProject()! };
        state.projectMenuOpen = false;
        render();
      }
      return;
    case 'close-editor':
      state.editorOpen = false;
      render();
      return;
    case 'clear-license': {
      const next = await withAction(t('settings'), () => api.clearLicense());
      if (next) {
        state.appState = next;
        state.licenseDraft = '';
        state.activeTab = 'settings';
        setNotice('success', t('settingsSaved'));
      }
      return;
    }
    case 'browse-project-folder': {
      syncEditorFromForm();
      const result = await withAction(t('browse'), () => api.browseProjectFolder(state.editorProject.projectPath));
      if (typeof result === 'string' && result) {
        const inferredLocalHost = inferLocalHostFromPath(result);
        state.editorProject.projectPath = result;
        if (state.editorProject.shareMode === 'quick' && !state.editorProject.localHost.trim()) {
          state.editorProject.localHost = inferredLocalHost;
        }
        render();
      }
      return;
    }
    case 'random-subdomain': {
      syncEditorFromForm();
      state.editorProject.subdomain = randomSubdomainValue();
      render();
      return;
    }
    case 'open-database-manager': {
      const next = await withAction(t('dbManagerOpening'), () => api.openDatabaseManager());
      if (next) {
        setNotice('success', `${t('dbManagerOpened')}: ${next}`);
      }
      return;
    }
    case 'stack-toggle-all': {
      // One-click switch in the navbar: turn the whole stack on, or off
      // when any service is already running.
      const turningOff = state.stackStatuses.some((s) => s.running);
      const action = turningOff ? api.stopStack() : api.startStack();
      const next = await withAction(turningOff ? t('stackStopAll') : t('stackStartAll'), () => action);
      if (next) {
        state.appState = next;
        state.stackStatuses = await api.stackStatus().catch(() => state.stackStatuses);
        setNotice('success', turningOff ? t('stackStopped') : t('stackStarted'));
      }
      return;
    }
    case 'stack-start-all': {
      const next = await withAction(t('stackStartAll'), () => api.startStack());
      if (next) {
        state.appState = next;
        state.stackStatuses = await api.stackStatus().catch(() => state.stackStatuses);
        setNotice('success', t('stackStarted'));
      }
      return;
    }
    case 'stack-stop-all': {
      const next = await withAction(t('stackStopAll'), () => api.stopStack());
      if (next) {
        state.appState = next;
        state.stackStatuses = await api.stackStatus().catch(() => state.stackStatuses);
        setNotice('success', t('stackStopped'));
      }
      return;
    }
    case 'stack-start-service': {
      const service = id ?? '';
      if (!service) return;
      const next = await withAction(`${t('stackStart')} ${service}`, () => api.startStackService(service));
      if (next) {
        state.appState = next;
        state.stackStatuses = await api.stackStatus().catch(() => state.stackStatuses);
        setNotice('success', `${service} ${t('stackStarted')}`);
      }
      return;
    }
    case 'stack-stop-service': {
      const service = id ?? '';
      if (!service) return;
      const next = await withAction(`${t('stackStop')} ${service}`, () => api.stopStackService(service));
      if (next) {
        state.appState = next;
        state.stackStatuses = await api.stackStatus().catch(() => state.stackStatuses);
        setNotice('success', `${service} ${t('stackStopped')}`);
      }
      return;
    }
    case 'copy-url': {
      state.projectMenuOpen = false;
      const project = selectedProject();
      if (!project || !state.appState) return;
      const url = resolvedProjectURL(project, state.appState);
      if (!url) {
        setNotice('error', t('noPublicUrlForProject'));
        return;
      }
      await navigator.clipboard.writeText(url);
      state.copiedProjectId = project.id;
      if (copyLabelTimer !== null) {
        window.clearTimeout(copyLabelTimer);
      }
      copyLabelTimer = window.setTimeout(() => {
        state.copiedProjectId = null;
        copyLabelTimer = null;
        render();
      }, 1800);
      render();
      return;
    }
  }

  if (!id && ['share-project', 'share-random', 'share-quick', 'regenerate-url', 'open-url', 'npm-build', 'test-project', 'delete-project'].includes(action)) {
    setNotice('error', t('selectProjectFirst'));
    return;
  }

  switch (action) {
    case 'start-tunnel': {
      const project = selectedProject();
      if (project) {
        const shareAction = shareActionForProject(project);
        await handleAction(shareAction.action, project.id);
        return;
      }

      const refreshed = await withAction(t('refreshState'), () => api.refreshState());
      if (!refreshed) return;
      state.appState = refreshed;

      if (refreshed.status.running) {
        setNotice('info', t('tunnelAlreadyRunning'));
        return;
      }

      try {
        setBusy(t('startTunnel'));
        const next = await api.startTunnel();
        state.appState = next;
        setNotice('success', t('namedTunnelStarted'));
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        if (message.toLowerCase().includes('already running')) {
          const latest = await api.refreshState().catch(() => null);
          if (latest) {
            state.appState = latest;
          }
          setNotice('info', t('tunnelAlreadyRunning'));
        } else {
          setNotice('error', message);
        }
      } finally {
        setBusy(null);
      }
      return;
    }
    case 'ensure-cloudflared': {
      const next = await withAction(t('checkSetup'), () => api.ensureCloudflared());
      if (next) {
        state.appState = next;
        setNotice('success', t('cloudflaredReadyNotice'));
      }
      return;
    }
    case 'install-cloudflared': {
      const next = await withAction(t('installCloudflared'), () => api.installCloudflared());
      if (next) {
        state.appState = next;
        setNotice('success', t('cloudflaredInstalled'));
      }
      return;
    }
    case 'stop-tunnel': {
      const next = await withAction(t('stop'), () => api.stopTunnel());
      if (next) {
        state.appState = next;
        state.activeProjectId = null;
        setNotice('success', t('tunnelStopped'));
      }
      return;
    }
    case 'create-tunnel': {
      const next = await withAction(t('setupTunnel'), () => api.createTunnel());
      if (next) {
        state.appState = next;
        setNotice('success', t('namedTunnelReady'));
      }
      return;
    }
    case 'open-config':
      await withAction(t('open'), () => api.openConfigFile());
      return;
    case 'open-settings':
      await withAction(t('openSettings'), () => api.openSettingsFile());
      return;
    case 'open-latest-release':
      await withAction(t('openRelease'), () => api.openLatestRelease());
      return;
    case 'install-latest-update':
      {
        const message = await withAction(t('updateNow'), () => api.installLatestUpdate());
        if (typeof message === 'string' && message) {
          setNotice('success', message);
        }
      }
      return;
    case 'check-updates': {
      const next = await withAction(t('checkNow'), () => api.checkForUpdates());
      if (next) {
        state.appState = next;
        setNotice(next.update.available ? 'info' : 'success', next.update.message || t('stateRefreshed'));
      }
      return;
    }
    case 'refresh': {
      const next = await withAction(t('refreshState'), () => api.refreshState());
      if (next) {
        state.appState = next;
        if (!state.activeProjectId && next.status.running) {
          state.activeProjectId = inferActiveProjectId(next);
        }
        setNotice('success', t('stateRefreshed'));
      }
      return;
    }
    case 'share-project': {
      if (!(await confirmProjectSwitch(id!))) return;
      if (state.appState?.status.running && state.activeProjectId && state.activeProjectId !== id) {
        const stopped = await withAction(t('stop'), () => api.stopTunnel());
        if (!stopped) return;
        state.appState = stopped;
        state.activeProjectId = null;
      }
      const next = await withAction(t('createPublicUrl'), () => api.shareProject(id!));
      if (next) {
        state.appState = next;
        state.activeProjectId = id!;
        if (next.status.activeUrl || next.status.quickUrl) {
          state.projectUrls[id!] = next.status.activeUrl || next.status.quickUrl;
        }
        setNotice('success', t('projectShared'));
      }
      return;
    }
    case 'share-random': {
      if (!(await confirmProjectSwitch(id!))) return;
      if (state.appState?.status.running && state.activeProjectId && state.activeProjectId !== id) {
        const stopped = await withAction(t('stop'), () => api.stopTunnel());
        if (!stopped) return;
        state.appState = stopped;
        state.activeProjectId = null;
      }
      const next = await withAction(t('createPublicUrl'), () => api.shareProjectWithRandomURL(id!));
      if (next) {
        state.appState = next;
        state.activeProjectId = id!;
        if (next.settings.projects) {
          const savedProject = next.settings.projects.find((project) => project.id === id);
          if (savedProject?.publicURL) {
            state.projectUrls[id!] = savedProject.publicURL;
          }
        }
        setNotice('success', t('randomDomainActive'));
      }
      return;
    }
    case 'share-quick': {
      if (!(await confirmProjectSwitch(id!))) return;
      if (state.appState?.status.running && state.activeProjectId && state.activeProjectId !== id) {
        const stopped = await withAction(t('stop'), () => api.stopTunnel());
        if (!stopped) return;
        state.appState = stopped;
        state.activeProjectId = null;
      }
      const next = await withAction(t('createPublicUrl'), () => api.startQuickTunnel(id!));
      if (next) {
        state.appState = next;
        state.activeProjectId = id!;
        delete state.projectUrls[id!];
        if (next.status.activeUrl || next.status.quickUrl) {
          state.projectUrls[id!] = next.status.activeUrl || next.status.quickUrl;
        }
        setNotice('success', t('tunnelLive'));
      }
      return;
    }
    case 'regenerate-url': {
      const project = selectedProject();
      if (!project) return;
      if (!(await confirmProjectSwitch(project.id))) return;
      if (state.appState?.status.running) {
        const stopped = await withAction(t('stop'), () => api.stopTunnel());
        if (!stopped) return;
        state.appState = stopped;
        state.activeProjectId = null;
      }

      const next = await withAction(
        t('refreshUrl'),
        () => api.startQuickTunnel(project.id),
      );
        if (next) {
          state.appState = next;
          state.activeProjectId = project.id;
          delete state.projectUrls[project.id];
          if (next.status.activeUrl || next.status.quickUrl) {
            state.projectUrls[project.id] = next.status.activeUrl || next.status.quickUrl;
          }
          setNotice('success', t('newUrlGenerated'));
        }
        return;
    }
    case 'open-url':
      await withAction(t('open'), () => api.openPublicURL(id!));
      return;
    case 'npm-build': {
      const result = await withAction('npm build', () => api.runNpmBuild(id!));
      if (result !== undefined) setNotice('success', t('npmBuildStarted'));
      return;
    }
    case 'test-project': {
      const result = await withAction(t('localUrl'), () => api.testProject(id!));
      if (typeof result === 'string') setNotice('success', result);
      return;
    }
    case 'delete-project':
      if (window.confirm(t('confirmDeleteProject'))) {
        const next = await withAction(t('deleteProject'), () => api.deleteProject(id!));
        if (next) {
          state.appState = next;
          state.selectedProjectId = next.settings.projects[0]?.id ?? null;
          state.projectMenuOpen = false;
          setNotice('success', t('projectDeleted'));
        }
      }
      return;
  }
}

root.addEventListener('click', (event) => {
  const target = event.target as HTMLElement | null;
  const actionElement = target?.closest<HTMLElement>('[data-action]');
  const action = actionElement?.dataset.action;

  // Close project menu if clicking outside of the dropdown container,
  // but DON'T close it if we are clicking the toggle button itself (as that is handled by handleAction).
  const insideDropdown = target?.closest('.dropdown');
  if (state.projectMenuOpen && !insideDropdown) {
    state.projectMenuOpen = false;
    render();
  }

  if (!actionElement) return;

  event.preventDefault();
  const id = actionElement.dataset.id ?? selectedProject()?.id ?? null;
  if (!action) return;
  void handleAction(action, id);
});

async function bootstrap() {
  // Initial render to show loading state immediately
  render();

  // Short delay to allow Wails bindings to settle in dev mode
  await new Promise(resolve => setTimeout(resolve, 100));

  try {
    if (await WindowIsMaximised()) {
      WindowUnmaximise();
    }
    WindowSetMinSize(COMPACT_WINDOW_MIN_WIDTH, COMPACT_WINDOW_MIN_HEIGHT);
    WindowSetSize(COMPACT_WINDOW_WIDTH, COMPACT_WINDOW_HEIGHT);
    WindowCenter();
  } catch {
    // Ignore runtime window sizing failures outside the packaged desktop app.
  }

  const next = await withAction(t('loadingBackend'), () => api.bootstrap());
  if (!next) {
    state.fatalError = t('initLoadError');
    render();
    return;
  }
  state.appState = next;
  setLang(next.settings.language || 'en');
  setTheme(next.settings.theme || 'dark');
  syncProjectUrlsFromState(next);
  state.selectedProjectId = next.settings.projects[0]?.id ?? null;
  state.activeProjectId = inferActiveProjectId(next);
  syncStackDraftFromState(next);
  render();

  void api.stackStatus().then((statuses) => {
    state.stackStatuses = statuses ?? [];
    render();
  }).catch(() => {
    // Stack status is optional; ignore when bindings are unavailable.
  });

  void api.checkForUpdates().then((latest) => {
    state.appState = latest;
    if (latest.update.available) {
      setNotice('info', latest.update.message || `Exposely ${displayVersionLabel(latest.update.latestVersion)} ${t('available')}.`);
      return;
    }
    render();
  }).catch(() => {
    // Ignore update check failures to keep startup non-blocking.
  });

  window.runtime?.EventsOn('log', (payload) => {
    if (!state.appState) return;
    const entry = payload as LogEntry;
    state.appState.status.lastLogs = [...state.appState.status.lastLogs, entry].slice(-300);
    render();
  });

  window.runtime?.EventsOn('status', (payload) => {
    if (!state.appState) return;
    const status = payload as TunnelStatus;
    state.appState.status = status;
    if (!state.activeProjectId && status.running) {
      state.activeProjectId = inferActiveProjectId(state.appState);
    }
    if (state.activeProjectId && (status.activeUrl || status.quickUrl)) {
      const nextUrl = status.activeUrl || status.quickUrl;
      const currentUrl = state.projectUrls[state.activeProjectId];
      state.projectUrls[state.activeProjectId] = nextUrl;

      if (nextUrl && nextUrl !== currentUrl) {
        const activeProject = state.appState.settings.projects.find((project) => project.id === state.activeProjectId);
        if (activeProject && activeProject.publicURL !== nextUrl) {
          const updatedProject: ProjectPreset = {
            ...activeProject,
            publicURL: nextUrl,
          };

          void api.saveProject(updatedProject).then((nextState) => {
            state.appState = nextState;
            syncProjectUrlsFromState(nextState);
            render();
          }).catch(() => {
            // Ignore background URL persistence errors to avoid interrupting the live tunnel UI.
          });
        }
      }
    }
    render();
  });
}

window.addEventListener('error', (event) => {
  console.error('Unhandled error:', event.error);
  if (!state.appState && !state.fatalError) {
    state.fatalError = `A runtime error occurred during startup: ${event.message}`;
    render();
  }
});

bootstrap();
