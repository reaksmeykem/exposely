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
  insecureSkipOriginTls: boolean;
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
  // Legacy EnvKit-specific fields. Populated only when the detected
  // stack is specifically EnvKit. New code should prefer the generic
  // localStack* fields below.
  envkitDetected: boolean;
  envkitVersion: string;
  envkitPath: string;
  envkitOriginUrl: string;
  // Generic local-stack fields populated by the Go side for any
  // detected stack (EnvKit, Laravel Herd, Laravel Valet, Laragon, or a
  // generic HTTPS listener on 127.0.0.1:443).
  localStackDetected: boolean;
  localStackKind: string;
  localStackName: string;
  localStackVersion: string;
  localStackPath: string;
  localStackOriginUrl: string;
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
