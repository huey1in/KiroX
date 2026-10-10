export namespace email {
	
	export class CloudMailConfig {
	    name: string;
	    url: string;
	    email: string;
	    password: string;
	    domains: string[];
	
	    static createFrom(source: any = {}) {
	        return new CloudMailConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.email = source["email"];
	        this.password = source["password"];
	        this.domains = source["domains"];
	    }
	}
	export class MailNestConfig {
	    apiKey: string;
	    projectCode: string;
	
	    static createFrom(source: any = {}) {
	        return new MailNestConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	        this.projectCode = source["projectCode"];
	    }
	}
	export class MoeMailConfig {
	    name: string;
	    url: string;
	    apiKey: string;
	
	    static createFrom(source: any = {}) {
	        return new MoeMailConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.apiKey = source["apiKey"];
	    }
	}

}

export namespace proxy {
	
	export class PoolEntry {
	    id: string;
	    name: string;
	    url: string;
	    weight: number;
	    enabled: boolean;
	    probeOk: boolean;
	    probeIp: string;
	    probeCountry?: string;
	    probeCountryCode?: string;
	    probeRegion?: string;
	    probeCity?: string;
	    probeIsp?: string;
	    probeType?: string;
	    probeMs?: number;
	    probeError?: string;
	    probeAt?: number;
	
	    static createFrom(source: any = {}) {
	        return new PoolEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.weight = source["weight"];
	        this.enabled = source["enabled"];
	        this.probeOk = source["probeOk"];
	        this.probeIp = source["probeIp"];
	        this.probeCountry = source["probeCountry"];
	        this.probeCountryCode = source["probeCountryCode"];
	        this.probeRegion = source["probeRegion"];
	        this.probeCity = source["probeCity"];
	        this.probeIsp = source["probeIsp"];
	        this.probeType = source["probeType"];
	        this.probeMs = source["probeMs"];
	        this.probeError = source["probeError"];
	        this.probeAt = source["probeAt"];
	    }
	}

}

export namespace storage {
	
	export class AppSettings {
	    emailProxyMode: string;
	    emailProxy: string;
	    otpTimeoutSeconds: number;
	    retryProfile: string;
	    stopOnRisk: boolean;
	    soundEnabled: boolean;
	    desktopNotifications: boolean;
	    soundVolume: number;
	    autoCheckUpdates: boolean;
	    theme: string;
	    language: string;
	    persistentLogs: boolean;
	    logRetentionDays: number;
	    autoProbeProxies: boolean;
	    moeMailExpiryMinutes: number;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.emailProxyMode = source["emailProxyMode"];
	        this.emailProxy = source["emailProxy"];
	        this.otpTimeoutSeconds = source["otpTimeoutSeconds"];
	        this.retryProfile = source["retryProfile"];
	        this.stopOnRisk = source["stopOnRisk"];
	        this.soundEnabled = source["soundEnabled"];
	        this.desktopNotifications = source["desktopNotifications"];
	        this.soundVolume = source["soundVolume"];
	        this.autoCheckUpdates = source["autoCheckUpdates"];
	        this.theme = source["theme"];
	        this.language = source["language"];
	        this.persistentLogs = source["persistentLogs"];
	        this.logRetentionDays = source["logRetentionDays"];
	        this.autoProbeProxies = source["autoProbeProxies"];
	        this.moeMailExpiryMinutes = source["moeMailExpiryMinutes"];
	    }
	}

}

export namespace task {
	
	export class StartTaskRequest {
	    count: number;
	    concurrency: number;
	    delay: number;
	    outputPath: string;
	    emailProvider: string;
	    moemailDomains: string[];
	    moemailConfigs: Record<string, Array<email.MoeMailConfig>>;
	    moemailRandomMode: boolean;
	    cloudmailDomains: string[];
	    cloudmailConfigs: Record<string, Array<email.CloudMailConfig>>;
	    cloudmailRandomMode: boolean;
	    mailNestConfig: email.MailNestConfig;
	    proxy: string;
	    proxyConfigured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StartTaskRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.concurrency = source["concurrency"];
	        this.delay = source["delay"];
	        this.outputPath = source["outputPath"];
	        this.emailProvider = source["emailProvider"];
	        this.moemailDomains = source["moemailDomains"];
	        this.moemailConfigs = this.convertValues(source["moemailConfigs"], Array<email.MoeMailConfig>, true);
	        this.moemailRandomMode = source["moemailRandomMode"];
	        this.cloudmailDomains = source["cloudmailDomains"];
	        this.cloudmailConfigs = this.convertValues(source["cloudmailConfigs"], Array<email.CloudMailConfig>, true);
	        this.cloudmailRandomMode = source["cloudmailRandomMode"];
	        this.mailNestConfig = this.convertValues(source["mailNestConfig"], email.MailNestConfig);
	        this.proxy = source["proxy"];
	        this.proxyConfigured = source["proxyConfigured"];
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

