import type { AppSettings, AppState, ProjectPreset } from './types';

type AppBindings = NonNullable<NonNullable<NonNullable<typeof window.go>['main']>['App']>;

function appBinding(): AppBindings {
  const binding = window.go?.main?.App;
  if (!binding) {
    throw new Error('Wails bindings are not available. Run the project with `wails dev` or `wails build`.');
  }
  return binding;
}

export const api = {
  bootstrap: (): Promise<AppState> => appBinding().Bootstrap(),
  refreshState: (): Promise<AppState> => appBinding().RefreshState(),
  ensureCloudflared: (): Promise<AppState> => (appBinding() as AppBindings & { EnsureCloudflared(): Promise<AppState> }).EnsureCloudflared(),
  saveSettings: (settings: AppSettings): Promise<AppState> => appBinding().SaveSettings(settings),
  saveProject: (project: ProjectPreset): Promise<AppState> => appBinding().SaveProject(project),
  deleteProject: (id: string): Promise<AppState> => appBinding().DeleteProject(id),
  shareProject: (id: string): Promise<AppState> => appBinding().ShareProject(id),
  shareProjectWithRandomURL: (id: string): Promise<AppState> => appBinding().ShareProjectWithRandomURL(id),
  startQuickTunnel: (id: string): Promise<AppState> => appBinding().StartQuickTunnel(id),
  startTunnel: (): Promise<AppState> => appBinding().StartTunnel(),
  stopTunnel: (): Promise<AppState> => appBinding().StopTunnel(),
  createTunnel: (): Promise<AppState> => appBinding().CreateTunnel(),
  runNpmBuild: (id: string): Promise<void> => appBinding().RunNpmBuild(id),
  openPublicURL: (id: string): Promise<void> => appBinding().OpenPublicURL(id),
  openConfigFile: (): Promise<void> => appBinding().OpenConfigFile(),
  openSettingsFile: (): Promise<void> => appBinding().OpenSettingsFile(),
  browseProjectFolder: (currentPath: string): Promise<string> => appBinding().BrowseProjectFolder(currentPath),
  testProject: (id: string): Promise<string> => appBinding().TestProject(id),
};
