import './style.css';
import { api } from './api';
import { WindowCenter, WindowIsMaximised, WindowSetMinSize, WindowSetSize, WindowUnmaximise } from '../wailsjs/runtime/runtime';
import type { AppState, LogEntry, ProjectPreset, ShareMode, TunnelStatus } from './types';

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

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;');
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
      return { action: 'share-quick', label: 'Start Auto Share' };
    case 'quick':
      return { action: 'share-quick', label: 'Create Public URL' };
    case 'host-html':
      return { action: 'share-quick', label: 'Create HTML Site URL' };
    default:
      return { action: 'share-quick', label: 'Create Public URL' };
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
      return 'Auto Project';
    case 'host-html':
      return 'HTML Project';
    default:
      return 'Local Host Project';
  }
}

function projectPrimaryTarget(project: ProjectPreset): string {
  if (project.localURL.trim()) return project.localURL.trim();
  if (looksLikeURL(project.projectPath)) return project.projectPath.trim();
  if (project.shareMode === 'host-html') return 'HTML Site (Folder)';
  if (project.localHost.trim()) return project.localHost.trim();
  if (project.startCommand.trim()) return `Auto via ${project.startCommand.trim()}`;
  return project.projectPath.trim() || 'Project source';
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
      return 'HTML Site';
    default:
      return 'Public URL';
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
          <span class="eyebrow">Live tunnel usage</span>
          <span class="usage-badge">warming up</span>
        </div>
        <p class="usage-hint">Collecting live metrics from cloudflared... This takes a few seconds after the tunnel starts.</p>
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
    : '<span class="usage-code usage-code-muted">No responses yet</span>';

  const edge = (usage.edgeLocations || []).filter(Boolean);
  const edgeLabel = edge.length ? edge.join(', ') : '—';

  return `
    <div class="usage-panel">
      <div class="usage-panel-header">
        <span class="eyebrow">Live tunnel usage</span>
        <span class="usage-badge usage-badge-live">live</span>
      </div>
      <div class="usage-grid">
        <div class="usage-cell">
          <span class="usage-label">Requests</span>
          <strong>${formatNumber(usage.totalRequests)}</strong>
          <span class="usage-sub">${formatNumber(usage.requestsPerMin)}/min</span>
        </div>
        <div class="usage-cell">
          <span class="usage-label">Edge connections</span>
          <strong>${formatNumber(usage.haConnections)}</strong>
          <span class="usage-sub">active to Cloudflare</span>
        </div>
        <div class="usage-cell">
          <span class="usage-label">In-flight</span>
          <strong>${formatNumber(usage.activeConns)}</strong>
          <span class="usage-sub">concurrent requests</span>
        </div>
        <div class="usage-cell">
          <span class="usage-label">Uptime</span>
          <strong>${escapeHtml(formatUptime(usage.uptimeSeconds))}</strong>
          <span class="usage-sub">${escapeHtml(edgeLabel)}</span>
        </div>
      </div>
      <div class="usage-codes">${codesHtml}</div>
      <p class="usage-hint">Live from local cloudflared. No Cloudflare login required. Counters reset when the tunnel restarts.</p>
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
        <span class="install-banner-label">Update available</span>
        <strong>Exposely ${escapeHtml(displayVersionLabel(update.latestVersion))} is ready</strong>
        <p>${escapeHtml(update.message || `You are using v${update.currentVersion}.`)}</p>
      </div>
      <div class="install-banner-action">
        <button type="button" data-action="install-latest-update">Update Now</button>
        <button type="button" data-action="open-latest-release">Open Release</button>
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
  return appState.settings.projects
    .map((project) => {
      const isSelected = selectedProject()?.id === project.id;
      const showRunning = state.activeProjectId === project.id && appState.status.running;
      return `
        <button type="button" class="project-row ${isSelected ? 'selected' : ''}" data-action="select-project" data-id="${escapeHtml(project.id)}">
          <div class="project-copy">
            <strong>${escapeHtml(project.displayName)}</strong>
            <span class="project-type-tag">${escapeHtml(projectTypeLabel(project))}</span>
          </div>
          ${showRunning ? '<span class="project-running-badge">Running</span>' : ''}
        </button>
      `;
    })
    .join('');
}

function render() {
  if (state.fatalError) {
    root.innerHTML = `
      <main class="shell loading-state">
        <section class="hero-card" style="border-color: var(--danger);">
          <h1 style="color: var(--danger);">Initialization Error</h1>
          <p>${escapeHtml(state.fatalError)}</p>
          <button type="button" onclick="window.location.reload()" style="margin-top: 20px;">Reload Application</button>
        </section>
      </main>
    `;
    return;
  }

  if (!state.appState) {
    root.innerHTML = `
      <main class="shell loading-state">
        <section class="hero-card">
          <h1>Exposely</h1>
          <p>Loading backend state...</p>
        </section>
      </main>
    `;
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
  const shareToolStatusLabel = shareToolReady ? 'Installed' : 'Not installed';
  const shareToolStatusBadgeClass = shareToolReady ? 'pill-success' : 'pill-outline';
  const shareToolStatusMessage = shareToolReady
    ? 'cloudflared was detected and is ready to create a tunnel URL.'
    : 'Install cloudflared first before sharing projects.';
  const setupTunnelStatusLabel = canSetupTunnel ? 'Available' : 'Install required';
  const setupTunnelStatusBadgeClass = canSetupTunnel ? 'pill-success' : 'pill-outline';
  const setupTunnelStatusMessage = canSetupTunnel
    ? 'cloudflared is available for tunnel sharing.'
    : 'Install cloudflared first before using tunnel sharing.';
  const installBannerPath = appState.cloudflaredPath || 'cloudflared.exe (PATH)';
  const headerHint = !shareToolReady
    ? ''
    : !hasProjects
      ? 'Create a project first'
      : '';

  root.innerHTML = `
    <main class="shell">
      <aside class="sidebar">
        <div class="sidebar-header">
          <div class="logo">
            <img src="./logo.png?v=3" alt="App logo" />
            <div class="logo-text">
              <h1>Exposely</h1>
              <div class="status-indicator tone-${statusTone(tunnelStatus)}">
                <span class="status-dot"></span>
                <span>${escapeHtml(tunnelStatus.running ? 'Running' : 'Stopped')}</span>
              </div>
            </div>
          </div>
        </div>

        <nav class="sidebar-nav">
          <button type="button" class="nav-item ${state.activeTab === 'overview' ? 'active' : ''}" data-action="tab-overview">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M10 2L2 7L10 12L18 7L10 2Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M2 13L10 18L18 13" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <span>Overview</span>
          </button>
          <button type="button" class="nav-item ${state.activeTab === 'settings' ? 'active' : ''}" data-action="tab-settings">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="10" cy="10" r="3" stroke="currentColor" stroke-width="2"/>
              <path d="M10 2V4M10 16V18M18 10H16M4 10H2M15.66 4.34L14.24 5.76M5.76 14.24L4.34 15.66M15.66 15.66L14.24 14.24M5.76 5.76L4.34 4.34" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
            <span>Settings</span>
          </button>
          <button type="button" class="nav-item ${state.activeTab === 'about' ? 'active' : ''}" data-action="tab-about">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="12" y1="16" x2="12" y2="12"></line>
              <line x1="12" y1="8" x2="12.01" y2="8"></line>
            </svg>
            <span>About</span>
          </button>
        </nav>

        ${state.activeTab === 'overview' ? `
          <div class="sidebar-section sidebar-create-section">
            <div class="section-header">
              <h3>Projects</h3>
              <button type="button" class="add-button" data-action="new-project" title="New Project">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                  <line x1="12" y1="5" x2="12" y2="19"></line>
                  <line x1="5" y1="12" x2="19" y2="12"></line>
                </svg>
              </button>
            </div>
          </div>

          <div class="sidebar-section sidebar-projects-section">
            <div class="project-list">${projectRows(appState)}</div>
          </div>
        ` : '<div class="sidebar-section sidebar-projects-empty"></div>'}

        <div class="sidebar-footer">
          <div class="nav-item ${state.activeTab === 'logs' ? 'active' : ''} logs-nav-item" data-action="tab-logs">
            <span>Logs</span>
          </div>
        </div>
      </aside>

      <section class="main-content">
        ${!shareToolReady ? `
          <section class="install-banner" data-action="tab-settings">
            <div class="install-banner-copy">
              <span class="install-banner-label">Required setup</span>
              <strong>Install cloudflared first</strong>
              <p>This app will create a Cloudflare Tunnel URL for the selected local project.</p>
              <span class="install-banner-path">Expected path: ${escapeHtml(installBannerPath)}</span>
            </div>
            <div class="install-banner-action">
              <button type="button" class="danger-button" data-action="tab-settings">Open Settings</button>
            </div>
          </section>
        ` : ''}
        ${updateBanner(appState)}

        <header class="content-header">
          <div class="header-info">
            <h2>${state.activeTab === 'overview' ? 'Projects' : state.activeTab === 'settings' ? 'Settings' : state.activeTab === 'logs' ? 'Logs' : 'About'}</h2>
            ${state.busy ? `<p class="busy-indicator">${escapeHtml(state.busy)}</p>` : ''}
          </div>
          <div class="header-actions">
            <button type="button" data-action="start-tunnel" ${state.appState?.status.running || !canStartTunnel ? 'disabled' : ''}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"></polygon></svg>
              ${escapeHtml(shareAction?.label || 'Start Tunnel')}
            </button>
            <button type="button" class="secondary" data-action="stop-tunnel">
               <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="6" width="12" height="12"></rect></svg>
               Stop
            </button>
            <button type="button" class="secondary" data-action="refresh" title="Refresh state">
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
                    <p class="eyebrow">${state.editorMode === 'create' ? 'New project' : 'Edit project'}</p>
                    <h2>${escapeHtml(state.editorProject.displayName || 'Project preset')}</h2>
                  </div>
                  <button type="button" class="secondary" data-action="close-editor">Close</button>
                </div>
                <form id="project-form" class="form-grid editor-grid">
                  <input type="hidden" name="id" value="${escapeHtml(state.editorProject.id)}" />
                  <label class="wide">Display name<input name="displayName" value="${escapeHtml(state.editorProject.displayName)}" /></label>
                  <label class="wide">
                    ${state.editorProject.shareMode === 'quick' ? 'Project folder' : 'Project folder (optional when Local URL is set)'}
                    <div class="folder-picker">
                      <input
                        name="projectPath"
                        value="${escapeHtml(state.editorProject.projectPath)}"
                        placeholder="${state.editorProject.shareMode === 'quick' ? 'D:\\code\\hr-system' : 'D:\\code\\site'}"
                      />
                      <button type="button" class="secondary browse-button" data-action="browse-project-folder">Browse</button>
                    </div>
                  </label>
                  <label>
                    Project Type
                    <select name="shareMode">
                      <option value="auto" ${state.editorProject.shareMode === 'auto' ? 'selected' : ''}>Auto Detect Project</option>
                      <option value="quick" ${state.editorProject.shareMode === 'quick' ? 'selected' : ''}>Local Host Project</option>
                      <option value="host-html" ${state.editorProject.shareMode === 'host-html' ? 'selected' : ''}>HTML Project (Folder or Local URL)</option>
                    </select>
                  </label>
                  ${
                    state.editorProject.shareMode === 'quick' || state.editorProject.shareMode === 'auto'
                      ? `<label>${state.editorProject.shareMode === 'auto' ? 'Local host (optional)' : 'Local host'}<input name="localHost" value="${escapeHtml(state.editorProject.localHost)}" placeholder="app.test" /></label>`
                      : ''
                  }
                  ${
                    state.editorProject.shareMode === 'auto' || state.editorProject.shareMode === 'host-html'
                      ? `<label>Local URL (optional)<input name="localURL" value="${escapeHtml(state.editorProject.localURL)}" placeholder="http://127.0.0.1:5500" /></label>`
                      : ''
                  }
                  ${
                    state.editorProject.shareMode === 'quick' || state.editorProject.shareMode === 'auto'
                      ? `<label>Origin URL (optional)<input name="originURL" value="${escapeHtml(state.editorProject.originURL)}" placeholder="http://127.0.0.1:80" /></label>`
                      : ''
                  }
                  ${
                    state.editorProject.shareMode === 'auto'
                      ? `<label class="wide">Start command (optional)<input name="startCommand" value="${escapeHtml(state.editorProject.startCommand)}" placeholder="npm run dev -- --port 4173" /></label>`
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
                         <span>Share either a local HTML folder or an already running local URL like http://127.0.0.1:5500 via Cloudflare Tunnel.</span>
                        </div>
                        `
                      : state.editorProject.shareMode === 'auto'
                        ? `
                        <div class="field-hint hint-info">
                          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                           <circle cx="7" cy="7" r="6" stroke="currentColor" stroke-width="1.2"/>
                           <path d="M7 10V7M7 4H7.01" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
                         </svg>
                         <span>Auto mode can use a local URL, run a start command and detect the dev server, use a local host, auto-detect Laravel folders, or serve a static folder/build output.</span>
                        </div>
                        `
                       : `
                        <div class="field-hint hint-success">
                          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                           <circle cx="7" cy="7" r="6" stroke="currentColor" stroke-width="1.2"/>
                           <path d="M4.5 7L6 8.5L9.5 5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
                         </svg>
                         <span>Expose a local host such as app.test through a public URL.</span>
                       </div>
                     `
                   }
                  <div class="action-row wide"><button type="submit">${state.editorMode === 'create' ? 'Save Project' : 'Update Project'}</button></div>
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
                      <h2>${escapeHtml(project?.displayName || 'No project selected')}</h2>
                    </div>
                    <div class="selected-header-actions">
                      ${
                        project
                          ? `
                            <div class="dropdown">
                              <button type="button" class="secondary menu-button" data-action="toggle-project-menu">Manage</button>
                              ${
                                state.projectMenuOpen
                                  ? `
                                    <div class="dropdown-menu">
                                      <button type="button" class="dropdown-item" data-action="edit-project">
                                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                                          <path d="M11.333 2.667a.943.943 0 0 1 1.333 1.333l-6.4 6.4L3.333 11.333l.933-2.933 6.4-5.733Z" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
                                        </svg>
                                        <span>Edit project</span>
                                      </button>
                                      <button type="button" class="dropdown-item danger-item" data-action="delete-project">
                                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                                          <path d="M2.667 4h10.666M6 2.667h4M5.333 6v5.333M8 6v5.333M10.667 6v5.333M4.667 4l.4 8A1.333 1.333 0 0 0 6.4 13.333h3.2A1.333 1.333 0 0 0 10.933 12l.4-8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
                                        </svg>
                                        <span>Delete project</span>
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
                              <p>${escapeHtml(projectUrl || 'No public URL is available yet')}</p>
                              ${projectUrl ? `<button type="button" class="secondary inline-copy-button" data-action="open-url" aria-label="Open public URL">
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>
                                Open
                              </button>` : ''}
                              ${!projectUrl && project.shareMode !== 'stable' ? `<button type="button" class="secondary inline-copy-button" data-action="regenerate-url" aria-label="Generate public URL" ${!shareToolReady ? 'disabled' : ''}>
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M23 4v6h-6"></path><path d="M1 20v-6h6"></path><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
                                Refresh URL
                              </button>` : ''}
                            </div>
                          </div>
                        </div>

                        ${renderUsagePanel(tunnelStatus)}
                      `
                      : '<p class="empty-copy">Create or select a project to start sharing.</p>'
                  }
                </article>
              `
                 : state.activeTab === 'settings'
                 ? `
                    <article class="panel compact-panel">
                      <div class="panel-header">
                        <div>
                          <p class="eyebrow">Setup</p>
                          <h2>Share tool and tunnel setup</h2>
                        </div>
                      </div>
                      <div class="metric-grid">
                        <div class="metric-card metric-card-split">
                          <div class="metric-card-copy">
                            <span class="summary-label">Share Tool</span>
                            <strong>${escapeHtml(shareToolStatusLabel)} <span class="pill ${shareToolStatusBadgeClass}">${escapeHtml(shareToolStatusLabel)}</span></strong>
                            <p>${escapeHtml(shareToolStatusMessage)}</p>
                          </div>
                          <div class="action-row">
                            ${
                              shareToolReady
                                ? '<button type="button" class="secondary" data-action="ensure-cloudflared">Recheck cloudflared</button>'
                                : `
                                  <button type="button" data-action="install-cloudflared">Install cloudflared</button>
                                  <button type="button" class="secondary" data-action="ensure-cloudflared">Check setup</button>
                                `
                            }
                          </div>
                        </div>
                        <div class="metric-card metric-card-split">
                          <div class="metric-card-copy">
                            <span class="summary-label">Setup tunnel</span>
                            <strong>${escapeHtml(setupTunnelStatusLabel)} <span class="pill ${setupTunnelStatusBadgeClass}">${escapeHtml(setupTunnelStatusLabel)}</span></strong>
                            <p>${escapeHtml(setupTunnelStatusMessage)}</p>
                          </div>
                          <div class="action-row">
                            <button type="button" class="secondary" disabled>Not needed</button>
                          </div>
                        </div>
                      </div>
                    </article>

                 <article class="panel compact-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Defaults</p>
                        <h2>App settings</h2>
                      </div>
                    </div>
                    <form id="settings-form" class="form-grid">
                      <label>Custom cloudflared path (optional)<input name="cloudflaredPath" value="${escapeHtml(appState.settings.cloudflaredPath)}" placeholder="Leave blank to use default" /></label>
                      <label>Local service URL<input name="defaultServiceURL" value="${escapeHtml(appState.settings.defaultServiceURL)}" /></label>
                      <div class="action-row wide"><button type="submit">Save Settings</button></div>
                    </form>
                  </article>
                 `
                : state.activeTab === 'logs'
                ? `
                  <section class="panel logs-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Logs</p>
                        <h2>cloudflared and npm output</h2>
                      </div>
                      <span class="pill pill-outline">${escapeHtml(`${appState.status.lastLogs.length} entries`)}</span>
                    </div>
                    <div class="log-stream">${logRows(appState.status.lastLogs)}</div>
                  </section>
                `
               : `
                  <article class="panel compact-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Application</p>
                        <h2>Exposely</h2>
                      </div>
                    </div>
                    <div class="metric-grid">
                      <div class="metric-card">
                         <span class="summary-label">Version</span>
                         <strong>${escapeHtml(displayVersionLabel(appState.productVersion))}</strong>
                      </div>
                      <div class="metric-card">
                         <span class="summary-label">Platform</span>
                         <strong>Windows</strong>
                      </div>
                    </div>
                    <div class="metric-grid" style="margin-top: 20px;">
                      <div class="metric-card">
                        <span class="summary-label">Updates</span>
                        <strong>${escapeHtml(appState.update.available ? `${displayVersionLabel(appState.update.latestVersion)} available` : appState.update.checked ? 'Up to date' : 'Checking in background')}</strong>
                        <p style="margin-top: 10px; color: var(--text-secondary);">${escapeHtml(appState.update.message || 'The app can check GitHub releases for new versions.')}</p>
                        <div class="action-row" style="margin-top: 14px;">
                          <button type="button" class="secondary" data-action="check-updates">Check Now</button>
                          <button type="button" ${appState.update.available ? '' : 'disabled'} data-action="install-latest-update">Update Now</button>
                          <button type="button" ${appState.update.releaseUrl ? '' : 'disabled'} data-action="open-latest-release">Open Release</button>
                        </div>
                      </div>
                    </div>
                    <div style="padding: 24px; color: var(--text-secondary); line-height: 1.6;">
                      <p>Manage your Cloudflare Tunnels easily. This application provides a graphical interface to create and manage tunnel URLs for your local projects using Cloudflare's <code>cloudflared</code> tool.</p>
                      <p style="margin-top: 16px;">Created for developers to simplify the process of sharing local services with the world securely.</p>
                      <p style="margin-top: 16px; color: var(--text-primary); font-weight: 500;">License developed by Reaksmey Kem</p>
                    </div>
                  </article>
               `
          }

        </section>
      </section>

    </main>
  `;

  bindForms();
}

function formValue(form: HTMLFormElement, name: string): string {
  return (form.elements.namedItem(name) as HTMLInputElement | HTMLSelectElement | null)?.value?.trim() ?? '';
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

  return window.confirm('Another project is currently running. Stop the current project and continue with this one?');
}

function bindForms() {
  const licenseForm = root.querySelector<HTMLFormElement>('#license-form');
  licenseForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const token = formValue(licenseForm, 'licenseToken');
    state.licenseDraft = token;
    const next = await withAction('Activating license...', () => api.activateLicense(token));
    if (next) {
      state.appState = next;
      state.licenseDraft = '';
      if (next.license.isAdmin && state.activeTab !== 'settings') {
        state.activeTab = 'settings';
      }
      setNotice('success', next.license.message || 'License activated');
    }
  });

  const settingsForm = root.querySelector<HTMLFormElement>('#settings-form');
  settingsForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
      const payload = {
        ...state.appState!.settings,
        cloudflaredPath: formValue(settingsForm, 'cloudflaredPath'),
        defaultServiceURL: formValue(settingsForm, 'defaultServiceURL'),
      };
    const next = await withAction('Saving settings...', () => api.saveSettings(payload));
    if (next) {
      state.appState = next;
      setNotice('success', 'Settings saved');
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
    const next = await withAction('Saving project...', () => api.saveProject(payload));
    if (!next) return;

    state.appState = next;
    const savedProjectId = payload.id || (next.settings.projects[next.settings.projects.length - 1]?.id ?? null);
    state.selectedProjectId = savedProjectId;
    state.editorOpen = false;
    setNotice('success', 'Project preset saved');
  });
}

