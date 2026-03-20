export type ShareMode = 'stable' | 'random-domain' | 'quick';

export interface ProjectPreset {
  id: string;
  displayName: string;
  localHost: string;
  subdomain: string;
  publicURL: string;
  projectPath: string;
  shareMode: ShareMode;
}

export interface AppSettings {
  defaultDomain: string;
  tunnelName: string;
  cloudflaredPath: string;
  defaultServiceURL: string;
  licenseToken?: string;
  projects: ProjectPreset[];
}

export interface LicenseState {
  valid: boolean;
  isAdmin: boolean;
  owner: string;
  plan: string;
  expiresAt: string;
  deviceId: string;
  message: string;
  configured: boolean;
}

export interface LogEntry {
  timestamp: string;
  source: string;
  level: string;
  message: string;
}

export interface TunnelStatus {
  tunnelName: string;
  tunnelId: string;
  running: boolean;
  mode: string;
  pid: number;
  activeUrl: string;
  quickUrl: string;
  activeHostnames: string[];
  lastLogs: LogEntry[];
  lastError: string;
  detectedCloudflaredPath: string;
  configPath: string;
}

export interface AppState {
  settings: AppSettings;
  status: TunnelStatus;
  license: LicenseState;
  configPath: string;
  settingsPath: string;
  homeDir: string;
  managedCloudflaredPath: string;
  cloudflaredDetected: boolean;
  cloudflaredPath: string;
  configReadable: boolean;
  configReadError: string;
  buildRunning: boolean;
  buildCommandDetected: boolean;
  productVersion: string;
}
