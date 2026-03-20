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
  selectedProjectId: string | null;
  activeProjectId: string | null;
  projectUrls: Record<string, string>;
  activeTab: 'overview' | 'settings' | 'logs' | 'setup';
  projectMenuOpen: boolean;
  editorOpen: boolean;
  editorMode: 'create' | 'edit';
  editorProject: ProjectPreset;
  notice: Notice | null;
  busy: string | null;
}

const blankProject = (): ProjectPreset => ({
  id: '',
  displayName: '',
  localHost: '',
  subdomain: '',
  publicURL: '',
  projectPath: '',
  shareMode: 'quick',
});

const state: UIState = {
  appState: null,
  selectedProjectId: null,
  activeProjectId: null,
  projectUrls: {},
  activeTab: 'overview',
  projectMenuOpen: false,
  editorOpen: false,
  editorMode: 'create',
  editorProject: blankProject(),
  notice: null,
  busy: null,
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
  if (project.subdomain && project.shareMode !== 'quick') return `https://${project.subdomain}.${domain}`;
  return '';
}

function inferLocalHostFromPath(projectPath: string): string {
  const normalized = projectPath.trim().replace(/[/\\]+$/, '');
  if (!normalized) return '';

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
    if (project.publicURL?.trim()) {
      state.projectUrls[project.id] = project.publicURL.trim();
    }
  }
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
    case 'random-domain':
      return 'Random domain';
    case 'quick':
      return 'Public URL';
    default:
      return 'Stable';
  }
}

function statusTone(status: TunnelStatus): string {
  if (status.running) return 'success';
  if (status.lastError) return 'error';
  return 'idle';
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
      const monogram = (project.displayName || project.localHost || 'P').slice(0, 2).toUpperCase();
      const showRunning = state.activeProjectId === project.id && appState.status.running;
      return `
        <button type="button" class="project-row ${isSelected ? 'selected' : ''}" data-action="select-project" data-id="${escapeHtml(project.id)}">
          <div class="project-avatar">${escapeHtml(monogram)}</div>
          <div class="project-copy">
            <strong>${escapeHtml(project.displayName)}</strong>
            <span>${escapeHtml(project.localHost)}</span>
          </div>
          ${showRunning ? '<span class="project-running-badge">Running</span>' : ''}
        </button>
      `;
    })
    .join('');
}

