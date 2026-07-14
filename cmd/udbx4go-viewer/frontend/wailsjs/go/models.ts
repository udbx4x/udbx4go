export namespace main {

	export class AdvancedSettingsDTO {
	    showPreviewStats: boolean;

	    static createFrom(source: any = {}) {
	        return new AdvancedSettingsDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.showPreviewStats = source["showPreviewStats"];
	    }
	}
	export class BenchmarkSelectionDTO {
	    datasetName: string;
	    page: number;
	    rowIndex: number;

	    static createFrom(source: any = {}) {
	        return new BenchmarkSelectionDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.datasetName = source["datasetName"];
	        this.page = source["page"];
	        this.rowIndex = source["rowIndex"];
	    }
	}
	export class BenchmarkScenarioDTO {
	    name: string;
	    filePath: string;
	    layers: string[];
	    selection: BenchmarkSelectionDTO;

	    static createFrom(source: any = {}) {
	        return new BenchmarkScenarioDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.filePath = source["filePath"];
	        this.layers = source["layers"];
	        this.selection = this.convertValues(source["selection"], BenchmarkSelectionDTO);
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
	export class BenchmarkConfigDTO {
	    runId: string;
	    outputPath: string;
	    scenario: BenchmarkScenarioDTO;

	    static createFrom(source: any = {}) {
	        return new BenchmarkConfigDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.outputPath = source["outputPath"];
	        this.scenario = this.convertValues(source["scenario"], BenchmarkScenarioDTO);
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
	export class BenchmarkMetricsDTO {
	    openFileMs: number;
	    loadLayersMs: number;
	    fitVisibleLayersMs: number;
	    selectAndFitMs: number;

	    static createFrom(source: any = {}) {
	        return new BenchmarkMetricsDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.openFileMs = source["openFileMs"];
	        this.loadLayersMs = source["loadLayersMs"];
	        this.fitVisibleLayersMs = source["fitVisibleLayersMs"];
	        this.selectAndFitMs = source["selectAndFitMs"];
	    }
	}
	export class BenchmarkResultDTO {
	    runId: string;
	    status: string;
	    startedAt: string;
	    scenario: string;
	    metrics: BenchmarkMetricsDTO;
	    error: string;

	    static createFrom(source: any = {}) {
	        return new BenchmarkResultDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.status = source["status"];
	        this.startedAt = source["startedAt"];
	        this.scenario = source["scenario"];
	        this.metrics = this.convertValues(source["metrics"], BenchmarkMetricsDTO);
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


	export class BoundingBoxDTO {
	    minX: number;
	    minY: number;
	    maxX: number;
	    maxY: number;

	    static createFrom(source: any = {}) {
	        return new BoundingBoxDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.minX = source["minX"];
	        this.minY = source["minY"];
	        this.maxX = source["maxX"];
	        this.maxY = source["maxY"];
	    }
	}
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
	export class FeatureAttributesDTO {
	    datasetName: string;
	    id: number;
	    geometryType: string;
	    bbox?: BoundingBoxDTO;
	    properties: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new FeatureAttributesDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.datasetName = source["datasetName"];
	        this.id = source["id"];
	        this.geometryType = source["geometryType"];
	        this.bbox = this.convertValues(source["bbox"], BoundingBoxDTO);
	        this.properties = source["properties"];
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
	export class MapInteractionSettingsDTO {
	    zoomToSelectedFeature: boolean;

	    static createFrom(source: any = {}) {
	        return new MapInteractionSettingsDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.zoomToSelectedFeature = source["zoomToSelectedFeature"];
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
	export class PreviewGeometryDTO {
	    type: string;
	    coordinates: any[];
	    hasZ: boolean;

	    static createFrom(source: any = {}) {
	        return new PreviewGeometryDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.coordinates = source["coordinates"];
	        this.hasZ = source["hasZ"];
	    }
	}
	export class PreviewFeatureDTO {
	    id: number;
	    geometry: PreviewGeometryDTO;
	    bbox?: BoundingBoxDTO;
	    properties?: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new PreviewFeatureDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.geometry = this.convertValues(source["geometry"], PreviewGeometryDTO);
	        this.bbox = this.convertValues(source["bbox"], BoundingBoxDTO);
	        this.properties = source["properties"];
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

	export class SpatialPreviewDTO {
	    datasetName: string;
	    kind: string;
	    srid?: number;
	    extent?: BoundingBoxDTO;
	    features: PreviewFeatureDTO[];
	    estimatedVertexCount: number;
	    sampled: boolean;
	    sampleReason?: string;

	    static createFrom(source: any = {}) {
	        return new SpatialPreviewDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.datasetName = source["datasetName"];
	        this.kind = source["kind"];
	        this.srid = source["srid"];
	        this.extent = this.convertValues(source["extent"], BoundingBoxDTO);
	        this.features = this.convertValues(source["features"], PreviewFeatureDTO);
	        this.estimatedVertexCount = source["estimatedVertexCount"];
	        this.sampled = source["sampled"];
	        this.sampleReason = source["sampleReason"];
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
	export class SpatialPreviewRequestDTO {
	    viewport?: BoundingBoxDTO;
	    limit: number;
	    maxVertices: number;
	    simplify: boolean;

	    static createFrom(source: any = {}) {
	        return new SpatialPreviewRequestDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.viewport = this.convertValues(source["viewport"], BoundingBoxDTO);
	        this.limit = source["limit"];
	        this.maxVertices = source["maxVertices"];
	        this.simplify = source["simplify"];
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
	export class SpatialPreviewSettingsDTO {
	    featureLimit: number;
	    vertexBudget: number;
	    autoFitOnLayerChange: boolean;

	    static createFrom(source: any = {}) {
	        return new SpatialPreviewSettingsDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.featureLimit = source["featureLimit"];
	        this.vertexBudget = source["vertexBudget"];
	        this.autoFitOnLayerChange = source["autoFitOnLayerChange"];
	    }
	}
	export class SpatialSummaryDTO {
	    datasetName: string;
	    kind: string;
	    srid?: number;
	    extent?: BoundingBoxDTO;
	    objectCount: number;
	    estimatedVertexCount: number;
	    previewSupported: boolean;
	    unsupportedReason?: string;

	    static createFrom(source: any = {}) {
	        return new SpatialSummaryDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.datasetName = source["datasetName"];
	        this.kind = source["kind"];
	        this.srid = source["srid"];
	        this.extent = this.convertValues(source["extent"], BoundingBoxDTO);
	        this.objectCount = source["objectCount"];
	        this.estimatedVertexCount = source["estimatedVertexCount"];
	        this.previewSupported = source["previewSupported"];
	        this.unsupportedReason = source["unsupportedReason"];
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
	export class TableSettingsDTO {
	    defaultOpen: boolean;

	    static createFrom(source: any = {}) {
	        return new TableSettingsDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultOpen = source["defaultOpen"];
	    }
	}
	export class ViewerSettingsDTO {
	    spatialPreview: SpatialPreviewSettingsDTO;
	    mapInteraction: MapInteractionSettingsDTO;
	    table: TableSettingsDTO;
	    advanced: AdvancedSettingsDTO;

	    static createFrom(source: any = {}) {
	        return new ViewerSettingsDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.spatialPreview = this.convertValues(source["spatialPreview"], SpatialPreviewSettingsDTO);
	        this.mapInteraction = this.convertValues(source["mapInteraction"], MapInteractionSettingsDTO);
	        this.table = this.convertValues(source["table"], TableSettingsDTO);
	        this.advanced = this.convertValues(source["advanced"], AdvancedSettingsDTO);
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

