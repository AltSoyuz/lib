import { browser } from '$app/environment';

const KEY = 'kit.sidebar';

function stored(): boolean {
	try {
		return localStorage.getItem(KEY) === 'collapsed';
	} catch {
		return false;
	}
}

// Whether the desktop sidebar is folded to its strip. The browser keeps the choice; <html data-sidebar> carries it to the CSS.
export const sidebar = $state({ collapsed: browser ? stored() : false });

function apply() {
	document.documentElement.dataset.sidebar = sidebar.collapsed ? 'collapsed' : 'expanded';
	try {
		localStorage.setItem(KEY, sidebar.collapsed ? 'collapsed' : 'expanded');
	} catch {
		// A private window keeps the choice for the page only.
	}
}

if (browser) apply();

export function toggleSidebar() {
	sidebar.collapsed = !sidebar.collapsed;
	apply();
}