async function handleAction(action: string, id: string | null) {
  switch (action) {
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
      const next = await withAction('Removing license...', () => api.clearLicense());
      if (next) {
        state.appState = next;
        state.licenseDraft = '';
        state.activeTab = 'settings';
        setNotice('success', 'License removed');
      }
      return;
    }
    case 'browse-project-folder': {
      syncEditorFromForm();
      const result = await withAction('Opening folder picker...', () => api.browseProjectFolder(state.editorProject.projectPath));
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
    case 'copy-url': {
      state.projectMenuOpen = false;
      const project = selectedProject();
      if (!project || !state.appState) return;
      const url = resolvedProjectURL(project, state.appState);
      if (!url) {
        setNotice('error', 'No public URL is available for the selected project');
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
    setNotice('error', 'Select a project first');
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

      const refreshed = await withAction('Checking tunnel state...', () => api.refreshState());
      if (!refreshed) return;
      state.appState = refreshed;

      if (refreshed.status.running) {
        setNotice('info', 'A tunnel is already running. Stop it first before starting again.');
        return;
      }

      try {
        setBusy('Starting tunnel...');
        const next = await api.startTunnel();
        state.appState = next;
        setNotice('success', 'Named tunnel started');
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        if (message.toLowerCase().includes('already running')) {
          const latest = await api.refreshState().catch(() => null);
          if (latest) {
            state.appState = latest;
          }
          setNotice('info', 'A tunnel is already running. Stop it first before starting again.');
        } else {
          setNotice('error', message);
        }
      } finally {
        setBusy(null);
      }
      return;
    }
    case 'ensure-cloudflared': {
      const next = await withAction('Checking cloudflared setup...', () => api.ensureCloudflared());
      if (next) {
        state.appState = next;
        setNotice('success', 'cloudflared is ready to create public URLs.');
      }
      return;
    }
    case 'install-cloudflared': {
      const next = await withAction('Installing cloudflared...', () => api.installCloudflared());
      if (next) {
        state.appState = next;
        setNotice('success', 'cloudflared installed successfully.');
      }
      return;
    }
    case 'stop-tunnel': {
      const next = await withAction('Stopping tunnel...', () => api.stopTunnel());
      if (next) {
        state.appState = next;
        state.activeProjectId = null;
        setNotice('success', 'Tunnel stopped');
      }
      return;
    }
    case 'create-tunnel': {
      const next = await withAction('Ensuring named tunnel...', () => api.createTunnel());
      if (next) {
        state.appState = next;
        setNotice('success', 'Named tunnel is ready');
      }
      return;
    }
    case 'open-config':
      await withAction('Opening config file...', () => api.openConfigFile());
      return;
    case 'open-settings':
      await withAction('Opening settings file...', () => api.openSettingsFile());
      return;
    case 'open-latest-release':
      await withAction('Opening latest release...', () => api.openLatestRelease());
      return;
    case 'install-latest-update':
      {
        const message = await withAction('Downloading and opening the latest update...', () => api.installLatestUpdate());
        if (typeof message === 'string' && message) {
          setNotice('success', message);
        }
      }
      return;
    case 'check-updates': {
      const next = await withAction('Checking for updates...', () => api.checkForUpdates());
      if (next) {
        state.appState = next;
        setNotice(next.update.available ? 'info' : 'success', next.update.message || 'Update check finished');
      }
      return;
    }
    case 'refresh': {
      const next = await withAction('Refreshing state...', () => api.refreshState());
      if (next) {
        state.appState = next;
        if (!state.activeProjectId && next.status.running) {
          state.activeProjectId = inferActiveProjectId(next);
        }
        setNotice('success', 'State refreshed');
      }
      return;
    }
    case 'share-project': {
      if (!(await confirmProjectSwitch(id!))) return;
      if (state.appState?.status.running && state.activeProjectId && state.activeProjectId !== id) {
        const stopped = await withAction('Stopping current project...', () => api.stopTunnel());
        if (!stopped) return;
        state.appState = stopped;
        state.activeProjectId = null;
      }
      const next = await withAction('Sharing project...', () => api.shareProject(id!));
      if (next) {
        state.appState = next;
        state.activeProjectId = id!;
        if (next.status.activeUrl || next.status.quickUrl) {
          state.projectUrls[id!] = next.status.activeUrl || next.status.quickUrl;
        }
        setNotice('success', 'Project shared through named tunnel');
      }
      return;
    }
    case 'share-random': {
      if (!(await confirmProjectSwitch(id!))) return;
      if (state.appState?.status.running && state.activeProjectId && state.activeProjectId !== id) {
        const stopped = await withAction('Stopping current project...', () => api.stopTunnel());
        if (!stopped) return;
        state.appState = stopped;
        state.activeProjectId = null;
      }
      const next = await withAction('Generating random hostname...', () => api.shareProjectWithRandomURL(id!));
      if (next) {
        state.appState = next;
        state.activeProjectId = id!;
        if (next.settings.projects) {
          const savedProject = next.settings.projects.find((project) => project.id === id);
          if (savedProject?.publicURL) {
            state.projectUrls[id!] = savedProject.publicURL;
          }
        }
        setNotice('success', 'Random domain share is active');
      }
      return;
    }
    case 'share-quick': {
      if (!(await confirmProjectSwitch(id!))) return;
      if (state.appState?.status.running && state.activeProjectId && state.activeProjectId !== id) {
        const stopped = await withAction('Stopping current project...', () => api.stopTunnel());
        if (!stopped) return;
        state.appState = stopped;
        state.activeProjectId = null;
      }
      const next = await withAction('Creating Cloudflare tunnel URL...', () => api.startQuickTunnel(id!));
      if (next) {
        state.appState = next;
        state.activeProjectId = id!;
        delete state.projectUrls[id!];
        if (next.status.activeUrl || next.status.quickUrl) {
          state.projectUrls[id!] = next.status.activeUrl || next.status.quickUrl;
        }
        setNotice('success', 'Cloudflare tunnel URL is live');
      }
      return;
    }
    case 'regenerate-url': {
      const project = selectedProject();
      if (!project) return;
      if (!(await confirmProjectSwitch(project.id))) return;
      if (state.appState?.status.running) {
        const stopped = await withAction('Stopping current project...', () => api.stopTunnel());
        if (!stopped) return;
        state.appState = stopped;
        state.activeProjectId = null;
      }

      const next = await withAction(
        'Generating new Cloudflare tunnel URL...',
        () => api.startQuickTunnel(project.id),
      );
        if (next) {
          state.appState = next;
          state.activeProjectId = project.id;
          delete state.projectUrls[project.id];
          if (next.status.activeUrl || next.status.quickUrl) {
            state.projectUrls[project.id] = next.status.activeUrl || next.status.quickUrl;
          }
          setNotice('success', 'New public URL generated');
        }
        return;
    }
    case 'open-url':
      await withAction('Opening public URL...', () => api.openPublicURL(id!));
      return;
    case 'npm-build': {
      const result = await withAction('Running npm build...', () => api.runNpmBuild(id!));
      if (result !== undefined) setNotice('success', 'npm build started. Watch the log stream for progress');
      return;
    }
    case 'test-project': {
      const result = await withAction('Testing local URL...', () => api.testProject(id!));
      if (typeof result === 'string') setNotice('success', result);
      return;
    }
    case 'delete-project':
      if (window.confirm('Delete this project preset?')) {
        const next = await withAction('Deleting project...', () => api.deleteProject(id!));
        if (next) {
          state.appState = next;
          state.selectedProjectId = next.settings.projects[0]?.id ?? null;
          state.projectMenuOpen = false;
          setNotice('success', 'Project deleted');
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

  const next = await withAction('Loading app state...', () => api.bootstrap());
  if (!next) {
    state.fatalError = 'Failed to load initial application state. Please ensure the backend is running.';
    render();
    return;
  }
  state.appState = next;
  syncProjectUrlsFromState(next);
  state.selectedProjectId = next.settings.projects[0]?.id ?? null;
  state.activeProjectId = inferActiveProjectId(next);
  render();

  void api.checkForUpdates().then((latest) => {
    state.appState = latest;
    if (latest.update.available) {
      setNotice('info', latest.update.message || `Exposely ${displayVersionLabel(latest.update.latestVersion)} is available.`);
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
