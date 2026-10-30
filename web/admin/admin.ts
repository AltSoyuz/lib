// The back office API of lib's backoffice package (/api/v1/admin/*), its types and the account labels.
import { request } from '../api';
import type { Audience, Feature } from '../features.svelte';

export type AccessState = 'active' | 'restrained' | 'pending';

export interface AdminUser {
	id: string;
	email: string;
	role: string;
	access_state: AccessState;
	created_at: string;
	last_seen_at: string;
	active_session_count: number;
	connected: boolean;
}

export interface Announcement {
	text: string;
	tone: 'info' | 'warning';
	updated_at: string;
}

export interface AuditEvent {
	id: number;
	at: string;
	actor_id: string;
	actor: string;
	action: string;
	target: string;
	target_label: string;
	details: Record<string, unknown> | null;
}

export interface AuditPage {
	events: AuditEvent[];
	next_before: number;
	since: string;
	actions: string[];
}

export interface AuditFilter {
	before?: number;
	action?: string;
	actor?: string;
}

export interface AccountStats {
	total: number;
	by_role: Record<string, number>;
	active_24h: number;
	active_7d: number;
	never_seen_since_sign_up: number;
	new_7d: number;
	pending: number;
	restrained: number;
}

export const ANNOUNCEMENT_MAX_CHARS = 280;

const call = (method: string, path: string, body?: unknown) => request(method, `/api/v1/admin${path}`, body, undefined, 'admin_failed');

export async function listUsers(): Promise<{ users: AdminUser[]; roles: string[] }> {
	return (await call('GET', '/users')).json();
}

export const setRole = (id: string, role: string) => call('POST', `/users/${id}/role`, { role });
export const restrain = (id: string) => call('POST', `/users/${id}/restrain`);
export const unrestrain = (id: string) => call('POST', `/users/${id}/unrestrain`);
export const approve = (id: string) => call('POST', `/users/${id}/approve`);
export const impersonate = (id: string) => call('POST', `/users/${id}/impersonate`);

// restored: the admin's own session is back; false sends them to sign in again.
export async function stopImpersonation(): Promise<boolean> {
	const r = (await (await call('POST', '/impersonation/stop')).json()) as { restored: boolean };
	return r.restored;
}

export async function setAnnouncement(text: string, tone: Announcement['tone']): Promise<Announcement> {
	return (await call('PUT', '/announcement', { text, tone })).json();
}

export const clearAnnouncement = () => call('DELETE', '/announcement');

export async function setFeature(name: string, audience: Audience): Promise<Feature> {
	return (await call('PUT', `/features/${name}`, { audience })).json();
}

export function auditQuery(f: AuditFilter): string {
	const q = new URLSearchParams();
	if (f.before) q.set('before', String(f.before));
	if (f.action) q.set('action', f.action);
	if (f.actor) q.set('actor', f.actor);
	const s = q.toString();
	return s ? `?${s}` : '';
}

export async function fetchAudit(f: AuditFilter): Promise<AuditPage> {
	return (await call('GET', `/audit${auditQuery(f)}`)).json();
}

export async function fetchStats(): Promise<AccountStats> {
	return (await call('GET', '/stats')).json();
}

export const whenShort = (iso: string) =>
	iso ? new Date(iso).toLocaleString('en-GB', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }) : '-';

export const timeOf = (iso: string) => new Date(iso).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });

// The local day of a time, as YYYY-MM-DD.
export function dayOf(iso: string): string {
	const d = new Date(iso);
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

export function dayTitle(key: string): string {
	const today = dayOf(new Date().toISOString());
	const yesterday = dayOf(new Date(Date.now() - 864e5).toISOString());
	if (key === today) return 'Today';
	if (key === yesterday) return 'Yesterday';
	return new Date(`${key}T12:00:00`).toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'long' });
}

// dayGroups splits events, latest first, into runs of the same local day.
export function dayGroups<T extends { at: string }>(events: T[]): { day: string; events: T[] }[] {
	const out: { day: string; events: T[] }[] = [];
	for (const e of events) {
		const day = dayOf(e.at);
		const last = out[out.length - 1];
		if (last && last.day === day) last.events.push(e);
		else out.push({ day, events: [e] });
	}
	return out;
}

export const accountLabels: Record<string, string> = {
	signed_in: 'Sign-in',
	signed_up: 'Sign-up',
	signed_out: 'Sign-out',
	role_changed: 'Role change',
	user_restrained: 'Access restricted',
	user_restored: 'Access restored',
	user_approved: 'Sign-up approved',
	impersonation_started: 'View as started',
	impersonation_stopped: 'View as stopped',
	announcement_set: 'Announcement posted',
	announcement_cleared: 'Announcement cleared',
	feature_set: 'Feature audience'
};

const text = (v: unknown) => (typeof v === 'string' ? v : v === undefined || v === null ? '' : String(v));

// describeAccount is the sentence of an account audit line, null for an action of the app's own domain.
export function describeAccount(e: AuditEvent): string | null {
	const who = e.actor;
	const target = e.target_label || e.target;
	const d = e.details ?? {};
	switch (e.action) {
		case 'signed_in':
			return `${who} signed in`;
		case 'signed_up':
			return d.access_state === 'pending' ? `${who} signed up and waits for approval` : `${who} signed up`;
		case 'signed_out':
			return `${who} signed out`;
		case 'role_changed':
			return `${who} changed the role of ${target} to ${text(d.to)}`;
		case 'user_restrained':
			return `${who} restricted the access of ${target}`;
		case 'user_restored':
			return `${who} restored the access of ${target}`;
		case 'user_approved':
			return `${who} approved ${target}${d.how === 'restore' ? ' with Restore access' : ''}`;
		case 'impersonation_started':
			return `${who} started viewing the app as ${target}`;
		case 'impersonation_stopped':
			return `${who} stopped viewing the app as ${target}`;
		case 'announcement_set':
			return `${who} posted the announcement (${text(d.tone)}, ${text(d.length)} characters)`;
		case 'announcement_cleared':
			return `${who} cleared the announcement`;
		case 'feature_set':
			return `${who} showed ${text(d.label) || target} to ${d.audience === 'admins' ? 'the admins only' : 'everyone'}`;
	}
	return null;
}
