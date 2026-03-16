import type { AppState, AppSettings, ProjectPreset, TunnelStatus, LogEntry } from './types';

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          Bootstrap(): Promise<AppState>;
          RefreshState(): Promise<AppState>;
          SaveSettings(settings: AppSettings): Promise<AppState>;
          SaveProject(project: ProjectPreset): Promise<AppState>;
          DeleteProject(id: string): Promise<AppState>;
          ShareProject(id: string): Promise<AppState>;
          ShareProjectWithRandomURL(id: string): Promise<AppState>;
          StartQuickTunnel(id: string): Promise<AppState>;
          StartTunnel(): Promise<AppState>;
          StopTunnel(): Promise<AppState>;
          CreateTunnel(): Promise<AppState>;
          RunNpmBuild(id: string): Promise<void>;
          OpenPublicURL(id: string): Promise<void>;
          OpenConfigFile(): Promise<void>;
          OpenSettingsFile(): Promise<void>;
          BrowseProjectFolder(currentPath: string): Promise<string>;
          TestProject(id: string): Promise<string>;
        };
      };
    };
    runtime?: {
      EventsOn(name: string, callback: (payload: TunnelStatus | LogEntry) => void): void;
    };
  }
}

export {};
