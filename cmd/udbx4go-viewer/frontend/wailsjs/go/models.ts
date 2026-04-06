export namespace main {
	
	export class DatasetInfoDTO {
	    name: string;
	    kind: string;
	    objectCount: number;
	    iconType: string;
	
	    static createFrom(source: any = {}) {
	        return new DatasetInfoDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.objectCount = source["objectCount"];
	        this.iconType = source["iconType"];
	    }
	}
	export class FileInfo {
	    path: string;
	    datasetCount: number;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.datasetCount = source["datasetCount"];
	    }
	}
	export class PageData {
	    rows: string[][];
	    columns: string[];
	    currentPage: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new PageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = source["rows"];
	        this.columns = source["columns"];
	        this.currentPage = source["currentPage"];
	        this.totalPages = source["totalPages"];
	    }
	}

}

