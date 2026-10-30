import { featureOn } from './features.svelte';

// What a tour says (Tour.svelte): the app writes it, the kit draws it.

// A row of the welcome page: one thing the app does, in a title and a few words.
export interface TourRow {
	icon: string;
	title: string;
	text: string;
	// The back office feature it shows (features.svelte.ts).
	feature?: string;
}

export interface TourWelcome {
	logo: string;
	title: string;
	rows: TourRow[];
	// One line at the bottom of the welcome.
	note?: string;
}

// A page after the welcome: one feature, drawn by the app's scene, a title, one sentence, and Try it to open it for real.
export interface TourStep {
	id: string;
	title: string;
	lead: string;
	try?: { label: string; href: string };
	// For the admins by their role: such steps go last, so a page number kept midway still points at its step once /me answers.
	admin?: boolean;
	feature?: string;
}

// shownInTour keeps what every account has: a feature kept to the admins stays out of the tour, for the admins too.
export function shownInTour<T extends { admin?: boolean; feature?: string }>(items: T[], admin: boolean): T[] {
	return items.filter((item) => (!item.admin || admin) && (!item.feature || featureOn(item.feature, null)));
}
