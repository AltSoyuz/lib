import { request } from './api';

// The phone side of lib's push package (GET /api/v1/push, PUT and DELETE /api/v1/push/subscription).
// unavailable: the server has them off, or the browser cannot; install: an iPhone gets them only from the installed app.
export type PushState = 'unavailable' | 'install' | 'blocked' | 'off' | 'on';

// The server's key, base64url, as the bytes pushManager.subscribe takes.
export function keyBytes(key: string): Uint8Array<ArrayBuffer> {
	const b64 = key.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(key.length / 4) * 4, '=');
	const raw = atob(b64);
	const out = new Uint8Array(raw.length);
	for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
	return out;
}

const supported = () => typeof window !== 'undefined' && 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window;

async function subscription(): Promise<PushSubscription | null> {
	const reg = await navigator.serviceWorker.getRegistration();
	return (await reg?.pushManager.getSubscription()) ?? null;
}

export async function pushState(standalone: boolean): Promise<{ state: PushState; key: string }> {
	const { key } = (await (await request('GET', '/api/v1/push')).json()) as { key: string };
	if (!key) return { state: 'unavailable', key };
	if (!supported()) return { state: standalone ? 'unavailable' : 'install', key };
	if (Notification.permission === 'denied') return { state: 'blocked', key };
	return { state: Notification.permission === 'granted' && (await subscription()) ? 'on' : 'off', key };
}

// Called from a tap: iOS asks for the permission only from a user gesture.
export async function turnOn(key: string): Promise<PushState> {
	const permission = await Notification.requestPermission();
	if (permission !== 'granted') return permission === 'denied' ? 'blocked' : 'off';
	const reg = await navigator.serviceWorker.ready;
	const sub =
		(await reg.pushManager.getSubscription()) ?? (await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: keyBytes(key) }));
	try {
		await request('PUT', '/api/v1/push/subscription', sub.toJSON(), undefined, 'push_failed');
	} catch (e) {
		await sub.unsubscribe();
		throw e;
	}
	return 'on';
}

export async function turnOff(): Promise<PushState> {
	const sub = await subscription();
	if (sub) {
		await request('DELETE', '/api/v1/push/subscription', { endpoint: sub.endpoint }, undefined, 'push_failed');
		await sub.unsubscribe();
	}
	return 'off';
}
