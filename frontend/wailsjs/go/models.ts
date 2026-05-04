export namespace cleaner {
	
	export class CleanupItem {
	    path: string;
	    operation: string;
	    target?: string;
	
	    static createFrom(source: any = {}) {
	        return new CleanupItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.operation = source["operation"];
	        this.target = source["target"];
	    }
	}
	export class CleanupResult {
	    path: string;
	    success: boolean;
	    error?: string;
	    freedBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new CleanupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.success = source["success"];
	        this.error = source["error"];
	        this.freedBytes = source["freedBytes"];
	    }
	}

}

export namespace llm {
	
	export class ConnectionTestResult {
	    success: boolean;
	    error?: string;
	    noFunctionCalling: boolean;
	    modelInfo?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.noFunctionCalling = source["noFunctionCalling"];
	        this.modelInfo = source["modelInfo"];
	    }
	}
	export class LLMConfig {
	    provider: string;
	    endpoint: string;
	    apiKey: string;
	    modelName: string;
	    isActive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LLMConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.endpoint = source["endpoint"];
	        this.apiKey = source["apiKey"];
	        this.modelName = source["modelName"];
	        this.isActive = source["isActive"];
	    }
	}

}

export namespace platform {
	
	export class DriveInfo {
	    path: string;
	    label: string;
	    total: number;
	    used: number;
	    free: number;
	    isSystem: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DriveInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.label = source["label"];
	        this.total = source["total"];
	        this.used = source["used"];
	        this.free = source["free"];
	        this.isSystem = source["isSystem"];
	    }
	}
	export class QuickTarget {
	    path: string;
	    label: string;
	    description: string;
	    icon: string;
	
	    static createFrom(source: any = {}) {
	        return new QuickTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.label = source["label"];
	        this.description = source["description"];
	        this.icon = source["icon"];
	    }
	}

}

export namespace scanner {
	
	export class FileEntry {
	    path: string;
	    name: string;
	    size: number;
	    // Go type: time
	    modTime: any;
	    // Go type: time
	    accessTime: any;
	    isDir: boolean;
	    extension: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.modTime = this.convertValues(source["modTime"], null);
	        this.accessTime = this.convertValues(source["accessTime"], null);
	        this.isDir = source["isDir"];
	        this.extension = source["extension"];
	        this.error = source["error"];
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
	export class ScanOptions {
	    maxDepth: number;
	    excludeDirs: string[];
	
	    static createFrom(source: any = {}) {
	        return new ScanOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxDepth = source["maxDepth"];
	        this.excludeDirs = source["excludeDirs"];
	    }
	}
	export class ScanResult {
	    root: string;
	    totalFiles: number;
	    totalDirs: number;
	    totalSize: number;
	    durationMs: number;
	    entries: FileEntry[];
	    errors: FileEntry[];
	
	    static createFrom(source: any = {}) {
	        return new ScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.totalFiles = source["totalFiles"];
	        this.totalDirs = source["totalDirs"];
	        this.totalSize = source["totalSize"];
	        this.durationMs = source["durationMs"];
	        this.entries = this.convertValues(source["entries"], FileEntry);
	        this.errors = this.convertValues(source["errors"], FileEntry);
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

export namespace store {
	
	export class CleanupLog {
	    id: number;
	    scanId: number;
	    filePath: string;
	    operation: string;
	    targetPath: string;
	    result: string;
	    errorMessage: string;
	    freedBytes: number;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new CleanupLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.scanId = source["scanId"];
	        this.filePath = source["filePath"];
	        this.operation = source["operation"];
	        this.targetPath = source["targetPath"];
	        this.result = source["result"];
	        this.errorMessage = source["errorMessage"];
	        this.freedBytes = source["freedBytes"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class LLMConfig {
	    id: number;
	    provider: string;
	    endpoint: string;
	    apiKey: string;
	    modelName: string;
	    isActive: boolean;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new LLMConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.provider = source["provider"];
	        this.endpoint = source["endpoint"];
	        this.apiKey = source["apiKey"];
	        this.modelName = source["modelName"];
	        this.isActive = source["isActive"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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

