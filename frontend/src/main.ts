import './style.css';
import { api } from './api';
import { t, setLang, getLang } from './i18n';
import { WindowCenter, WindowIsMaximised, WindowSetMinSize, WindowSetSize, WindowUnmaximise } from '../wailsjs/runtime/runtime';
import type { AppState, LogEntry, PHPConfigInfo, ProjectPreset, ShareMode, StackServiceStatus, TerminalExitEvent, TerminalLineEvent, TerminalStatus, TunnelStatus } from './types';

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
  sidebarCollapsed: boolean;
  projectMenuOpen: boolean;
  projectPickerOpen: boolean;
  stackMenuOpen: boolean;
  stackTab: string;
  overviewTab: 'project' | 'terminal';
  editorOpen: boolean;
  editorMode: 'create' | 'edit';
  editorProject: ProjectPreset;
  notice: Notice | null;
  busy: string | null;
  licenseDraft: string;
  projectSearch: string;
  stackStatuses: StackServiceStatus[];
  phpConfig: PHPConfigInfo | null;
  phpVersions: { installable: string[]; installed: { version: string; dir: string; legacy: boolean }[]; active: string } | null;
  stackDraft: {
    nginxBinaryPath: string;
    phpCgiBinaryPath: string;
    mysqldBinaryPath: string;
    nginxPort: number;
    phpPort: number;
    mysqlPort: number;
  };
  terminalProjectId: string | null;
  terminalCommand: string;
  terminalRunning: boolean;
  terminalStarting: boolean;
  terminalSession: number;
  terminalExitedSession: number;
  terminalLines: { stream: string; line: string }[];
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

const SIDEBAR_COLLAPSED_KEY = 'exposely.sidebarCollapsed';

function readSidebarCollapsed(): boolean {
  try {
    return window.localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === '1';
  } catch {
    return false;
  }
}

function rememberSidebarCollapsed(collapsed: boolean) {
  try {
    window.localStorage.setItem(SIDEBAR_COLLAPSED_KEY, collapsed ? '1' : '0');
  } catch {
    // WebView storage can be unavailable; the toggle still works per session.
  }
}

