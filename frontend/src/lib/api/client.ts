import { env } from '$env/dynamic/public';
import { auth } from '$lib/stores/auth.svelte';
import type { paths } from './schema';

// Dev serves the API through the Vite proxy at /api. The production build
// (adapter-node) has no proxy, so PUBLIC_API_URL points the browser straight
// at the backend; it falls back to /api when unset.
const BASE = env.PUBLIC_API_URL || '/api';

// ApiError carries the HTTP status alongside the message so callers can branch
// on it — e.g. a detail page renders a "not found" panel for 404 instead of a
// blank screen, but a generic error for everything else.
export class ApiError extends Error {
	readonly status: number;

	constructor(status: number, message: string) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
	}
}

type HttpMethod = 'get' | 'put' | 'post' | 'delete';

// PathsFor selects the path templates that declare a given HTTP method.
// openapi-typescript types absent methods as optional `never`, so a real
// operation is one whose non-nullable form is not `never`.
type PathsFor<M extends HttpMethod> = {
	[P in keyof paths]: [NonNullable<paths[P][M]>] extends [never] ? never : P;
}[keyof paths];

type GetPath = PathsFor<'get'>;
type PostPath = PathsFor<'post'>;
type PutPath = PathsFor<'put'>;
type DeletePath = PathsFor<'delete'>;

// Op resolves the OpenAPI operation backing a path + method pair.
type Op<P extends keyof paths, M extends HttpMethod> = NonNullable<paths[P][M]>;

type PathParams<O> = O extends { parameters: { path?: infer PP } }
	? [PP] extends [never | undefined]
		? undefined
		: PP
	: undefined;

type Query<O> = O extends { parameters: { query?: infer Q } }
	? [Q] extends [never | undefined]
		? undefined
		: Q
	: undefined;

// JsonBody extracts the JSON request body, or `never` when the operation
// takes none. Absent bodies are typed as optional `never` by openapi-typescript.
type JsonBody<O> = O extends { requestBody?: infer RB }
	? [NonNullable<RB>] extends [never]
		? never
		: NonNullable<RB> extends { content: { 'application/json': infer B } }
			? B
			: never
	: never;

// Response collapses the JSON success bodies (200/201) and a 204 to `void`.
type OpResponse<O> = O extends { responses: infer R }
	?
			| (R extends { 200: { content: { 'application/json': infer T } } } ? T : never)
			| (R extends { 201: { content: { 'application/json': infer T } } } ? T : never)
			| (R extends { 204: unknown } ? void : never)
	: never;

// RequestOpts is the per-call argument: a key is required when the operation
// requires it (path params, JSON body) and optional otherwise (query).
type RequestOpts<O> = (PathParams<O> extends undefined
	? { path?: undefined }
	: { path: PathParams<O> }) &
	(Query<O> extends undefined ? { query?: undefined } : { query?: Query<O> }) &
	([JsonBody<O>] extends [never] ? { body?: undefined } : { body: JsonBody<O> });

// OptsArg makes the opts argument optional only when nothing in it is required.
type OptsArg<O> = Record<string, never> extends RequestOpts<O>
	? [opts?: RequestOpts<O>]
	: [opts: RequestOpts<O>];

// Page-shaped GET responses, the only ones `list` accepts.
type ListPath = {
	[P in GetPath]: OpResponse<Op<P, 'get'>> extends { items: unknown; has_more: boolean }
		? P
		: never;
}[GetPath];

type PageItem<O> = OpResponse<O> extends { items: infer I }
	? NonNullable<I> extends readonly (infer E)[]
		? E
		: never
	: never;

interface CallOpts {
	path?: Record<string, string | number>;
	query?: Record<string, string | number | undefined>;
	body?: unknown;
}

// resolvePath substitutes `{name}` placeholders and appends a query string.
function resolvePath(template: string, opts?: CallOpts): string {
	let path = template;
	if (opts?.path) {
		for (const [key, value] of Object.entries(opts.path)) {
			path = path.replace(`{${key}}`, encodeURIComponent(String(value)));
		}
	}
	if (opts?.query) {
		const params = new URLSearchParams();
		for (const [key, value] of Object.entries(opts.query)) {
			if (value !== undefined) params.set(key, String(value));
		}
		const qs = params.toString();
		if (qs) path += `?${qs}`;
	}
	return path;
}

async function authHeaders(extra: Record<string, string> = {}): Promise<Record<string, string>> {
	const token = await auth.getIdToken();
	const headers: Record<string, string> = { ...extra };
	if (token) headers['Authorization'] = `Bearer ${token}`;
	return headers;
}

