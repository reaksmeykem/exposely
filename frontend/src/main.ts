import './style.css';
import { api } from './api';
import type { AppState, LogEntry, ProjectPreset, ShareMode, TunnelStatus } from './types';

type NoticeTone = 'info' | 'success' | 'error';

interface Notice {
  tone: NoticeTone;
  message: string;
}

interface UIState {
  appState: AppState | null;
  selectedProjectId: string | null;
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
  shareMode: 'stable',
});

const state: UIState = {
  appState: null,
  selectedProjectId: null,
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

function selectedProject(): ProjectPreset | null {
  const projects = state.appState?.settings.projects ?? [];
  if (projects.length === 0) return null;
  if (!state.selectedProjectId) return projects[0];
  return projects.find((project) => project.id === state.selectedProjectId) ?? projects[0];
}

function setNotice(tone: NoticeTone, message: string) {
  state.notice = { tone, message };
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
      return 'Quick tunnel';
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
      const url = formatProjectURL(project, appState.settings.defaultDomain);
      return `
        <button type="button" class="project-row ${isSelected ? 'selected' : ''}" data-action="select-project" data-id="${escapeHtml(project.id)}">
          <div class="project-row-top">
            <div>
              <strong>${escapeHtml(project.displayName)}</strong>
              <span>${escapeHtml(project.localHost)}</span>
            </div>
            <span class="mini-pill">${escapeHtml(shareModeLabel(project.shareMode))}</span>
          </div>
          <div class="project-row-bottom">
            <span>${escapeHtml(project.subdomain || 'dynamic')}</span>
            <span>${escapeHtml(url || 'not shared yet')}</span>
          </div>
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
  const project = selectedProject();
  state.selectedProjectId = project?.id ?? null;
  const tunnelStatus = appState.status;
  const activeUrl = tunnelStatus.activeUrl || tunnelStatus.quickUrl || project?.publicURL || '';
  const projectUrl = project ? formatProjectURL(project, appState.settings.defaultDomain) : '';

  root.innerHTML = `
    <main class="shell">
      <section class="masthead">
        <div class="masthead-copy">
          <p class="eyebrow">Cloudflare Tunnel Console</p>
          <h1>Laravel Herd sharing without terminal drift</h1>
          <p class="hero-copy">Manage named tunnel ingress, random share URLs, host-header routing, and local build commands from one Windows desktop panel.</p>
        </div>
        <div class="masthead-status tone-${statusTone(tunnelStatus)}">
          <span class="status-dot"></span>
          <div>
            <strong>${escapeHtml(tunnelStatus.running ? 'Tunnel running' : 'Tunnel stopped')}</strong>
            <p>${escapeHtml(tunnelStatus.mode || 'named')} mode • ${escapeHtml(appState.settings.tunnelName)}</p>
          </div>
        </div>
      </section>

      ${state.notice ? `<section class="notice notice-${state.notice.tone}">${escapeHtml(state.notice.message)}</section>` : ''}

      <section class="command-deck">
        <article class="command-bar">
          <div class="command-group">
            <span class="deck-label">Tunnel</span>
            <strong>${escapeHtml(appState.settings.tunnelName)}</strong>
            <span>${escapeHtml(tunnelStatus.tunnelId || 'UUID pending')}</span>
          </div>
          <div class="command-group">
            <span class="deck-label">Cloudflared</span>
            <strong>${escapeHtml(appState.cloudflaredPath || 'Not detected')}</strong>
            <span>${escapeHtml(tunnelStatus.pid ? `PID ${tunnelStatus.pid}` : 'Process idle')}</span>
          </div>
          <div class="command-group">
            <span class="deck-label">Active URL</span>
            <strong>${escapeHtml(activeUrl || 'No public route active')}</strong>
            <span>${escapeHtml(tunnelStatus.running ? 'Live session' : 'Waiting')}</span>
          </div>
          <div class="action-row compact">
            <button type="button" data-action="start-tunnel">Start Tunnel</button>
            <button type="button" class="secondary" data-action="stop-tunnel">Stop</button>
            <button type="button" class="secondary" data-action="create-tunnel">Create / Reuse</button>
            <button type="button" class="secondary" data-action="refresh">Refresh</button>
          </div>
        </article>
      </section>

      <section class="workspace">
        <aside class="panel sidebar-panel">
          <div class="panel-header sticky-header">
            <div>
              <p class="eyebrow">Project presets</p>
              <h2>Laravel Herd apps</h2>
            </div>
            <button type="button" data-action="new-project">Add Project</button>
          </div>
          <div class="sidebar-summary">
            <div class="summary-block">
              <span class="summary-label">Saved projects</span>
              <strong>${escapeHtml(String(appState.settings.projects.length))}</strong>
            </div>
            <div class="summary-block">
              <span class="summary-label">Default domain</span>
              <strong>${escapeHtml(appState.settings.defaultDomain)}</strong>
            </div>
          </div>
          <div class="project-list">${projectRows(appState)}</div>
        </aside>

        <section class="content-column">
          <article class="panel selected-panel">
            <div class="panel-header">
              <div>
                <p class="eyebrow">Selected project</p>
                <h2>${escapeHtml(project?.displayName || 'No project selected')}</h2>
              </div>
              <span class="pill pill-outline">${escapeHtml(project ? shareModeLabel(project.shareMode) : 'n/a')}</span>
            </div>

            ${
              project
                ? `
                  <div class="hero-project">
                    <div class="hero-project-main">
                      <strong>${escapeHtml(project.localHost)}</strong>
                      <p>${escapeHtml(projectUrl || 'No public URL assigned yet')}</p>
                    </div>
                    <div class="host-tags">
                      <span>${escapeHtml(project.subdomain || 'dynamic subdomain')}</span>
                      <span>${escapeHtml(project.projectPath)}</span>
                    </div>
                  </div>

                  <div class="metric-grid">
                    <div class="metric-card">
                      <span class="summary-label">Origin service</span>
                      <strong>${escapeHtml(appState.settings.defaultServiceURL)}</strong>
                      <p>Requests are forwarded here with the Herd host header override.</p>
                    </div>
                    <div class="metric-card">
                      <span class="summary-label">Host header</span>
                      <strong>${escapeHtml(project.localHost)}</strong>
                      <p>Applied via <code>originRequest.httpHostHeader</code>.</p>
                    </div>
                    <div class="metric-card">
                      <span class="summary-label">Connected routes</span>
                      <strong>${escapeHtml(String(tunnelStatus.activeHostnames.length))}</strong>
                      <p>${escapeHtml(tunnelStatus.activeHostnames.join(', ') || 'No ingress hostnames currently loaded.')}</p>
                    </div>
                  </div>

                  <div class="action-cluster">
                    <div class="cluster-block">
                      <span class="cluster-title">Sharing</span>
                      <div class="action-row">
                        <button type="button" data-action="share-project">Share Selected Project</button>
                        <button type="button" class="secondary" data-action="share-random">Random URL</button>
                        <button type="button" class="secondary" data-action="share-quick">Quick Tunnel</button>
                        <button type="button" class="secondary" data-action="open-url">Open URL</button>
                        <button type="button" class="secondary" data-action="copy-url">Copy URL</button>
                      </div>
                    </div>
                    <div class="cluster-block">
                      <span class="cluster-title">Project tools</span>
                      <div class="action-row">
                        <button type="button" class="secondary" data-action="npm-build">Run npm build</button>
                        <button type="button" class="secondary" data-action="test-project">Test local URL</button>
                        <button type="button" class="secondary" data-action="edit-project">Edit Project</button>
                        <button type="button" class="danger" data-action="delete-project">Delete Project</button>
                      </div>
                    </div>
                  </div>
                `
                : '<p class="empty-copy">Create or select a project to start sharing.</p>'
            }
          </article>

          <div class="sub-grid">
            <article class="panel">
              <div class="panel-header">
                <div>
                  <p class="eyebrow">Tunnel context</p>
                  <h2>Routes and runtime</h2>
                </div>
                <span class="pill ${tunnelStatus.running ? 'pill-success' : 'pill-muted'}">${escapeHtml(tunnelStatus.running ? 'running' : 'stopped')}</span>
              </div>
              <div class="status-grid">
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

            <article class="panel">
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
                <div class="action-row wide"><button type="submit">Save Settings</button></div>
              </form>
            </article>
          </div>

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
                  <label>Local host<input name="localHost" value="${escapeHtml(state.editorProject.localHost)}" placeholder="hr-system.test" /></label>
                  <label>
                    Project folder
                    <div class="folder-picker">
                      <input name="projectPath" value="${escapeHtml(state.editorProject.projectPath)}" placeholder="D:\\code\\hr-system" />
                      <button type="button" class="secondary browse-button" data-action="browse-project-folder">Browse</button>
                    </div>
                  </label>
                  <label>Stable subdomain<input name="subdomain" value="${escapeHtml(state.editorProject.subdomain)}" placeholder="app" /></label>
                  <label>
                    Share mode
                    <select name="shareMode">
                      <option value="stable" ${state.editorProject.shareMode === 'stable' ? 'selected' : ''}>Stable hostname</option>
                      <option value="random-domain" ${state.editorProject.shareMode === 'random-domain' ? 'selected' : ''}>Random under my domain</option>
                      <option value="quick" ${state.editorProject.shareMode === 'quick' ? 'selected' : ''}>Quick tunnel</option>
                    </select>
                  </label>
                  <div class="action-row wide"><button type="submit">${state.editorMode === 'create' ? 'Save Project' : 'Update Project'}</button></div>
                </form>
              </section>
            `
              : ''
          }

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
        </section>
      </section>

      <footer class="footer">
        <span>Settings file: ${escapeHtml(appState.settingsPath)}</span>
        <span>${escapeHtml(state.busy || 'Ready')}</span>
      </footer>
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

  state.editorProject = {
    id: formValue(projectForm, 'id'),
    displayName: formValue(projectForm, 'displayName'),
    localHost: formValue(projectForm, 'localHost'),
    projectPath: formValue(projectForm, 'projectPath'),
    subdomain: formValue(projectForm, 'subdomain'),
    publicURL: state.editorProject.publicURL,
    shareMode: (formValue(projectForm, 'shareMode') as ShareMode) || 'stable',
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
    if (next) {
      state.appState = next;
      state.selectedProjectId = payload.id || (next.settings.projects[next.settings.projects.length - 1]?.id ?? null);
      state.editorOpen = false;
      setNotice('success', 'Project preset saved');
    }
  });
}

async function handleAction(action: string, id: string | null) {
  switch (action) {
    case 'select-project':
      state.selectedProjectId = id ?? null;
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
        state.editorProject.projectPath = result;
        render();
      }
      return;
    }
    case 'copy-url': {
      const project = selectedProject();
      if (!project || !state.appState) return;
      const url = formatProjectURL(project, state.appState.settings.defaultDomain);
      if (!url) {
        setNotice('error', 'No public URL is available for the selected project');
        return;
      }
      await navigator.clipboard.writeText(url);
      setNotice('success', `Copied ${url}`);
      return;
    }
  }

  if (!id && ['share-project', 'share-random', 'share-quick', 'open-url', 'npm-build', 'test-project', 'delete-project'].includes(action)) {
    setNotice('error', 'Select a project first');
    return;
  }

  switch (action) {
    case 'start-tunnel': {
      const next = await withAction('Starting tunnel...', () => api.startTunnel());
      if (next) {
        state.appState = next;
        setNotice('success', 'Named tunnel started');
      }
      return;
    }
    case 'stop-tunnel': {
      const next = await withAction('Stopping tunnel...', () => api.stopTunnel());
      if (next) {
        state.appState = next;
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
      if (next) state.appState = next;
      return;
    }
    case 'share-project': {
      const next = await withAction('Sharing project...', () => api.shareProject(id!));
      if (next) {
        state.appState = next;
        setNotice('success', 'Project shared through named tunnel');
      }
      return;
    }
    case 'share-random': {
      const next = await withAction('Generating random hostname...', () => api.shareProjectWithRandomURL(id!));
      if (next) {
        state.appState = next;
        setNotice('success', 'Random domain share is active');
      }
      return;
    }
    case 'share-quick': {
      const next = await withAction('Starting quick tunnel...', () => api.startQuickTunnel(id!));
      if (next) {
        state.appState = next;
        setNotice('success', 'Quick tunnel started');
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
          setNotice('success', 'Project deleted');
        }
      }
      return;
  }
}

root.addEventListener('click', (event) => {
  const target = event.target as HTMLElement | null;
  const actionElement = target?.closest<HTMLElement>('[data-action]');
  if (!actionElement) return;

  event.preventDefault();
  const action = actionElement.dataset.action;
  const id = actionElement.dataset.id ?? selectedProject()?.id ?? null;
  if (!action) return;
  void handleAction(action, id);
});

async function bootstrap() {
  const next = await withAction('Loading app state...', () => api.bootstrap());
  if (!next) return;
  state.appState = next;
  state.selectedProjectId = next.settings.projects[0]?.id ?? null;
  render();

  window.runtime?.EventsOn('log', (payload) => {
    if (!state.appState) return;
    const entry = payload as LogEntry;
    state.appState.status.lastLogs = [...state.appState.status.lastLogs, entry].slice(-300);
    render();
  });

  window.runtime?.EventsOn('status', (payload) => {
    if (!state.appState) return;
    state.appState.status = payload as TunnelStatus;
    render();
  });
}

bootstrap();
