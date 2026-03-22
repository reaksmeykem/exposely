export namespace models {
	
	export class ProjectPreset {
	    id: string;
	    displayName: string;
	    localHost: string;
	    subdomain: string;
	    publicURL: string;
	    projectPath: string;
	    localURL: string;
	    startCommand: string;
	    shareMode: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.localHost = source["localHost"];
	        this.subdomain = source["subdomain"];
	        this.publicURL = source["publicURL"];
	        this.projectPath = source["projectPath"];
	        this.localURL = source["localURL"];
	        this.startCommand = source["startCommand"];
	        this.shareMode = source["shareMode"];
	    }
	}
	export class AppSettings {
	    defaultDomain: string;
	    tunnelName: string;
	    cloudflaredPath: string;
	    defaultServiceURL: string;
	    licenseToken?: string;
	    projects: ProjectPreset[];
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultDomain = source["defaultDomain"];
	        this.tunnelName = source["tunnelName"];
	        this.cloudflaredPath = source["cloudflaredPath"];
	        this.defaultServiceURL = source["defaultServiceURL"];
	        this.licenseToken = source["licenseToken"];
	        this.projects = this.convertValues(source["projects"], ProjectPreset);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LicenseState {
	    valid: boolean;
	    isAdmin: boolean;
	    owner: string;
	    plan: string;
	    expiresAt: string;
	    deviceId: string;
	    message: string;
	    configured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LicenseState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.isAdmin = source["isAdmin"];
	        this.owner = source["owner"];
	        this.plan = source["plan"];
	        this.expiresAt = source["expiresAt"];
	        this.deviceId = source["deviceId"];
	        this.message = source["message"];
	        this.configured = source["configured"];
	    }
	}
	export class LogEntry {
	    timestamp: string;
	    source: string;
	    level: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.source = source["source"];
	        this.level = source["level"];
	        this.message = source["message"];
	    }
	}
	export class TunnelStatus {
	    tunnelName: string;
	    tunnelId: string;
	    running: boolean;
	    mode: string;
	    pid: number;
	    activeUrl: string;
	    quickUrl: string;
	    htmlServerPort: number;
	    activeHostnames: string[];
	    lastLogs: LogEntry[];
	    lastError: string;
	    detectedCloudflaredPath: string;
	    configPath: string;
	
	    static createFrom(source: any = {}) {
	        return new TunnelStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tunnelName = source["tunnelName"];
	        this.tunnelId = source["tunnelId"];
	        this.running = source["running"];
	        this.mode = source["mode"];
	        this.pid = source["pid"];
	        this.activeUrl = source["activeUrl"];
	        this.quickUrl = source["quickUrl"];
	        this.htmlServerPort = source["htmlServerPort"];
	        this.activeHostnames = source["activeHostnames"];
	        this.lastLogs = this.convertValues(source["lastLogs"], LogEntry);
	        this.lastError = source["lastError"];
	        this.detectedCloudflaredPath = source["detectedCloudflaredPath"];
	        this.configPath = source["configPath"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AppState {
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
	
	    static createFrom(source: any = {}) {
	        return new AppState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settings = this.convertValues(source["settings"], AppSettings);
	        this.status = this.convertValues(source["status"], TunnelStatus);
	        this.license = this.convertValues(source["license"], LicenseState);
	        this.configPath = source["configPath"];
	        this.settingsPath = source["settingsPath"];
	        this.homeDir = source["homeDir"];
	        this.managedCloudflaredPath = source["managedCloudflaredPath"];
	        this.cloudflaredDetected = source["cloudflaredDetected"];
	        this.cloudflaredPath = source["cloudflaredPath"];
	        this.configReadable = source["configReadable"];
	        this.configReadError = source["configReadError"];
	        this.buildRunning = source["buildRunning"];
	        this.buildCommandDetected = source["buildCommandDetected"];
	        this.productVersion = source["productVersion"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	

}

