export type ShareMode = 'auto' | 'stable' | 'random-domain' | 'quick' | 'host-html';

export interface ProjectPreset {
  id: string;
  displayName: string;
  localHost: string;
  originURL: string;
  subdomain: string;
  publicURL: string;
  projectPath: string;
  localURL: string;
  startCommand: string;
  shareMode: ShareMode;
}

export interface AppSettings {
  defaultDomain: string;
  tunnelName: string;
  cloudflaredPath: string;
  defaultServiceURL: string;
  licenseToken?: string;
  language: string;
  theme?: 'dark' | 'light';
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
  usage?: TunnelUsage | null;
}

export interface TunnelUsage {
  available: boolean;
  startedAt: string;
  uptimeSeconds: number;
  totalRequests: number;
  requestsPerMin: number;
  activeConns: number;
  haConnections: number;
  responsesByCode?: Record<string, number> | null;
  edgeLocations?: string[] | null;
  lastUpdated: string;
  metricsAddr?: string;
  note?: string;
}

export interface AppState {
  settings: AppSettings;
  status: TunnelStatus;
  license: LicenseState;
  update: UpdateInfo;
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

export interface UpdateInfo {
  checked: boolean;
  available: boolean;
  currentVersion: string;
  latestVersion: string;
  releaseUrl: string;
  message: string;
}
