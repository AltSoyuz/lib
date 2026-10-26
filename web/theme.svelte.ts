import { browser } from '$app/environment';

export type ThemeChoice = 'system' | 'light' | 'dark';

const COLORS = { light: '#f2f2f7', dark: '#000000' };

export const theme = $state<{ choice: ThemeChoice }>({ choice: 'system' });

// The inline script of the app's app.html reads the same key before the first paint: keep them in step.
let key = '';
let forced: string | null = null;

const systemLight = browser ? matchMedia('(prefers-color-scheme: light)') : null;
systemLight?.addEventListener('change', () => {
	if (key && theme.choice === 'system') apply();
});

// Called once by the root layout, with the localStorage key of the app's choice.
export function initTheme(storageKey: string) {
	key = storageKey;
	if (!browser) return;
	theme.choice = read();
	apply();
}

function read(): ThemeChoice {
	try {
		const v = localStorage.getItem(key);
		return v === 'light' || v === 'dark' ? v : 'system';
	} catch (e) {
		console.warn('cannot read the theme of this phone:', e);
		return 'system';
	}
}

function apply() {
	const light = theme.choice === 'light' || (theme.choice === 'system' && !!systemLight?.matches);
	if (light) document.documentElement.dataset.theme = 'light';
	else delete document.documentElement.dataset.theme;
	document.querySelector('meta[name="theme-color"]')?.setAttribute('content', forced ?? (light ? COLORS.light : COLORS.dark));
}

// Safari tints its bars from theme-color: a full screen view of another color (a photo viewer) sets its own, null gives it back.
export function setThemeColor(color: string | null) {
	forced = color;
	apply();
}

// The choice stays on this phone: the system one is the absence of a choice.
export function setTheme(choice: ThemeChoice) {
	theme.choice = choice;
	try {
		if (choice === 'system') localStorage.removeItem(key);
		else localStorage.setItem(key, choice);
	} catch (e) {
		console.warn('cannot keep the theme on this phone:', e);
	}
	apply();
}
