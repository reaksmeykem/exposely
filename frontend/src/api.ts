import type { AppSettings, AppState, ProjectPreset } from './types';

type AppBindings = NonNullable<NonNullable<NonNullable<typeof window.go>['main']>['App']>;

function appBinding(): AppBindings {
  const binding = window.go?.main?.App;
  if (!binding) {
    const isWails = !!(window as any).wails || !!(window as any).runtime;
    if (isWails) {
        throw new Error('Wails bindings are not yet available. This might happen if Bootstrap is called too early.');
    }
    throw new Error('Wails bindings are not available. Please ensure you are running the application via Wails.');
  }
  return binding;
}

export const api = {
  bootstrap: (): Promise<AppState> => appBinding().Bootstrap(),
  checkForUpdates: (): Promise<AppState> => (appBinding() as AppBindings & { CheckForUpdates(): Promise<AppState> }).CheckForUpdates(),
  refreshState: (): Promise<AppState> => appBinding().RefreshState(),
  ensureCloudflared: (): Promise<AppState> => (appBinding() as AppBindings & { EnsureCloudflared(): Promise<AppState> }).EnsureCloudflared(),
  installCloudflared: (): Promise<AppState> => (appBinding() as AppBindings & { InstallCloudflared(): Promise<AppState> }).InstallCloudflared(),
  saveSettings: (settings: AppSettings): Promise<AppState> => appBinding().SaveSettings(settings),
  saveProject: (project: ProjectPreset): Promise<AppState> => appBinding().SaveProject(project),
  deleteProject: (id: string): Promise<AppState> => appBinding().DeleteProject(id),
  shareProject: (id: string): Promise<AppState> => appBinding().ShareProject(id),
  shareProjectWithRandomURL: (id: string): Promise<AppState> => appBinding().ShareProjectWithRandomURL(id),
  startQuickTunnel: (id: string): Promise<AppState> => appBinding().StartQuickTunnel(id),
  startTunnel: (): Promise<AppState> => appBinding().StartTunnel(),
  stopTunnel: (): Promise<AppState> => appBinding().StopTunnel(),
  createTunnel: (): Promise<AppState> => appBinding().CreateTunnel(),
  activateLicense: (token: string): Promise<AppState> => (appBinding() as AppBindings & { ActivateLicense(token: string): Promise<AppState> }).ActivateLicense(token),
  clearLicense: (): Promise<AppState> => (appBinding() as AppBindings & { ClearLicense(): Promise<AppState> }).ClearLicense(),
  runNpmBuild: (id: string): Promise<void> => appBinding().RunNpmBuild(id),
  openPublicURL: (id: string): Promise<void> => appBinding().OpenPublicURL(id),
  openConfigFile: (): Promise<void> => appBinding().OpenConfigFile(),
  openSettingsFile: (): Promise<void> => appBinding().OpenSettingsFile(),
  openLatestRelease: (): Promise<void> => (appBinding() as AppBindings & { OpenLatestRelease(): Promise<void> }).OpenLatestRelease(),
  browseProjectFolder: (currentPath: string): Promise<string> => appBinding().BrowseProjectFolder(currentPath),
  testProject: (id: string): Promise<string> => appBinding().TestProject(id),
};
