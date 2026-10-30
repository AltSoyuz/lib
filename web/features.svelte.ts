import { browser } from '$app/environment';
import { request } from './api';

// The features of lib's backoffice (GET /api/v1/features): what an admin keeps to the admins, hidden from the others.

export type Audience = 'everyone' | 'admins';

export interface Feature {
	name: string;
	label: string;
	audience: Audience;
}

export const features = $state<{ list: Feature[] }>({ list: [] });

let key = '';

// Called once by the root layout, with the localStorage key of the app's features: the ones kept on this phone show at
// once, offline included, until the server's answer.
export function initFeatures(storageKey: string) {
	key = storageKey;
	if (!browser) return;
	try {
		features.list = JSON.parse(localStorage.getItem(key) ?? '[]') as Feature[];
	} catch (e) {
		console.warn('cannot read the features kept on this phone:', e);
	}
	void loadFeatures();
}

// loadFeatures reads the server's features; a failure keeps the ones shown.
export async function loadFeatures(): Promise<void> {
	try {
		keepFeatures(((await (await request('GET', '/api/v1/features')).json()) as { features: Feature[] }).features);
	} catch (e) {
		console.warn('cannot read /api/v1/features:', e);
	}
}

// keepFeatures shows list and keeps it on this phone.
export function keepFeatures(list: Feature[]) {
	features.list = list;
	try {
		localStorage.setItem(key, JSON.stringify(list));
	} catch (e) {
		console.warn('cannot keep the features on this phone:', e);
	}
}

// featureOn: a feature for everyone, or one the server never named, is on; one for the admins needs the role of /me,
// so it stays hidden until /me answers.
export function featureOn(name: string, role: string | null | undefined): boolean {
	const feature = features.list.find((f) => f.name === name);
	return !feature || feature.audience === 'everyone' || role === 'admin';
}
