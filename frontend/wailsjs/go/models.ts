export namespace main {
	
	export class BaseStationConfiguration {
	    mac_address: string;
	    // Go type: time
	    last_seen: any;
	    channel: number;
	    nickname: string;
	    id: string;
	    managed_flags: number;
	
	    static createFrom(source: any = {}) {
	        return new BaseStationConfiguration(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mac_address = source["mac_address"];
	        this.last_seen = this.convertValues(source["last_seen"], null);
	        this.channel = source["channel"];
	        this.nickname = source["nickname"];
	        this.id = source["id"];
	        this.managed_flags = source["managed_flags"];
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
	export class Group {
	    name: string;
	    managed_flags: number;
	    base_stations: string[];
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.managed_flags = source["managed_flags"];
	        this.base_stations = source["base_stations"];
	    }
	}
	export class Configuration {
	    is_steamvr_managed: boolean;
	    is_steamvr_installed: boolean;
	    allow_tray: boolean;
	    tray_notified: boolean;
	    known_base_stations: Record<string, BaseStationConfiguration>;
	    groups: Record<string, Group>;
	    branch: string;
	
	    static createFrom(source: any = {}) {
	        return new Configuration(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_steamvr_managed = source["is_steamvr_managed"];
	        this.is_steamvr_installed = source["is_steamvr_installed"];
	        this.allow_tray = source["allow_tray"];
	        this.tray_notified = source["tray_notified"];
	        this.known_base_stations = this.convertValues(source["known_base_stations"], BaseStationConfiguration, true);
	        this.groups = this.convertValues(source["groups"], Group, true);
	        this.branch = source["branch"];
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
	
	export class RoomStatus {
	    supported: boolean;
	    steamvr_running: boolean;
	    helper_found: boolean;
	    room_setup_found: boolean;
	    watcher_running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RoomStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.steamvr_running = source["steamvr_running"];
	        this.helper_found = source["helper_found"];
	        this.room_setup_found = source["room_setup_found"];
	        this.watcher_running = source["watcher_running"];
	    }
	}
	export class Zone {
	    id: string;
	    name: string;
	    data?: string;
	    play_area: number[];
	    created_at: string;
	    auto_apply: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Zone(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.data = source["data"];
	        this.play_area = source["play_area"];
	        this.created_at = source["created_at"];
	        this.auto_apply = source["auto_apply"];
	    }
	}

}

