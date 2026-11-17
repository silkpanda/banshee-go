export namespace main {
	
	export class RelayStatusSnapshot {
	    state: string;
	    relayUrl: string;
	    lastContactAt?: string;
	    lastReplayCount?: number;
	    error?: string;
	    maxSeq: number;
	    alertCount: number;
	    missedCount: number;
	    newestAlert?: store.AlertPreview;
	    tokenSource: string;
	
	    static createFrom(source: any = {}) {
	        return new RelayStatusSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.relayUrl = source["relayUrl"];
	        this.lastContactAt = source["lastContactAt"];
	        this.lastReplayCount = source["lastReplayCount"];
	        this.error = source["error"];
	        this.maxSeq = source["maxSeq"];
	        this.alertCount = source["alertCount"];
	        this.missedCount = source["missedCount"];
	        this.newestAlert = this.convertValues(source["newestAlert"], store.AlertPreview);
	        this.tokenSource = source["tokenSource"];
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
	export class ShellStatus {
	    appName: string;
	    version: string;
	    goVersion: string;
	    platform: string;
	    relayUrl: string;
	    relayState: string;
	    dbReady: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ShellStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.version = source["version"];
	        this.goVersion = source["goVersion"];
	        this.platform = source["platform"];
	        this.relayUrl = source["relayUrl"];
	        this.relayState = source["relayState"];
	        this.dbReady = source["dbReady"];
	    }
	}

}

export namespace secrets {
	
	export class SecretStatus {
	    encryptionAvailable: boolean;
	    turingTokenSet: boolean;
	    deviceTokenSet: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SecretStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.encryptionAvailable = source["encryptionAvailable"];
	        this.turingTokenSet = source["turingTokenSet"];
	        this.deviceTokenSet = source["deviceTokenSet"];
	    }
	}

}

export namespace store {
	
	export class AlertPreview {
	    cameraName: string;
	    eventType: string;
	    occurredAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AlertPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cameraName = source["cameraName"];
	        this.eventType = source["eventType"];
	        this.occurredAt = source["occurredAt"];
	    }
	}
	export class SettingsSnapshot {
	    relayUrl: string;
	    soundVolume: number;
	    soundMuted: boolean;
	    eventTypeFilter: string;
	    autoStart: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SettingsSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relayUrl = source["relayUrl"];
	        this.soundVolume = source["soundVolume"];
	        this.soundMuted = source["soundMuted"];
	        this.eventTypeFilter = source["eventTypeFilter"];
	        this.autoStart = source["autoStart"];
	    }
	}

}

