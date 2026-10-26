// The error of an API call: message is the server's error code ({"error": "..."}), or <fallback>_<status> without one.
export class HttpError extends Error {
	status: number;
	// The whole JSON error answer, for the fields some errors carry beside their code.
	body: Record<string, unknown> | null;
	constructor(code: string, status: number, body: Record<string, unknown> | null = null) {
		super(code);
		this.status = status;
		this.body = body;
	}
}

// One JSON call to the app's own API: a 401 throws HttpError('unauthorized'), any other failure its code; the caller reads the body.
export async function request(method: string, url: string, body?: unknown, signal?: AbortSignal, fallback = 'request_failed'): Promise<Response> {
	const res = await fetch(url, {
		method,
		credentials: 'same-origin',
		headers: body === undefined ? undefined : { 'content-type': 'application/json' },
		body: body === undefined ? undefined : JSON.stringify(body),
		signal
	});
	if (res.status === 401) throw new HttpError('unauthorized', 401);
	if (!res.ok) {
		const data = (await res.json().catch(() => null)) as Record<string, unknown> | null;
		const code = typeof data?.error === 'string' ? data.error : `${fallback}_${res.status}`;
		throw new HttpError(code, res.status, data);
	}
	return res;
}

// fetch rejects with a TypeError when the network fails, before any HTTP status.
export function errorText(e: unknown): string {
	if (e instanceof TypeError) return 'Cannot reach the server. Check your connection and try again.';
	return (e as Error).message;
}
