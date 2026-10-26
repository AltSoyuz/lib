import { build, files, version } from '$service-worker';

// The worker types this file needs, declared here: it lives in src/lib, type-checked with the pages (DOM lib, no webworker lib).
interface ExtendableEvent extends Event {
	waitUntil(p: Promise<unknown>): void;
}
interface FetchEvent extends ExtendableEvent {
	request: Request;
	respondWith(r: Promise<Response>): void;
}
interface PushEvent extends ExtendableEvent {
	data: { json(): unknown; text(): string } | null;
}
interface NotificationEvent extends ExtendableEvent {
	notification: Notification;
}
interface WindowClient {
	url: string;
	focus(): Promise<WindowClient>;
	navigate(url: string): Promise<WindowClient | null>;
}
interface WorkerScope {
	location: Location;
	registration: ServiceWorkerRegistration;
	clients: {
		claim(): Promise<void>;
		matchAll(o: { type: 'window'; includeUncontrolled: boolean }): Promise<WindowClient[]>;
		openWindow(url: string): Promise<WindowClient | null>;
	};
	skipWaiting(): Promise<void>;
	addEventListener(type: 'install' | 'activate', fn: (e: ExtendableEvent) => void): void;
	addEventListener(type: 'fetch', fn: (e: FetchEvent) => void): void;
	addEventListener(type: 'push', fn: (e: PushEvent) => void): void;
	addEventListener(type: 'notificationclick', fn: (e: NotificationEvent) => void): void;
}

const sw = self as unknown as WorkerScope;

const CACHE = `shell-${version}`;
const SHELL = '/200.html';
const NAV_TIMEOUT_MS = 4000;
// PNG icons are only read by the OS at install time: not worth the mobile data.
const STATIC = [...files.filter((f) => !f.endsWith('.png')), SHELL];
const PRECACHED = new Set([...build, ...STATIC]);

// The whole worker of an app: its src/service-worker.ts calls serve once; name and icon title its notifications.
export function serve(app: { name: string; icon: string }) {
	sw.addEventListener('install', (event) => {
		event.waitUntil(
			(async () => {
				const cache = await caches.open(CACHE);
				// Unhashed files bypass the HTTP cache so a new version never stores an old copy.
				await cache.addAll([...build, ...STATIC.map((f) => new Request(f, { cache: 'reload' }))]);
				await sw.skipWaiting();
			})()
		);
	});

	sw.addEventListener('activate', (event) => {
		event.waitUntil(
			(async () => {
				for (const key of await caches.keys()) {
					if (key !== CACHE) await caches.delete(key);
				}
				await sw.clients.claim();
			})()
		);
	});

	sw.addEventListener('fetch', (event) => {
		const req = event.request;
		if (req.method !== 'GET') return;
		const url = new URL(req.url);
		// API responses are never cached: what the server says must be live.
		if (url.origin !== sw.location.origin || url.pathname.startsWith('/api/')) return;

		if (req.mode === 'navigate') {
			event.respondWith(networkThenShell(req));
		} else if (PRECACHED.has(url.pathname)) {
			event.respondWith(cacheThenNetwork(url.pathname, req));
		}
	});

	// Every push shows a notification: Safari stops pushing to a web app that receives one without showing it.
	sw.addEventListener('push', (event) => {
		let m: { title?: string; body?: string; url?: string; tag?: string } = {};
		try {
			m = (event.data?.json() as typeof m | undefined) ?? {};
		} catch {
			m = { body: event.data?.text() };
		}
		event.waitUntil(
			sw.registration.showNotification(m.title || app.name, { body: m.body ?? '', tag: m.tag, data: { url: m.url || '/' }, icon: app.icon })
		);
	});

	sw.addEventListener('notificationclick', (event) => {
		event.notification.close();
		const url = new URL((event.notification.data as { url?: string } | null)?.url ?? '/', sw.location.origin).href;
		event.waitUntil(openAt(url));
	});
}

async function networkThenShell(req: Request): Promise<Response> {
	const network = fetch(req);
	network.catch(() => {});
	// On a connection that hangs, open the cached shell after a few seconds; the page fetches its own data.
	const late = new Promise<null>((resolve) => setTimeout(() => resolve(null), NAV_TIMEOUT_MS));
	try {
		const res = await Promise.race([network, late]);
		if (res) return res;
	} catch {
		// Offline: fall through to the cached shell.
	}
	const shell = await (await caches.open(CACHE)).match(SHELL);
	return shell ?? network;
}

async function cacheThenNetwork(path: string, req: Request): Promise<Response> {
	const hit = await (await caches.open(CACHE)).match(path);
	return hit ?? fetch(req);
}

// The app already open comes to the front on the page; otherwise it opens there.
async function openAt(url: string) {
	const [client] = await sw.clients.matchAll({ type: 'window', includeUncontrolled: true });
	if (!client) {
		await sw.clients.openWindow(url);
		return;
	}
	const focused = await client.focus();
	if (focused.url === url) return;
	try {
		await focused.navigate(url);
	} catch {
		// A page the worker does not control cannot be navigated from here.
		await sw.clients.openWindow(url);
	}
}