function render() {
  if (!state.appState) {
    root.innerHTML = `
      <main class="shell loading-state">
        <section class="hero-card">
          <h1>Cloudflare Tunnel Manager</h1>
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
  const storedProjectUrl = project ? (state.projectUrls[project.id] || project.publicURL || '') : '';
  const liveProjectUrl = state.activeProjectId === project?.id ? (tunnelStatus.activeUrl || tunnelStatus.quickUrl || '') : '';
  const activeUrl = liveProjectUrl || storedProjectUrl;
  const projectUrl = project ? formatProjectURL(project, appState.settings.defaultDomain) : '';
  const hasProjects = appState.settings.projects.length > 0;
  const shareToolReady = appState.cloudflaredDetected;
  const canSetupTunnel = shareToolReady;
  const canStartTunnel = shareToolReady && hasProjects;
  const headerHint = !shareToolReady
    ? 'Prepare Share Tool first'
    : !hasProjects
      ? 'Create a project first'
      : '';

  root.innerHTML = `
    <main class="shell">
      <aside class="sidebar">
        <div class="sidebar-header">
          <div class="logo">
            <svg width="32" height="32" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect width="32" height="32" rx="8" fill="#F48120"/>
              <path d="M16 8L24 16L16 24L8 16L16 8Z" fill="white"/>
            </svg>
            <div>
              <h1>Cloudflare Tunnel</h1>
              <p>Manager</p>
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
          <button type="button" class="nav-item ${state.activeTab === 'setup' ? 'active' : ''}" data-action="tab-setup">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M10 3L16 6.5V13.5L10 17L4 13.5V6.5L10 3Z" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
              <path d="M10 8V10.5" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              <circle cx="10" cy="13" r="1" fill="currentColor"/>
            </svg>
            <span>Setup</span>
          </button>
          <button type="button" class="nav-item ${state.activeTab === 'logs' ? 'active' : ''}" data-action="tab-logs">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M3 5H17M3 10H17M3 15H17" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
            <span>Logs</span>
          </button>
        </nav>

        <div class="sidebar-section sidebar-create-section">
          <div class="section-header">
            <h3>Projects</h3>
            <button type="button" class="add-button" data-action="new-project">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M8 3V13M3 8H13" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
            </button>
          </div>
        </div>

        <div class="sidebar-section sidebar-projects-section">
          <div class="project-list">${projectRows(appState)}</div>
        </div>

        <div class="sidebar-footer">
          <div class="status-indicator tone-${statusTone(tunnelStatus)}">
            <span class="status-dot"></span>
            <span>${escapeHtml(tunnelStatus.running ? 'Running' : 'Stopped')}</span>
          </div>
        </div>
      </aside>

      <section class="main-content">
        <header class="content-header">
          <div class="header-info">
            <h2>${state.activeTab === 'overview' ? 'Overview' : state.activeTab === 'settings' ? 'Settings' : state.activeTab === 'setup' ? 'Setup' : 'Logs'}</h2>
          </div>
          <div class="header-actions">
            <button type="button" data-action="start-tunnel" ${state.appState?.status.running || !canStartTunnel ? 'disabled' : ''}>Start Tunnel</button>
            <button type="button" class="secondary" data-action="stop-tunnel">Stop</button>
            <button type="button" class="secondary" data-action="refresh">Refresh</button>
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
                  <label>Display name<input name="displayName" value="${escapeHtml(state.editorProject.displayName)}" /></label>
                  <label>
                    Project folder
                    <div class="folder-picker">
                      <input name="projectPath" value="${escapeHtml(state.editorProject.projectPath)}" placeholder="D:\\code\\hr-system" />
                      <button type="button" class="secondary browse-button" data-action="browse-project-folder">Browse</button>
                    </div>
                  </label>
                  <label>Local host<input name="localHost" value="${escapeHtml(state.editorProject.localHost)}" placeholder="hr-system.test" /></label>
                  <label>
                    Share mode
                    <select name="shareMode">
                      <option value="quick" ${state.editorProject.shareMode === 'quick' ? 'selected' : ''}>One-click public URL</option>
                      <option value="stable" ${state.editorProject.shareMode === 'stable' ? 'selected' : ''}>Stable hostname</option>
                      <option value="random-domain" ${state.editorProject.shareMode === 'random-domain' ? 'selected' : ''}>Random under my domain</option>
                    </select>
                  </label>
                  ${
                    state.editorProject.shareMode === 'stable'
                      ? `
                        <label>
                          Stable subdomain
                          <div class="folder-picker">
                            <input name="subdomain" value="${escapeHtml(state.editorProject.subdomain)}" placeholder="app" />
                            <button type="button" class="secondary browse-button" data-action="random-subdomain">Random</button>
                          </div>
                        </label>
                      `
                      : ''
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
                            <strong>${escapeHtml(project.localHost)}</strong>
                            <div class="inline-url-row">
                              <p>${escapeHtml(activeUrl || projectUrl || 'No public URL is available yet')}</p>
                              ${(activeUrl || projectUrl) ? `<button type="button" class="secondary inline-copy-button" data-action="copy-url" aria-label="Copy public URL">Copy</button>` : ''}
                            </div>
                          </div>
                        </div>

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
                        <p class="eyebrow">Defaults</p>
                        <h2>App settings</h2>
                      </div>
                    </div>
                    <form id="settings-form" class="form-grid">
                      <label>Default domain<input name="defaultDomain" value="${escapeHtml(appState.settings.defaultDomain)}" /></label>
                      <label>Tunnel name<input name="tunnelName" value="${escapeHtml(appState.settings.tunnelName)}" /></label>
                      <label>Cloudflared path<input name="cloudflaredPath" value="${escapeHtml(appState.settings.cloudflaredPath)}" placeholder="Leave blank to use PATH" /></label>
                      <label>Local service URL<input name="defaultServiceURL" value="${escapeHtml(appState.settings.defaultServiceURL)}" /></label>
                      <label>Managed cloudflared location<input value="${escapeHtml(appState.managedCloudflaredPath)}" disabled /></label>
                      <div class="action-row wide"><button type="submit">Save Settings</button></div>
                    </form>
                  </article>

                  <article class="panel compact-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Tunnel context</p>
                        <h2>Routes and runtime</h2>
                      </div>
                      <span class="pill ${tunnelStatus.running ? 'pill-success' : 'pill-muted'}">${escapeHtml(tunnelStatus.running ? 'running' : 'stopped')}</span>
                    </div>
                    <div class="status-grid two-column">
                      <div><label>Config file</label><strong>${escapeHtml(appState.configPath)}</strong></div>
                      <div><label>Cloudflared path</label><strong>${escapeHtml(appState.cloudflaredPath || 'not detected')}</strong></div>
                    </div>
                    <div class="status-list">
                      <label>Loaded hostnames</label>
                      <div class="host-tags">
                        ${tunnelStatus.activeHostnames.length ? tunnelStatus.activeHostnames.map((hostname) => `<span>${escapeHtml(hostname)}</span>`).join('') : '<span>none</span>'}
                      </div>
                    </div>
                    <div class="action-row">
                      <button type="button" class="secondary" data-action="open-config">Open Config</button>
                      <button type="button" class="secondary" data-action="open-settings">Open Settings</button>
                    </div>
                    ${tunnelStatus.lastError ? `<p class="error-copy">${escapeHtml(tunnelStatus.lastError)}</p>` : ''}
                  </article>
                `
                : state.activeTab === 'setup'
                  ? `
                    <article class="panel compact-panel">
                      <div class="panel-header">
                        <div>
                          <p class="eyebrow">Setup</p>
                          <h2>Share tool and tunnel setup</h2>
                        </div>
                      </div>
                      <div class="metric-grid">
                        <div class="metric-card">
                          <span class="summary-label">Prepare share tool</span>
                          <strong>${escapeHtml(appState.cloudflaredDetected ? 'Ready' : 'Required')}</strong>
                          <p>Prepare cloudflared before starting or sharing projects.</p>
                          <div class="action-row">
                            <button type="button" class="${shareToolReady ? 'secondary' : ''}" data-action="ensure-cloudflared">Prepare Share Tool</button>
                          </div>
                        </div>
                        <div class="metric-card">
                          <span class="summary-label">Setup tunnel</span>
                          <strong>${escapeHtml(canSetupTunnel ? 'Available' : 'Waiting')}</strong>
                          <p>Create or reuse the named tunnel after the share tool is ready.</p>
                          <div class="action-row">
                            <button type="button" class="secondary" data-action="create-tunnel" ${!canSetupTunnel ? 'disabled' : ''}>Setup Tunnel</button>
                          </div>
                        </div>
                      </div>
                    </article>
                  `
                : `
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
  const validShareModes: ShareMode[] = ['quick', 'stable', 'random-domain'];
  const shareMode = validShareModes.includes(shareModeValue) ? shareModeValue : 'stable';
  const projectPath = formValue(projectForm, 'projectPath');
  const localHost = formValue(projectForm, 'localHost') || inferLocalHostFromPath(projectPath);

  state.editorProject = {
    id: formValue(projectForm, 'id'),
    displayName: formValue(projectForm, 'displayName'),
    localHost,
    projectPath,
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
  const settingsForm = root.querySelector<HTMLFormElement>('#settings-form');
  settingsForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const payload = {
      ...state.appState!.settings,
      defaultDomain: formValue(settingsForm, 'defaultDomain'),
      tunnelName: formValue(settingsForm, 'tunnelName'),
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

    if (target.name === 'shareMode' || target.name === 'projectPath' || target.name === 'localHost') {
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
      projectPath: formValue(projectForm, 'projectPath'),
      subdomain: formValue(projectForm, 'subdomain'),
      publicURL: state.editorProject.publicURL,
      shareMode: (formValue(projectForm, 'shareMode') as ShareMode) || 'stable',
    };
    const next = await withAction('Saving project...', () => api.saveProject(payload));
    if (!next) return;

    state.appState = next;
    const savedProjectId = payload.id || (next.settings.projects[next.settings.projects.length - 1]?.id ?? null);
    state.selectedProjectId = savedProjectId;
    state.editorOpen = false;

    if (!savedProjectId) {
      setNotice('success', 'Project preset saved');
      return;
    }

    if (!(await confirmProjectSwitch(savedProjectId))) {
      setNotice('success', 'Project preset saved');
      return;
    }

    if (state.appState?.status.running && state.activeProjectId && state.activeProjectId !== savedProjectId) {
      const stopped = await withAction('Stopping current project...', () => api.stopTunnel());
      if (!stopped) return;
      state.appState = stopped;
      state.activeProjectId = null;
    }

    const shareMode = payload.shareMode;
    const shared = await withAction(
      shareMode === 'quick'
        ? 'Creating public URL...'
        : shareMode === 'random-domain'
          ? 'Generating random hostname...'
          : 'Sharing project...',
      () => {
        if (shareMode === 'quick') return api.startQuickTunnel(savedProjectId);
        if (shareMode === 'random-domain') return api.shareProjectWithRandomURL(savedProjectId);
        return api.shareProject(savedProjectId);
      },
    );

    if (shared) {
      state.appState = shared;
      state.activeProjectId = savedProjectId;
      setNotice('success', 'Project saved and shared successfully');
      return;
    }

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
    case 'tab-setup':
      state.activeTab = 'setup';
      state.projectMenuOpen = false;
      render();
      return;
    case 'tab-logs':
      state.activeTab = 'logs';
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
    case 'browse-project-folder': {
      syncEditorFromForm();
      const result = await withAction('Opening folder picker...', () => api.browseProjectFolder(state.editorProject.projectPath));
      if (typeof result === 'string' && result) {
        const inferredLocalHost = inferLocalHostFromPath(result);
        state.editorProject.projectPath = result;
        if (!state.editorProject.localHost.trim()) {
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
      const url = formatProjectURL(project, state.appState.settings.defaultDomain);
      if (!url) {
        setNotice('error', 'No public URL is available for the selected project');
        return;
      }
      await navigator.clipboard.writeText(url);
      return;
    }
  }

  if (!id && ['share-project', 'share-random', 'share-quick', 'regenerate-url', 'open-url', 'npm-build', 'test-project', 'delete-project'].includes(action)) {
    setNotice('error', 'Select a project first');
    return;
  }

  switch (action) {
    case 'start-tunnel': {
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
      const next = await withAction('Preparing cloudflared...', () => api.ensureCloudflared());
      if (next) {
        state.appState = next;
        setNotice('success', 'cloudflared is ready for one-click sharing');
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
    case 'refresh': {
      const next = await withAction('Refreshing state...', () => api.refreshState());
      if (next) {
        state.appState = next;
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
      const next = await withAction('Creating public URL...', () => api.startQuickTunnel(id!));
      if (next) {
        state.appState = next;
        state.activeProjectId = id!;
        if (next.status.activeUrl || next.status.quickUrl) {
          state.projectUrls[id!] = next.status.activeUrl || next.status.quickUrl;
        }
        setNotice('success', 'Public URL is live');
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
        project.shareMode === 'quick' ? 'Generating new public URL...' : 'Generating new random URL...',
        () => project.shareMode === 'quick' ? api.startQuickTunnel(project.id) : api.shareProjectWithRandomURL(project.id),
      );
      if (next) {
        state.appState = next;
        state.activeProjectId = project.id;
        if (next.status.activeUrl || next.status.quickUrl) {
          state.projectUrls[project.id] = next.status.activeUrl || next.status.quickUrl;
        }
        if (next.settings.projects) {
          const savedProject = next.settings.projects.find((item) => item.id === project.id);
          if (savedProject?.publicURL) {
            state.projectUrls[project.id] = savedProject.publicURL;
          }
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
  const insideDropdown = target?.closest('.dropdown');
  if (state.projectMenuOpen && !insideDropdown) {
    state.projectMenuOpen = false;
    render();
    return;
  }

  const actionElement = target?.closest<HTMLElement>('[data-action]');
  if (!actionElement) return;

  event.preventDefault();
  const action = actionElement.dataset.action;
  const id = actionElement.dataset.id ?? selectedProject()?.id ?? null;
  if (!action) return;
  void handleAction(action, id);
});

async function bootstrap() {
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
  if (!next) return;
  state.appState = next;
  syncProjectUrlsFromState(next);
  state.selectedProjectId = next.settings.projects[0]?.id ?? null;
  state.activeProjectId = null;
  render();

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

bootstrap();