async function fail(res: Response): Promise<never> {
	if (res.status === 401) {
		await auth.logout();
		throw new ApiError(401, 'Session expired');
	}
	const body = await res.json().catch(() => ({}));
	throw new ApiError(res.status, body.detail || body.title || `Request failed: ${res.status}`);
}

// sendRequest issues one authenticated request against the API, attaching
// the auth header and routing every non-2xx response through `fail`. It is
// the single place request/upload/download share auth-error handling.
async function sendRequest(
	url: string,
	init: RequestInit,
	extraHeaders: Record<string, string> = {}
): Promise<Response> {
	const headers = await authHeaders(extraHeaders);
	const res = await fetch(`${BASE}${url}`, { ...init, headers });
	if (!res.ok) await fail(res);
	return res;
}

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
	const res = await sendRequest(
		url,
		{ method, body: body === undefined ? undefined : JSON.stringify(body) },
		{ 'Content-Type': 'application/json' }
	);
	if (res.status === 204) return undefined as T;
	return res.json();
}

// listAll walks every page of a paginated endpoint and returns the flattened
// items, so callers without pagination UI keep getting a full array.
async function listAll<T>(template: string, opts?: CallOpts): Promise<T[]> {
	const out: T[] = [];
	const limit = 100;
	let offset = 0;
	for (;;) {
		const url = resolvePath(template, {
			...opts,
			query: { ...opts?.query, limit, offset }
		});
		const page = await request<{ items: T[] | null; has_more: boolean }>('GET', url);
		const items = page.items ?? [];
		out.push(...items);
		if (!page.has_more || items.length === 0) break;
		offset += items.length;
	}
	return out;
}

async function upload<T>(template: string, formData: FormData): Promise<T> {
	const res = await sendRequest(template, { method: 'POST', body: formData });
	return res.json();
}

// MultipartPath selects POST operations whose request body is multipart form
// data — the upload endpoints, the only callers of `api.upload`.
type MultipartPath = {
	[P in PostPath]: Op<P, 'post'> extends {
		requestBody?: { content: { 'multipart/form-data': unknown } };
	}
		? P
		: never;
}[PostPath];

async function download(template: string, opts?: CallOpts): Promise<void> {
	const res = await sendRequest(resolvePath(template, opts), {});
	const disposition = res.headers.get('Content-Disposition') ?? '';
	const match = disposition.match(/filename="?([^"]+)"?/);
	const filename = match ? match[1] : template.split('/').pop() || 'download';
	const blob = await res.blob();
	const objectUrl = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = objectUrl;
	a.download = filename;
	document.body.appendChild(a);
	a.click();
	a.remove();
	URL.revokeObjectURL(objectUrl);
}

// api is the typed low-level client: every path is checked against the
// generated OpenAPI `paths`, and params / body / response are derived from it.
export const api = {
	get<P extends GetPath>(path: P, ...args: OptsArg<Op<P, 'get'>>): Promise<OpResponse<Op<P, 'get'>>> {
		return request('GET', resolvePath(path, args[0] as CallOpts));
	},
	list<P extends ListPath>(path: P, ...args: OptsArg<Op<P, 'get'>>): Promise<PageItem<Op<P, 'get'>>[]> {
		return listAll(path, args[0] as CallOpts);
	},
	post<P extends PostPath>(
		path: P,
		...args: OptsArg<Op<P, 'post'>>
	): Promise<OpResponse<Op<P, 'post'>>> {
		const opts = args[0] as CallOpts | undefined;
		return request('POST', resolvePath(path, opts), opts?.body);
	},
	put<P extends PutPath>(path: P, ...args: OptsArg<Op<P, 'put'>>): Promise<OpResponse<Op<P, 'put'>>> {
		const opts = args[0] as CallOpts | undefined;
		return request('PUT', resolvePath(path, opts), opts?.body);
	},
	del<P extends DeletePath>(
		path: P,
		...args: OptsArg<Op<P, 'delete'>>
	): Promise<OpResponse<Op<P, 'delete'>>> {
		return request('DELETE', resolvePath(path, args[0] as CallOpts));
	},
	upload<P extends MultipartPath>(path: P, formData: FormData): Promise<OpResponse<Op<P, 'post'>>> {
		return upload(path, formData);
	},
	download<P extends GetPath>(path: P, ...args: OptsArg<Op<P, 'get'>>): Promise<void> {
		return download(path, args[0] as CallOpts);
	}
};