const state: UIState = {
  appState: null,
  fatalError: null,
  selectedProjectId: null,
  activeProjectId: null,
  projectUrls: {},
  copiedProjectId: null,
  activeTab: 'overview',
  sidebarCollapsed: readSidebarCollapsed(),
  projectMenuOpen: false,
  projectPickerOpen: false,
  stackMenuOpen: false,
  stackTab: 'nginx',
  overviewTab: 'project',
  editorOpen: false,
  editorMode: 'create',
  editorProject: blankProject(),
  notice: null,
  busy: null,
  licenseDraft: '',
  projectSearch: '',
  stackStatuses: [],
  phpConfig: null,
  phpVersions: null,
  stackDraft: blankStackDraft(),
  terminalProjectId: null,
  terminalCommand: 'npm run dev',
  terminalRunning: false,
  terminalStarting: false,
  terminalSession: 0,
  terminalExitedSession: 0,
  terminalLines: [],
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

function languageCode(lang: string): string {
  return lang === 'km' ? 'KH' : 'EN';
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

function hasPersistedNamedURL(project: ProjectPreset): boolean {
  const url = (project.publicURL || '').trim();
  if (!url.toLowerCase().startsWith('https://')) return false;
  const domain = (state.appState?.settings.defaultDomain || '').trim().toLowerCase();
  if (!isMeaningfulDomain(domain)) return false;
  try {
    return new URL(url).hostname.toLowerCase().endsWith('.' + domain);
  } catch {
    return false;
  }
}

function usesEphemeralURL(project: ProjectPreset): boolean {
  // trycloudflare quick tunnels and HTML shares get a new address each
  // start. Fixed / random-domain named-tunnel URLs are persisted —
  // including a fixed share started from a quick/auto project, whose URL
  // lives on the app's own domain and survives restarts.
  if (project.shareMode === 'host-html') return true;
  if (project.shareMode !== 'auto' && project.shareMode !== 'quick') return false;
  return !hasPersistedNamedURL(project);
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
  const domain = state.appState?.settings.defaultDomain ?? '';
  const canNamed = isMeaningfulDomain(domain);
  switch (project.shareMode) {
    case 'auto':
      return { action: 'share-quick', label: t('startAutoShare') };
    case 'stable':
      return canNamed
        ? { action: 'share-fixed', label: t('startFixedUrl') }
        : { action: 'share-quick', label: t('createPublicUrl') };
    case 'random-domain':
      return canNamed
        ? { action: 'share-random', label: t('startRandomUrl') }
        : { action: 'share-quick', label: t('createPublicUrl') };
    case 'host-html':
      return { action: 'share-quick', label: t('createHtmlUrl') };
    case 'quick':
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
    case 'stable':
      return t('fixedPublicUrl');
    case 'random-domain':
      return t('randomPublicUrl');
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

const SELECTED_PROJECT_KEY = 'exposely.selectedProjectId';

function rememberSelectedProject(id: string | null) {
  try {
    if (id) {
      window.localStorage.setItem(SELECTED_PROJECT_KEY, id);
    } else {
      window.localStorage.removeItem(SELECTED_PROJECT_KEY);
    }
  } catch {
    // WebView storage can be unavailable; remembering is a convenience.
  }
}

function rememberedProjectId(projects: ProjectPreset[]): string | null {
  try {
    const stored = window.localStorage.getItem(SELECTED_PROJECT_KEY);
    if (stored && projects.some((project) => project.id === stored)) return stored;
  } catch {
    // Ignore and fall back to the first project.
  }
  return null;
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

// A running terminal stays attached to the project it was started for;
// an idle terminal belongs to the currently selected project. This keeps
// the panel under each project honest when the selection changes mid-run.
function terminalOwnerProject(): ProjectPreset | null {
  const selected = selectedProject();
  if (!state.terminalRunning) return selected;
  const projects = state.appState?.settings.projects ?? [];
  return projects.find((entry) => entry.id === state.terminalProjectId) ?? selected;
}

function terminalOutputHtml(): string {
  if (state.terminalLines.length === 0) {
    return `<span class="terminal-line terminal-empty">${escapeHtml(t('terminalEmpty'))}</span>`;
  }
  // Each line is a block element, so no newline text nodes between
  // spans — those would render as blank lines and double the spacing.
  return state.terminalLines
    .map((entry) => `<span class="terminal-line terminal-${escapeHtml(entry.stream || 'stdout')}">${escapeHtml(entry.line) || '&nbsp;'}</span>`)
    .join('');
}

// The terminal lives under the selected project instead of being its own
// sidebar page: one project at a time, with the command form and output
// directly below the project details.
function renderTerminalSection(project: ProjectPreset): string {
  const owner = terminalOwnerProject();
  const runningElsewhere = state.terminalRunning && !!owner && owner.id !== project.id;
  const folderPath = !looksLikeURL(project.projectPath.trim()) ? project.projectPath.trim() : '';

  if (runningElsewhere && owner) {
    return `
      <section class="panel terminal-panel terminal-in-project">
        <div class="terminal-section-head">
          <p class="eyebrow">${t('terminal')}</p>
        </div>
        <p class="hint">${escapeHtml(t('terminalRunningElsewhere').replace('{name}', owner.displayName))}</p>
        <div class="action-row">
          <button type="button" class="danger-button" data-action="terminal-stop">${t('stop')}</button>
        </div>
      </section>
    `;
  }

  return `
    <section class="panel terminal-panel terminal-in-project">
      ${folderPath ? `<div class="terminal-section-head">
        <span class="terminal-cwd" title="${escapeHtml(folderPath)}">${escapeHtml(folderPath)}</span>
      </div>` : ''}
      <form id="terminal-command-form" class="terminal-command-row">
        <label class="wide">${t('terminalCommand')}
          <input name="terminalCommand" value="${escapeHtml(state.terminalCommand)}" placeholder="npm run dev" spellcheck="false" autocomplete="off" />
        </label>
        <button type="submit" ${state.terminalRunning ? 'disabled' : ''}>${t('terminalRun')}</button>
        <button type="button" class="danger-button" data-action="terminal-stop" ${state.terminalRunning ? '' : 'disabled'}>${t('stop')}</button>
        <button type="button" class="secondary" data-action="terminal-clear">${t('terminalClear')}</button>
      </form>
      <pre id="terminal-output" class="terminal-output">${terminalOutputHtml()}</pre>
      <form id="terminal-input-form" class="terminal-input-row">
        <input name="terminalInput" placeholder="${escapeHtml(t('terminalInputPlaceholder'))}" autocomplete="off" ${state.terminalRunning ? '' : 'disabled'} />
        <button type="submit" class="secondary" ${state.terminalRunning ? '' : 'disabled'}>${t('terminalSend')}</button>
      </form>
      ${folderPath ? '' : `<p class="hint">${escapeHtml(t('terminalNoFolder'))}</p>`}
    </section>
  `;
}

function appendTerminalLine(stream: string, line: string) {
  state.terminalLines.push({ stream, line });
  if (state.terminalLines.length > 2000) {
    state.terminalLines.splice(0, state.terminalLines.length - 2000);
  }
  const output = root.querySelector<HTMLElement>('#terminal-output');
  if (!output) return;

  const empty = output.querySelector('.terminal-empty');
  if (empty) empty.remove();

  const row = document.createElement('span');
  row.className = `terminal-line terminal-${stream || 'stdout'}`;
  row.textContent = line || ' ';
  output.appendChild(row);
  output.scrollTop = output.scrollHeight;
}

async function runTerminalCommand() {
  // The terminal always runs for the project currently shown in the
  // overview — no separate terminal project selection any more.
  const project = selectedProject();
  if (!project) {
    setNotice('error', t('selectProjectFirst'));
    return;
  }
  const input = root.querySelector<HTMLInputElement>('#terminal-command-form input[name=terminalCommand]');
  const command = (input?.value ?? state.terminalCommand).trim();
  if (!command) return;

  state.terminalCommand = command;
  state.terminalProjectId = project.id;
  state.terminalLines = [];
  state.terminalStarting = true;
  const status = await withAction(t('terminalRun'), () => api.terminalStart(project.id, command));
  state.terminalStarting = false;
  if (!status) {
    render();
    return;
  }
  state.terminalRunning = !!status.running && status.session !== state.terminalExitedSession;
  state.terminalSession = status.session;
  render();
}

function projectRowSearchText(project: ProjectPreset): string {
  return [
    project.displayName,
    projectTypeLabel(project),
    project.localHost,
    project.localURL,
    project.projectPath,
    project.publicURL,
    project.subdomain,
  ]
    .join(' ')
    .toLowerCase();
}

function projectRows(appState: AppState): string {
  const projects = appState.settings.projects;

  if (projects.length === 0) {
    return `<div class="empty-list">${escapeHtml(t('noProject'))}</div>`;
  }

  const rows = projects
    .map((project) => {
      const isSelected = selectedProject()?.id === project.id;
      const showRunning = state.activeProjectId === project.id && appState.status.running;
      return `
        <button type="button" class="project-row ${isSelected ? 'selected' : ''}" data-action="select-project" data-id="${escapeHtml(project.id)}" data-search="${escapeHtml(projectRowSearchText(project))}">
          <div class="project-copy">
            <strong>${escapeHtml(project.displayName)}</strong>
            <span class="project-type-tag">${escapeHtml(projectTypeLabel(project))}</span>
          </div>
          ${showRunning ? `<span class="project-running-badge">${t('running')}</span>` : ''}
        </button>
      `;
    })
    .join('');

  return `${rows}<div class="empty-list picker-empty" hidden>${escapeHtml(t('noMatchingProjects'))}</div>`;
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

// captureProjectsScroll/restoreProjectsScroll do the same for the project
// picker's list (.picker-list), which scrolls on its own inside the modal.
// Without this, a tunnel status/log re-render snaps the list back to the top
// while the user is scrolling through a long project list.
function captureProjectsScroll(): number {
  const list = root.querySelector<HTMLElement>('.picker-list');
  return list ? list.scrollTop : 0;
}

function restoreProjectsScroll(scrollTop: number) {
  const list = root.querySelector<HTMLElement>('.picker-list');
  if (!list) return;
  const max = list.scrollHeight - list.clientHeight;
  list.scrollTop = Math.max(0, Math.min(scrollTop, max));
}

// Tunnel status / log events fire every few seconds while cloudflared is
// running. A full re-render rebuilds the DOM from saved settings and
// would wipe anything the user has typed but not yet saved. Capture
// live form field values before innerHTML is replaced and put them back
// after, so Settings can be edited while a tunnel is live.
type FormSnapshot = Record<string, string>;

interface FocusSnapshot {
  formKey: string;
  name: string;
  selectionStart: number | null;
  selectionEnd: number | null;
}

// form.id is shadowed by any control named "id" (HTMLFormElement named
// getters win over the IDL attribute), and the project form has
// <input name="id">. Read the attribute so its snapshot keys stay stable
// and match the "project-form::" refresh filter.
function formSnapshotKey(form: HTMLFormElement): string {
  return form.getAttribute('id') || form.className;
}

function captureFormSnapshot(): { fields: FormSnapshot; focus: FocusSnapshot | null } {
  const fields: FormSnapshot = {};
  let focus: FocusSnapshot | null = null;
  for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
    for (const el of Array.from(form.elements)) {
      const input = el as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement;
      const name = input.name;
      if (!name) continue;
      const key = `${formSnapshotKey(form)}::${name}`;
      if (input instanceof HTMLInputElement && (input.type === 'checkbox' || input.type === 'radio')) {
        if (input.type === 'radio' && !input.checked) continue;
        fields[key] = input.checked ? '1' : '';
        continue;
      }
      fields[key] = input.value;
      if (document.activeElement === input) {
        const start = 'selectionStart' in input ? input.selectionStart : null;
        const end = 'selectionEnd' in input ? input.selectionEnd : null;
        focus = {
          formKey: formSnapshotKey(form),
          name,
          selectionStart: typeof start === 'number' ? start : null,
          selectionEnd: typeof end === 'number' ? end : null,
        };
      }
    }
  }
  return { fields, focus };
}

function restoreFormSnapshot(snap: { fields: FormSnapshot; focus: FocusSnapshot | null }) {
  if (!snap?.fields || Object.keys(snap.fields).length === 0) return;
  let focused: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement | null = null;
  for (const form of root.querySelectorAll<HTMLFormElement>('form')) {
    for (const el of Array.from(form.elements)) {
      const input = el as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement;
      const name = input.name;
      if (!name) continue;
      const key = `${formSnapshotKey(form)}::${name}`;
      if (!(key in snap.fields)) continue;
      const value = snap.fields[key];
      if (input instanceof HTMLInputElement && (input.type === 'checkbox' || input.type === 'radio')) {
        input.checked = value === '1';
        continue;
      }
      input.value = value;
      if (snap.focus && formSnapshotKey(form) === snap.focus.formKey && name === snap.focus.name) {
        focused = input;
      }
    }
  }
  const focusInfo = snap.focus;
  if (focused && focusInfo) {
    focused.focus();
    const input = focused as HTMLInputElement | HTMLTextAreaElement;
    if (focusInfo.selectionStart != null && 'setSelectionRange' in input) {
      try {
        input.setSelectionRange(focusInfo.selectionStart, focusInfo.selectionEnd ?? focusInfo.selectionStart);
      } catch {
        // Some input types (number) reject setSelectionRange — ignore.
      }
    }
  }
}

// render() optionally accepts { refreshProjectForm: true } for renders that
// follow a deliberate state change of the project editor (browsed folder,
// random subdomain, new/edit project). The DOM snapshot taken at the top of
// render() still holds the pre-change values, so restoring it wholesale
// would show the old path again — and Save reads the DOM, silently losing
// the change. Those renders keep every other form's drafts but drop the
// project-form keys so the form shows the new state values.
function navIcon(tab: 'overview' | 'terminal' | 'logs' | 'settings' | 'about'): string {
  const attrs = 'width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"';
  if (tab === 'overview') {
    return `<svg ${attrs}><rect x="3" y="3" width="7.5" height="7.5" rx="1.6"/><rect x="13.5" y="3" width="7.5" height="7.5" rx="1.6"/><rect x="3" y="13.5" width="7.5" height="7.5" rx="1.6"/><rect x="13.5" y="13.5" width="7.5" height="7.5" rx="1.6"/></svg>`;
  }
  if (tab === 'terminal') {
    return `<svg ${attrs}><polyline points="5 17 10 11 5 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>`;
  }
  if (tab === 'logs') {
    return `<svg ${attrs}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="8" y1="13" x2="16" y2="13"/><line x1="8" y1="17" x2="16" y2="17"/></svg>`;
  }
  if (tab === 'settings') {
    return `<svg ${attrs}><line x1="4" y1="21" x2="4" y2="14"/><line x1="4" y1="10" x2="4" y2="3"/><line x1="12" y1="21" x2="12" y2="12"/><line x1="12" y1="8" x2="12" y2="3"/><line x1="20" y1="21" x2="20" y2="16"/><line x1="20" y1="12" x2="20" y2="3"/><line x1="1" y1="14" x2="7" y2="14"/><line x1="9" y1="8" x2="15" y2="8"/><line x1="17" y1="16" x2="23" y2="16"/></svg>`;
  }
  return `<svg ${attrs}><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>`;
}

function navItemMarkup(
  tab: 'overview' | 'terminal' | 'logs' | 'settings' | 'about',
  label: string,
  hint: string,
  count?: number,
): string {
  return `
    <button type="button" class="nav-item ${state.activeTab === tab ? 'active' : ''}" data-action="tab-${tab}" title="${escapeHtml(hint)}">
      ${navIcon(tab)}
      <span class="nav-item-label">${escapeHtml(label)}</span>
      ${typeof count === 'number' ? `<span class="nav-count">${count}</span>` : ''}
    </button>
  `;
}

function pageHeadMarkup(opts: { eyebrow: string; title: string; desc?: string; chips?: string; actions?: string }): string {
  const side = opts.chips || opts.actions
    ? `
      <div class="page-head-side">
        ${opts.chips ? `<div class="page-head-chips">${opts.chips}</div>` : ''}
        ${opts.actions ? `<div class="page-head-actions">${opts.actions}</div>` : ''}
      </div>
    `
    : '';
  return `
    <header class="page-head">
      <div class="page-head-main">
        <span class="eyebrow">${escapeHtml(opts.eyebrow)}</span>
        <h1>${escapeHtml(opts.title)}</h1>
        ${opts.desc ? `<p class="page-head-desc">${escapeHtml(opts.desc)}</p>` : ''}
      </div>
      ${side}
    </header>
  `;
}

function render(options: { refreshProjectForm?: boolean } = {}) {
  const formSnap = captureFormSnapshot();

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
  const projectFolderPath = project && !looksLikeURL(project.projectPath.trim())
    ? project.projectPath.trim()
    : '';
  const hasProjects = appState.settings.projects.length > 0;
  const shareToolReady = appState.cloudflaredDetected;
  const hasDomain = isMeaningfulDomain(appState.settings.defaultDomain);
  const stackConfigured = !!(appState.settings.stack && (appState.settings.stack.nginxBinaryPath || appState.settings.stack.phpCgiBinaryPath || appState.settings.stack.mysqldBinaryPath));
  const stackRunningCount = state.stackStatuses.filter((s) => s.running).length;
  const stackTotalCount = state.stackStatuses.length || 3;
  const canSetupTunnel = shareToolReady;
  const shareAction = project ? shareActionForProject(project) : null;
  const canShareSelectedProject = Boolean(
    project
      && shareAction
      && shareToolReady
      && (project.shareMode === 'auto' ||
          project.shareMode === 'quick' ||
          project.shareMode === 'host-html' ||
          project.shareMode === 'stable' ||
          project.shareMode === 'random-domain'),
  );
  const canStartTunnel = project ? canShareSelectedProject : shareToolReady && hasProjects;
  const canRunProjectBuild = Boolean(project?.projectPath.trim()) && !looksLikeURL(project?.projectPath ?? '') && appState.buildCommandDetected && !appState.buildRunning;
  const canTestProject = Boolean(project?.localHost.trim()) && project?.shareMode !== 'host-html';
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
  const busyChip = state.busy ? `<span class="busy-indicator">${escapeHtml(state.busy)}</span>` : '';

  // Shared chunks used by more than one view.
  const bannersHtml = `
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
  `;

  const editorPanelHtml = state.editorOpen
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
              ${hasDomain ? `<option value="stable" ${state.editorProject.shareMode === 'stable' ? 'selected' : ''}>${t('fixedPublicUrlOption')}</option>
              <option value="random-domain" ${state.editorProject.shareMode === 'random-domain' ? 'selected' : ''}>${t('randomPublicUrlOption')}</option>` : ''}
              <option value="host-html" ${state.editorProject.shareMode === 'host-html' ? 'selected' : ''}>${t('htmlProjectOption')}</option>
            </select>
          </label>
          ${
            hasDomain && (state.editorProject.shareMode === 'stable' || state.editorProject.shareMode === 'random-domain')
              ? `
              <label>${t('urlType')}
                <select name="urlType" disabled>
                  <option>${state.editorProject.shareMode === 'stable' ? t('fixedPublicUrl') : t('randomPublicUrl')}</option>
                </select>
              </label>
              ${
                state.editorProject.shareMode === 'stable'
                  ? `<label>${t('subdomain')}<input name="subdomain" value="${escapeHtml(state.editorProject.subdomain)}" placeholder="${t('subdomainPlaceholder')}" required /></label>`
                  : `<label>${t('subdomain')}<input name="subdomain" value="${escapeHtml(state.editorProject.subdomain)}" placeholder="${t('subdomainPlaceholder')}" /></label>`
              }
              <p class="hint wide">${escapeHtml(state.editorProject.shareMode === 'stable' ? t('fixedPublicUrlHint') : t('randomPublicUrlHint'))} ${t('defaultDomainHint')}</p>
              <p class="hint wide">${t('localHostTestHint')}</p>
              `
              : ''
          }
          ${
            state.editorProject.shareMode === 'quick' ||
            state.editorProject.shareMode === 'auto' ||
            state.editorProject.shareMode === 'stable' ||
            state.editorProject.shareMode === 'random-domain'
              ? `<label>${state.editorProject.shareMode === 'auto' ? t('localHostOptional') : t('localHostRequired')}<input name="localHost" value="${escapeHtml(state.editorProject.localHost)}" placeholder="app.test" /></label>`
              : ''
          }
          ${
            state.editorProject.shareMode === 'auto' || state.editorProject.shareMode === 'host-html'
              ? `<label>${t('localUrl')}<input name="localURL" value="${escapeHtml(state.editorProject.localURL)}" placeholder="http://127.0.0.1:5500" /></label>`
              : ''
          }
          ${
            state.editorProject.shareMode === 'quick' ||
            state.editorProject.shareMode === 'auto' ||
            state.editorProject.shareMode === 'stable' ||
            state.editorProject.shareMode === 'random-domain'
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
    : '';

  // The project list lives in a modal instead of a permanent column, so the
  // selected project's details get the full width. Rendered at the end of the
  // shell with position:fixed so it is never clipped by a scroll container.
  const projectPickerHtml = state.projectPickerOpen
    ? `
      <div class="modal-layer">
        <div class="modal-backdrop" data-action="close-project-picker"></div>
        <div class="modal project-picker" role="dialog" aria-modal="true" aria-label="${escapeHtml(t('projects'))}">
          <div class="modal-head">
            <h2>${t('projects')}<span class="sidebar-count">${appState.settings.projects.length}</span></h2>
            <button type="button" class="secondary icon-only" data-action="close-project-picker" aria-label="${escapeHtml(t('close'))}" title="${escapeHtml(t('close'))}">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
            </button>
          </div>
          <input id="project-search" class="picker-search" type="search" value="${escapeHtml(state.projectSearch)}" placeholder="${t('searchProjects')}" aria-label="${t('searchProjects')}" autocomplete="off" />
          <div class="project-list picker-list">${projectRows(appState)}</div>
          <div class="modal-foot">
            <button type="button" class="secondary" data-action="new-project">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg>
              ${t('newProject')}
            </button>
          </div>
        </div>
      </div>
    `
    : '';

  const overviewActions = `
    <button type="button" class="secondary picker-trigger" data-action="open-project-picker" aria-haspopup="dialog" title="${escapeHtml(project ? project.displayName : t('projects'))}">
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg>
      ${t('projects')}
      <svg class="terminal-switch-caret" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="6 9 12 15 18 9"></polyline></svg>
    </button>
    ${
      appState.status.running
        ? `<button type="button" class="secondary" data-action="stop-tunnel">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="6" y="6" width="12" height="12"></rect></svg>
            ${t('stop')}
          </button>`
        : `<button type="button" data-action="start-tunnel" ${!canStartTunnel ? 'disabled' : ''}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polygon points="5 3 19 12 5 21 5 3"></polygon></svg>
            ${escapeHtml(shareAction?.label || t('startTunnel'))}
          </button>`
    }
    <button type="button" class="secondary icon-only" data-action="refresh" title="${t('refreshState')}" aria-label="${t('refreshState')}">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="23 4 23 10 17 10"></polyline><polyline points="1 20 1 14 7 14"></polyline><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
    </button>
    ${
      project
        ? `
          <div class="dropdown">
            <button type="button" class="secondary icon-only" data-action="toggle-project-menu" aria-haspopup="true" aria-expanded="${state.projectMenuOpen ? 'true' : 'false'}" aria-label="${escapeHtml(t('manage'))}" title="${escapeHtml(t('manage'))}">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><circle cx="5" cy="12" r="1.7"></circle><circle cx="12" cy="12" r="1.7"></circle><circle cx="19" cy="12" r="1.7"></circle></svg>
            </button>
            ${
              state.projectMenuOpen
                ? `
                  <div class="dropdown-menu">
                    <button type="button" class="dropdown-item" data-action="open-project-folder" ${projectFolderPath ? '' : 'disabled'}>
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
                      </svg>
                      <span>${t('openFolder')}</span>
                    </button>
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
  `;

  const overviewBody = project
    ? `
      <article class="panel selected-panel">
        <div class="overview-tabs" role="tablist" aria-label="${escapeHtml(t('projectView'))}">
          <button type="button" role="tab" aria-selected="${state.overviewTab === 'project' ? 'true' : 'false'}"
            class="overview-tab ${state.overviewTab === 'project' ? 'active' : ''}" data-action="overview-tab" data-id="project">
            ${t('projectInfo')}
          </button>
          <button type="button" role="tab" aria-selected="${state.overviewTab === 'terminal' ? 'true' : 'false'}"
            class="overview-tab ${state.overviewTab === 'terminal' ? 'active' : ''}" data-action="overview-tab" data-id="terminal">
            ${t('terminal')}
            ${state.terminalRunning ? '<span class="status-dot tone-live"></span>' : ''}
          </button>
        </div>
        ${
          state.overviewTab === 'terminal'
            ? renderTerminalSection(project)
            : `
        <div class="hero-project">
          <div class="hero-project-main">
            <div class="hero-target">
              <span class="summary-label">${t('publicUrl')}</span>
              <strong>${escapeHtml(projectPrimaryTarget(project))}</strong>
            </div>
            ${projectFolderPath ? `<div class="hero-project-path-row">
              <p class="hero-project-path" title="${escapeHtml(projectFolderPath)}">${escapeHtml(projectFolderPath)}</p>
              <button type="button" class="secondary inline-copy-button" data-action="open-project-folder" data-id="${escapeHtml(project.id)}" aria-label="${t('openFolder')}">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg>
                ${t('openFolder')}
              </button>
            </div>` : ''}
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
            ${
              hasDomain
                ? `<div class="action-row" style="margin-top: 12px; flex-wrap: wrap; gap: 8px;">
                    <button type="button" class="secondary" data-action="share-fixed" data-id="${escapeHtml(project.id)}" ${!shareToolReady ? 'disabled' : ''} title="${escapeHtml(t('fixedPublicUrlHint'))}">
                      ${t('startFixedUrl')}
                    </button>
                    <button type="button" class="secondary" data-action="share-random" data-id="${escapeHtml(project.id)}" ${!shareToolReady ? 'disabled' : ''} title="${escapeHtml(t('randomPublicUrlHint'))}">
                      ${t('startRandomUrl')}
                    </button>
                  </div>`
                : ''
            }
          </div>
        </div>

        ${renderUsagePanel(tunnelStatus)}
        `}
      </article>
    `
    : `
      <article class="panel empty-state-panel">
        <p class="empty-copy">${t('createOrSelect')}</p>
        <button type="button" data-action="new-project">${t('newProject')}</button>
      </article>
    `;

  const viewHead = state.activeTab === 'overview'
    ? pageHeadMarkup({
        eyebrow: t('projectsEyebrow'),
        title: project?.displayName || t('projects'),
        desc: t('projectsDesc'),
        chips: busyChip,
        actions: overviewActions,
      })
    : state.activeTab === 'logs'
        ? pageHeadMarkup({
            eyebrow: t('workspaceSection'),
            title: t('logs'),
            desc: t('navLogsHint'),
            chips: `<span class="chip chip-muted">${escapeHtml(`${appState.status.lastLogs.length} ${t('entries')}`)}</span>${busyChip}`,
          })
        : state.activeTab === 'settings'
          ? pageHeadMarkup({
              eyebrow: t('systemSection'),
              title: t('settings'),
              desc: t('navSettingsHint'),
              chips: busyChip,
            })
          : pageHeadMarkup({
              eyebrow: t('systemSection'),
              title: t('about'),
              desc: t('navAboutHint'),
              chips: busyChip,
            });

  const viewBody = state.activeTab === 'overview'
    ? overviewBody
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
              <button type="button" class="secondary" data-action="stack-detect" title="${t('stackDetectTitle')}">${t('stackDetect')}</button>
              <button type="button" class="secondary" data-action="nginx-install" title="${t('nginxInstall')}">${t('nginxInstall')}</button>
              <button type="button" class="secondary" data-action="mariadb-install" title="${t('mariaDBInstall')}">${t('mariaDBInstall')}</button>
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
          <div class="stack-status">
            ${stackStatusTabs()}
          </div>
          <div class="panel-header" style="margin-top: 8px;">
            <div>
              <p class="eyebrow">${t('phpSectionTitle')}</p>
              <h2>${t('phpSectionSubtitle')}</h2>
            </div>
            <div class="action-row">
              <select name="phpInstallVersion" class="php-version-select" title="${t('phpVersionPickTitle')}">
                ${(state.phpVersions?.installable ?? []).map((v) => `<option value="${escapeHtml(v)}" ${v === (state.phpConfig?.managedVersion || '') ? 'selected' : ''}>PHP ${escapeHtml(v)}</option>`).join('')}
              </select>
              <button type="button" class="secondary" data-action="php-install-version" title="${t('phpVersionInstallTitle')}">${t('phpVersionInstall')}</button>
              <button type="button" class="secondary" data-action="php-install">${state.phpConfig?.installed ? t('phpReinstall') : t('phpInstall')}</button>
            </div>
          </div>
          ${phpVersionsList()}
          <p class="hint" style="margin-top: 4px;">${state.phpConfig?.installed ? `${t('phpInstalledAt')}: ${escapeHtml(state.phpConfig.installDir)} (${escapeHtml(state.phpConfig.version || '?')})` : t('phpNotInstalledHint')}</p>
          ${state.phpConfig?.installed ? `
          <form id="php-config-form" class="form-grid">
            <label>${t('phpMemoryLimit')}<input name="memoryLimit" value="${escapeHtml(state.phpConfig.memoryLimit || '256M')}" placeholder="256M" /></label>
            <label>${t('phpUploadMax')}<input name="uploadMax" value="${escapeHtml(state.phpConfig.uploadMax || '64M')}" placeholder="64M" /></label>
            <label>${t('phpPostMax')}<input name="postMax" value="${escapeHtml(state.phpConfig.postMax || '64M')}" placeholder="64M" /></label>
            <label>${t('phpMaxExecTime')}<input type="number" min="1" max="600" name="maxExecTime" value="${escapeHtml(String(state.phpConfig.maxExecTime || 120))}" /></label>
            <label class="wide">${t('phpExtraExtensions')}<input name="extraExts" value="${escapeHtml((state.phpConfig.extraExts || []).join(', '))}" placeholder="soap, xsl, exif" /></label>
            <div class="action-row wide"><button type="submit">${t('phpSaveConfig')}</button></div>
          </form>
          <p class="hint">${t('phpIniPath')}: ${escapeHtml(state.phpConfig.iniPath)}</p>
          ` : ''}
          <p class="hint">${t('stackHint')}</p>
        </article>
      `
      : state.activeTab === 'logs'
        ? `
          <section class="panel logs-panel">
            <div class="log-stream">${logRows(appState.status.lastLogs)}</div>
          </section>
        `
        : `
            <article class="panel compact-panel">
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
              <div style="color: var(--text-secondary); line-height: 1.6;">
                <p>${t('aboutDescription')}</p>
                <p style="margin-top: 16px;">${t('aboutCreator')}</p>
                <p style="margin-top: 16px; color: var(--text-primary); font-weight: 500;">${t('aboutLicense')}</p>
              </div>
            </article>
          `;

  // Capture scroll position before we blow away the DOM. We only restore
  // when the tab did not change — explicit tab switches should land at the
  // top of the new tab so the user can see the page header.
  const previousScroll = captureMainScroll();
  const previousProjectsScroll = captureProjectsScroll();
  const tabChanged = lastRenderedTab !== null && lastRenderedTab !== state.activeTab;
  lastRenderedTab = state.activeTab;

  root.innerHTML = `
    <div class="app ${state.sidebarCollapsed ? 'sidebar-collapsed' : ''}">
      <header class="topbar">
        <div class="brand">
          <button type="button" class="brand-mark" data-action="toggle-sidebar"
            aria-expanded="${state.sidebarCollapsed ? 'false' : 'true'}"
            aria-label="${escapeHtml(state.sidebarCollapsed ? t('showSidebar') : t('hideSidebar'))}"
            title="${escapeHtml(state.sidebarCollapsed ? t('showSidebar') : t('hideSidebar'))}">
            <svg class="brand-logo" viewBox="0 0 32 32" fill="none" aria-hidden="true">
              <defs>
                <linearGradient id="exposelyLogoGrad" x1="6" y1="5" x2="26" y2="27" gradientUnits="userSpaceOnUse">
                  <stop class="logo-stop-a" offset="0"></stop>
                  <stop class="logo-stop-b" offset="1"></stop>
                </linearGradient>
              </defs>
              <path d="M6 27V15c0-6.1 4.5-10 10-10s10 3.9 10 10v12" stroke="url(#exposelyLogoGrad)" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"></path>
              <path d="M11.5 27V16.2c0-3.8 2-6 4.5-6s4.5 2.2 4.5 6V27" stroke="url(#exposelyLogoGrad)" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" opacity="0.9"></path>
              <ellipse cx="16" cy="16" rx="5.4" ry="2.1" transform="rotate(-18 16 16)" stroke="url(#exposelyLogoGrad)" stroke-width="1.2" opacity="0.8"></ellipse>
              <circle class="logo-core" cx="16" cy="16" r="2.1" fill="url(#exposelyLogoGrad)"></circle>
            </svg>
            <span class="brand-mark-toggle" aria-hidden="true">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"></rect><line x1="9" y1="3" x2="9" y2="21"></line></svg>
            </span>
          </button>
          <div class="brand-text">
            <h1>Exposely</h1>
            <span class="brand-tag">${t('appTagline')}</span>
          </div>
        </div>

        <div class="topbar-actions">
          <div class="dropdown">
            <button type="button" class="stack-toggle ${stackRunningCount > 0 ? 'stack-toggle-on' : ''}" data-action="toggle-stack-menu" aria-haspopup="true" aria-expanded="${state.stackMenuOpen ? 'true' : 'false'}" title="${t('stackMenuTitle')}" ${!stackConfigured ? 'disabled' : ''}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6h16M4 12h16M4 18h16"></path><circle cx="9" cy="6" r="1.6" fill="currentColor" stroke="none"></circle><circle cx="15" cy="12" r="1.6" fill="currentColor" stroke="none"></circle><circle cx="7" cy="18" r="1.6" fill="currentColor" stroke="none"></circle></svg>
              <span>Stack</span>
              <span class="stack-toggle-badge">${stackRunningCount}/${stackTotalCount}</span>
            </button>
            ${state.stackMenuOpen ? stackMenuHtml() : ''}
          </div>
          <button type="button" class="stack-toggle" data-action="open-database-manager" title="${t('dbManagerTitle')}" ${!stackConfigured ? 'disabled' : ''}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"></ellipse><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"></path><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"></path></svg>
            <span>${t('dbManager')}</span>
          </button>
          <button type="button" class="icon-switch" data-action="toggle-theme" aria-label="${nextTheme() === 'light' ? t('switchToLight') : t('switchToDark')}" title="${nextTheme() === 'light' ? t('switchToLight') : t('switchToDark')}">
            ${themeIcon(nextTheme())}
          </button>
          <button type="button" class="icon-switch lang-switch" data-action="toggle-lang" aria-label="${nextLanguage() === 'km' ? t('switchToKhmer') : t('switchToEnglish')}" title="${nextLanguage() === 'km' ? t('switchToKhmer') : t('switchToEnglish')}">
            <span aria-hidden="true">${languageCode(nextLanguage())}</span>
          </button>
        </div>
      </header>

      <div class="app-body">
        <aside class="sidebar">
          <nav class="sidebar-nav" aria-label="Primary">
            <div class="sidebar-label">${t('workspaceSection')}</div>
            ${navItemMarkup('overview', t('projects'), t('navProjectsHint'), appState.settings.projects.length)}
            ${navItemMarkup('logs', t('logs'), t('navLogsHint'))}
            <div class="sidebar-label">${t('systemSection')}</div>
            ${navItemMarkup('settings', t('settings'), t('navSettingsHint'))}
            ${navItemMarkup('about', t('about'), t('navAboutHint'))}
          </nav>

          <div class="sidebar-spacer"></div>

          <button type="button" class="status-card" data-action="tab-settings" title="${t('navSettingsHint')}">
            <div class="status-card-head">
              <span class="status-dot ${tunnelStatus.running ? 'tone-live' : 'tone-idle'}"></span>
              <span class="status-card-title">${t('tunnelLabel')} · ${tunnelStatus.running ? t('running') : t('stopped')}</span>
              <svg class="status-card-arrow" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 18 15 12 9 6"></polyline></svg>
            </div>
            <div class="status-card-sub">${t('stackSectionTitle')} ${stackRunningCount}/${stackTotalCount}</div>
            <div class="status-card-foot">
              <span>${shareToolReady ? t('installed') : t('installRequired')}</span>
              <span class="status-card-v">${escapeHtml(displayVersionLabel(appState.productVersion))}</span>
            </div>
          </button>
        </aside>

        <main class="main-content">
          ${bannersHtml}
          ${editorPanelHtml}
          ${viewHead}
          ${headerHint ? `<p class="header-hint">${escapeHtml(headerHint)}</p>` : ''}
          ${viewBody}
          ${state.notice ? `<section class="toast toast-${state.notice.tone}">${escapeHtml(state.notice.message)}</section>` : ''}
        </main>
      </div>
      ${projectPickerHtml}
    </div>
  `;

  bindForms();
  if (options.refreshProjectForm) {
    const fields: FormSnapshot = {};
    for (const [key, value] of Object.entries(formSnap.fields)) {
      if (!key.startsWith('project-form::')) fields[key] = value;
    }
    const focus = formSnap.focus && formSnap.focus.formKey !== 'project-form' ? formSnap.focus : null;
    restoreFormSnapshot({ fields, focus });
  } else {
    restoreFormSnapshot(formSnap);
  }

  if (!tabChanged) {
    // Re-rendering on the same tab (e.g. after Save Settings, Refresh, or
    // a tunnel-status update) should keep the user's scroll position so
    // they don't get yanked back to the top.
    restoreMainScroll(previousScroll);
  }
  // The project list scrolls independently of the detail pane, so its
  // position survives even tab switches — losing your place in a long list
  // because you opened Settings and came back is exactly what we want to avoid.
  restoreProjectsScroll(previousProjectsScroll);
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

function phpVersionsList(): string {
  const pv = state.phpVersions;
  if (!pv || !pv.installed.length) {
    return '';
  }
  const active = pv.active || state.phpConfig?.managedVersion || '';
  const rows = pv.installed.map((inst) => {
    const isActive = inst.version === active || inst.dir === state.phpConfig?.installDir;
    return `<span class="pill ${isActive ? 'pill-success' : 'pill-outline'} php-version-chip">
      PHP ${escapeHtml(inst.version)}${isActive ? ` · ${t('phpActiveVersion')}` : ''}
      ${!isActive ? `<button type="button" class="php-version-use" data-action="php-use-version" data-version="${escapeHtml(inst.version)}" title="${t('phpUseVersionTitle')}">${t('phpUseVersion')}</button>` : ''}
    </span>`;
  }).join(' ');
  return `<div class="php-versions-row"><span class="summary-label">${t('phpInstalledVersions')}</span><div class="php-version-chips">${rows}</div></div>`;
}

// One compact tab strip for the managed services instead of three cards
// side by side: the active tab shows that service's status and Start/Stop
// action, which stays readable in the narrow settings pane.
function stackStatusTabs(): string {
  if (!state.stackStatuses.length) {
    return '';
  }
  const active = state.stackStatuses.some((st) => st.service === state.stackTab)
    ? state.stackTab
    : state.stackStatuses[0].service;
  const selected = state.stackStatuses.find((st) => st.service === active) ?? state.stackStatuses[0];

  const tabs = state.stackStatuses
    .map((st) => {
      const isActive = st.service === active;
      return `<button type="button" role="tab" aria-selected="${isActive ? 'true' : 'false'}"
        class="stack-tab ${isActive ? 'active' : ''}" data-action="stack-tab" data-id="${escapeHtml(st.service)}">
        <span class="status-dot ${st.running ? 'tone-live' : 'tone-idle'}"></span>
        <span>${escapeHtml(stackServiceLabel(st.service))}</span>
      </button>`;
    })
    .join('');

  const badge = selected.running
    ? `<span class="pill pill-success">${t('running')}</span>`
    : `<span class="pill pill-outline">${t('stopped')}</span>`;
  const detail = selected.running ? `pid ${selected.pid}` : (selected.lastError || '');
  const action = selected.running ? 'stack-stop-service' : 'stack-start-service';

  return `
    <div class="stack-tabs" role="tablist" aria-label="${escapeHtml(t('stackSectionTitle'))}">
      ${tabs}
    </div>
    <div class="stack-tab-panel" role="tabpanel">
      <div class="stack-tab-summary">
        ${badge}
        ${detail ? `<span class="stack-tab-detail">${escapeHtml(detail)}</span>` : ''}
      </div>
      <div class="action-row">
        <button type="button" class="secondary" data-action="${action}" data-id="${escapeHtml(selected.service)}">${selected.running ? t('stackStop') : t('stackStart')}</button>
      </div>
    </div>
  `;
}

function stackServiceLabel(service: string): string {
  switch (service) {
    case 'php':
      return 'PHP';
    case 'mysql':
      return 'MySQL';
    case 'nginx':
      return 'Nginx';
    default:
      return service;
  }
}

// Per-service start/stop menu for the topbar stack pill. Start All / Stop All
// stay available at the bottom so the old one-click behaviour is unchanged.
function stackMenuHtml(): string {
  const runningCount = state.stackStatuses.filter((s) => s.running).length;
  const rows = state.stackStatuses
    .map((st) => `
      <div class="stack-menu-row">
        <div class="stack-menu-copy">
          <span class="stack-menu-service">${escapeHtml(stackServiceLabel(st.service))}</span>
          <span class="stack-menu-state ${st.running ? 'stack-menu-state-on' : ''}">${st.running ? t('running') : t('stopped')}</span>
        </div>
        <button type="button" class="secondary stack-menu-action" data-action="${st.running ? 'stack-stop-service' : 'stack-start-service'}" data-id="${escapeHtml(st.service)}">
          ${st.running ? t('stackStop') : t('stackStart')}
        </button>
      </div>
    `)
    .join('');

  return `<div class="dropdown-menu stack-menu" role="menu">
    <div class="stack-menu-head">${t('stackMenuTitle')}</div>
    ${rows}
    <div class="stack-menu-foot">
      <button type="button" class="secondary stack-menu-all" data-action="stack-start-all">${t('stackStartAll')}</button>
      <button type="button" class="secondary stack-menu-all" data-action="stack-stop-all" ${runningCount === 0 ? 'disabled' : ''}>${t('stackStopAll')}</button>
    </div>
  </div>`;
}

function formChecked(form: HTMLFormElement, name: string): boolean {
  return (form.elements.namedItem(name) as HTMLInputElement | null)?.checked ?? false;
}

function syncEditorFromForm() {
  const projectForm = root.querySelector<HTMLFormElement>('#project-form');
  if (!projectForm) return;

  const shareModeValue = formValue(projectForm, 'shareMode') as ShareMode;
  const validShareModes: ShareMode[] = ['auto', 'quick', 'host-html', 'stable', 'random-domain'];
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

// Filters a list of options in place (no re-render) so the search box keeps
// focus while the user types. Option elements must carry a lowercased
// data-search attribute. An optional empty-state element is toggled when the
// query hides every row. Re-applied after each render so a background status
// poll that rebuilds the list underneath an open picker does not clear it.
function bindInPlaceFilter(
  input: HTMLInputElement,
  optionSelector: string,
  onQuery: (value: string) => void,
  emptyEl: HTMLElement | null = null,
) {
  const apply = () => {
    const query = input.value.trim().toLowerCase();
    let visible = 0;
    root.querySelectorAll<HTMLElement>(optionSelector).forEach((option) => {
      const match = !query || (option.dataset.search ?? '').includes(query);
      option.hidden = !match;
      if (match) visible += 1;
    });
    if (emptyEl) emptyEl.hidden = visible > 0;
  };
  input.addEventListener('input', () => {
    onQuery(input.value);
    apply();
  });
  apply();
  input.focus();
  const caret = input.value.length;
  try {
    input.setSelectionRange(caret, caret);
  } catch {
    // Some input types reject setSelectionRange — ignore.
  }
}

function bindForms() {
  const projectSearch = root.querySelector<HTMLInputElement>('#project-search');
  if (projectSearch) {
    bindInPlaceFilter(
      projectSearch,
      '.project-row',
      (value) => {
        state.projectSearch = value;
      },
      root.querySelector<HTMLElement>('.picker-empty'),
    );
  }

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

  const phpConfigForm = root.querySelector<HTMLFormElement>('#php-config-form');
  phpConfigForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const extraExts = formValue(phpConfigForm, 'extraExts')
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
    const next = await withAction(t('phpSaveConfig'), () =>
      api.savePHPConfig(
        formValue(phpConfigForm, 'memoryLimit'),
        formValue(phpConfigForm, 'uploadMax'),
        formValue(phpConfigForm, 'postMax'),
        Number(formValue(phpConfigForm, 'maxExecTime')) || 120,
        extraExts,
      ));
    if (next) {
      state.appState = next;
      state.phpConfig = await api.getPHPConfig().catch(() => state.phpConfig);
      setNotice('success', t('phpConfigSaved'));
    }
  });

  // Reload the PHP versions panel whenever the settings view renders.
  const settingsRoot = root.querySelector('#settings-view, main, body') ?? root;
  if (!state.phpVersions) {
    void api.listPHPVersions().then((pv) => {
      state.phpVersions = pv;
      if (state.activeTab === 'settings') {
        render();
      }
    }).catch(() => {});
  }
  void settingsRoot;

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
    if (payload.shareMode === 'stable' && !payload.subdomain.trim()) {
      setNotice('error', t('subdomainRequired'));
      return;
    }
    const next = await withAction(t('saveProject'), () => api.saveProject(payload));
    if (!next) return;

    state.appState = next;
    const savedProjectId = payload.id || (next.settings.projects[next.settings.projects.length - 1]?.id ?? null);
    state.selectedProjectId = savedProjectId;
    rememberSelectedProject(savedProjectId);
    if (!state.terminalRunning) {
      state.terminalProjectId = savedProjectId;
    }
    state.editorOpen = false;
    setNotice('success', t('projectSaved'));
  });

  const terminalForm = root.querySelector<HTMLFormElement>('#terminal-command-form');
  terminalForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    await runTerminalCommand();
  });

  const terminalInputForm = root.querySelector<HTMLFormElement>('#terminal-input-form');
  terminalInputForm?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const input = terminalInputForm.querySelector<HTMLInputElement>('input[name=terminalInput]');
    const value = input?.value ?? '';
    if (!value.trim()) return;
    try {
      await api.terminalSendInput(value);
      if (input) input.value = '';
    } catch (error) {
      setNotice('error', error instanceof Error ? error.message : String(error));
    }
  });

  // Keep the embedded terminal scrolled to its newest line after a render.
  const terminalOutput = root.querySelector<HTMLElement>('#terminal-output');
  if (terminalOutput) terminalOutput.scrollTop = terminalOutput.scrollHeight;
}

async function handleAction(action: string, id: string | null, sourceEl: HTMLElement | null = null) {
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
    case 'toggle-sidebar':
      state.sidebarCollapsed = !state.sidebarCollapsed;
      rememberSidebarCollapsed(state.sidebarCollapsed);
      render();
      return;
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
      state.projectPickerOpen = false;
      state.projectSearch = '';
      render();
      return;
    case 'tab-settings':
      state.activeTab = 'settings';
      state.projectMenuOpen = false;
      state.projectPickerOpen = false;
      state.projectSearch = '';
      render();
      return;
    case 'tab-logs':
      state.activeTab = 'logs';
      state.projectMenuOpen = false;
      state.projectPickerOpen = false;
      state.projectSearch = '';
      render();
      return;
    case 'tab-about':
      state.activeTab = 'about';
      state.projectMenuOpen = false;
      state.projectPickerOpen = false;
      state.projectSearch = '';
      render();
      return;
    case 'toggle-project-menu':
      state.projectMenuOpen = !state.projectMenuOpen;
      render();
      return;
    case 'open-project-picker':
      state.projectPickerOpen = true;
      state.projectMenuOpen = false;
      render();
      return;
    case 'close-project-picker':
      state.projectPickerOpen = false;
      state.projectSearch = '';
      render();
      return;
    case 'select-project':
      state.selectedProjectId = id ?? null;
      if (!state.terminalRunning) {
        state.terminalProjectId = state.selectedProjectId;
      }
      rememberSelectedProject(state.selectedProjectId);
      state.projectMenuOpen = false;
      state.projectPickerOpen = false;
      state.projectSearch = '';
      render();
      return;
    case 'new-project':
      state.editorOpen = true;
      state.editorMode = 'create';
      state.editorProject = blankProject();
      state.projectPickerOpen = false;
      state.projectSearch = '';
      render({ refreshProjectForm: true });
      return;
    case 'edit-project':
      if (selectedProject()) {
        state.editorOpen = true;
        state.editorMode = 'edit';
        state.editorProject = { ...selectedProject()! };
        state.projectMenuOpen = false;
        state.projectPickerOpen = false;
        state.projectSearch = '';
        render({ refreshProjectForm: true });
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
        render({ refreshProjectForm: true });
      }
      return;
    }
    case 'random-subdomain': {
      syncEditorFromForm();
      state.editorProject.subdomain = randomSubdomainValue();
      render({ refreshProjectForm: true });
      return;
    }
    case 'php-install': {
      const next = await withAction(t('phpInstall'), () => api.installManagedPHP());
      if (next) {
        state.appState = next;
        state.phpConfig = await api.getPHPConfig().catch(() => state.phpConfig);
        state.phpVersions = await api.listPHPVersions().catch(() => state.phpVersions);
        setNotice('success', t('phpInstallDone'));
      }
      return;
    }
    case 'php-install-version': {
      const select = document.querySelector<HTMLSelectElement>('select[name="phpInstallVersion"]');
      const version = (select?.value ?? '').trim();
      if (!version) {
        setNotice('info', t('phpVersionPickTitle'));
        return;
      }
      const next = await withAction(`${t('phpVersionInstall')} ${version}`, () => api.installPHPVersion(version));
      if (next) {
        state.appState = next;
        state.phpConfig = await api.getPHPConfig().catch(() => state.phpConfig);
        state.phpVersions = await api.listPHPVersions().catch(() => state.phpVersions);
        setNotice('success', `${t('phpVersionInstallDone')}: PHP ${version}`);
      }
      return;
    }
    case 'php-use-version': {
      const version = sourceEl?.dataset.version ?? '';
      if (!version) {
        return;
      }
      const next = await withAction(`${t('phpUseVersion')} PHP ${version}`, () => api.installPHPVersion(version));
      if (next) {
        state.appState = next;
        state.phpConfig = await api.getPHPConfig().catch(() => state.phpConfig);
        state.phpVersions = await api.listPHPVersions().catch(() => state.phpVersions);
        setNotice('success', `${t('phpVersionSwitchDone')}: PHP ${version}`);
      }
      return;
    }
    case 'nginx-install': {
      const next = await withAction(t('nginxInstall'), () => api.installManagedNginx());
      if (next) {
        state.appState = next;
        setNotice('success', t('nginxInstallDone'));
      }
      return;
    }
    case 'mariadb-install': {
      const next = await withAction(t('mariaDBInstall'), () => api.installManagedMariaDB());
      if (next) {
        state.appState = next;
        setNotice('success', t('mariaDBInstallDone'));
      }
      return;
    }
    case 'stack-detect': {
      const before = `${state.appState?.settings.stack?.nginxBinaryPath ?? ''}|${state.appState?.settings.stack?.phpCgiBinaryPath ?? ''}|${state.appState?.settings.stack?.mysqldBinaryPath ?? ''}`;
      const next = await withAction(t('stackDetect'), () => api.detectStackBinaries());
      if (next) {
        state.appState = next;
        syncStackDraftFromState(next);
        const after = `${next.settings.stack?.nginxBinaryPath ?? ''}|${next.settings.stack?.phpCgiBinaryPath ?? ''}|${next.settings.stack?.mysqldBinaryPath ?? ''}`;
        setNotice(after !== before ? 'success' : 'info', after !== before ? t('stackDetectDone') : t('stackDetectNone'));
      }
      return;
    }
    case 'open-database-manager': {
      const next = await withAction(t('dbManagerOpening'), () => api.openDatabaseManager());
      if (next) {
        setNotice('success', `${t('dbManagerOpened')}: ${next}`);
      }
      return;
    }
    case 'overview-tab':
      if (id === 'project' || id === 'terminal') {
        state.overviewTab = id;
        render();
      }
      return;
    case 'stack-tab':
      if (id) {
        state.stackTab = id;
        render();
      }
      return;
    case 'toggle-stack-menu':
      state.stackMenuOpen = !state.stackMenuOpen;
      state.projectMenuOpen = false;
      render();
      return;
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

  if (!id && ['share-project', 'share-random', 'share-quick', 'share-fixed', 'regenerate-url', 'open-url', 'open-project-folder', 'npm-build', 'test-project', 'delete-project'].includes(action)) {
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
      const next = await withAction(t('startRandomUrl'), () => api.shareProjectWithRandomURL(id!));
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
    case 'share-fixed': {
      if (!(await confirmProjectSwitch(id!))) return;
      if (state.appState?.status.running && state.activeProjectId && state.activeProjectId !== id) {
        const stopped = await withAction(t('stop'), () => api.stopTunnel());
        if (!stopped) return;
        state.appState = stopped;
        state.activeProjectId = null;
      }
      const next = await withAction(t('startFixedUrl'), () => api.shareProjectFixedURL(id!));
      if (next) {
        state.appState = next;
        state.activeProjectId = id!;
        if (next.settings.projects) {
          const savedProject = next.settings.projects.find((project) => project.id === id);
          if (savedProject?.publicURL) {
            state.projectUrls[id!] = savedProject.publicURL;
          }
        }
        setNotice('success', t('projectShared'));
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
    case 'open-project-folder':
      await withAction(t('openFolder'), () => api.openProjectFolder(id!));
      return;
    case 'terminal-stop': {
      await withAction(t('stop'), () => api.terminalStop());
      state.terminalRunning = false;
      appendTerminalLine('system', t('terminalStopped'));
      render();
      return;
    }
    case 'terminal-clear':
      state.terminalLines = [];
      render();
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
          rememberSelectedProject(state.selectedProjectId);
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
  if ((state.projectMenuOpen || state.stackMenuOpen) && !insideDropdown) {
    state.projectMenuOpen = false;
    state.stackMenuOpen = false;
    render();
  }

  if (!actionElement) return;

  event.preventDefault();
  const id = actionElement.dataset.id ?? selectedProject()?.id ?? null;
  if (!action) return;
  void handleAction(action, id, actionElement);
});

root.addEventListener('keydown', (event) => {
  if (event.key !== 'Escape') return;
  if (state.projectPickerOpen) {
    state.projectPickerOpen = false;
    state.projectSearch = '';
    render();
    return;
  }
  if (state.projectMenuOpen || state.stackMenuOpen) {
    state.projectMenuOpen = false;
    state.stackMenuOpen = false;
    render();
  }
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
  state.selectedProjectId = rememberedProjectId(next.settings.projects) ?? next.settings.projects[0]?.id ?? null;
  state.activeProjectId = inferActiveProjectId(next);
  syncStackDraftFromState(next);
  state.terminalProjectId = state.selectedProjectId;
  render();

  void api.terminalStatus().then((status) => {
    if (!status?.running) return;
    state.terminalRunning = true;
    state.terminalSession = status.session;
    state.terminalProjectId = status.projectId || state.terminalProjectId;
    state.terminalCommand = status.command || state.terminalCommand;
    render();
  }).catch(() => {
    // Terminal status is optional; ignore when bindings are unavailable.
  });

  void api.stackStatus().then((statuses) => {
    state.stackStatuses = statuses ?? [];
    render();
  }).catch(() => {
    // Stack status is optional; ignore when bindings are unavailable.
  });

  void api.getPHPConfig().then((info) => {
    state.phpConfig = info;
    render();
  }).catch(() => {
    // PHP config panel is optional.
  });

  void api.listPHPVersions().then((pv) => {
    state.phpVersions = pv;
    if (state.activeTab === 'settings') {
      render();
    }
  }).catch(() => {
    // PHP versions panel is optional (older backend).
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

  window.runtime?.EventsOn('terminal-line', (payload) => {
    const event = payload as unknown as TerminalLineEvent;
    if (!state.terminalRunning && !state.terminalStarting) return;
    if (!state.terminalStarting && state.terminalSession && event.session !== state.terminalSession) return;
    appendTerminalLine(event.stream, event.line);
  });

  window.runtime?.EventsOn('terminal-exit', async (payload) => {
    const event = payload as unknown as TerminalExitEvent;
    if (!state.terminalRunning && !state.terminalStarting) return;
    if (!state.terminalStarting && state.terminalSession && event.session !== state.terminalSession) return;
    state.terminalRunning = false;
    state.terminalStarting = false;
    state.terminalExitedSession = event.session;
    appendTerminalLine('system', `${t('terminalExited')} ${event.code}`);
    render();
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
