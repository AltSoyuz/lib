import { browser } from '$app/environment';
import { updated } from '$app/state';
import { keptShort } from './viewport';

interface BeforeInstallPromptEvent extends Event {
	prompt(): Promise<void>;
	userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

export const pwa = $state({
	online: true,
	reconnected: false,
	standalone: false,
	installed: false,
	installPrompt: null as BeforeInstallPromptEvent | null
});

// Chrome fires beforeinstallprompt once per page load, often before /install mounts.
if (browser) {
	pwa.online = navigator.onLine;
	pwa.standalone =
		window.matchMedia('(display-mode: standalone)').matches ||
		(navigator as Navigator & { standalone?: boolean }).standalone === true;
	window.addEventListener('online', () => {
		pwa.online = true;
		pwa.reconnected = true;
	});
	window.addEventListener('offline', () => (pwa.online = false));
	window.addEventListener('beforeinstallprompt', (e) => {
		e.preventDefault();
		pwa.installPrompt = e as BeforeInstallPromptEvent;
	});
	window.addEventListener('appinstalled', () => {
		pwa.installPrompt = null;
		pwa.installed = true;
	});
	// Only iOS sets navigator.standalone: on Android the system bars keep the window shorter than the screen, so every remeasure would replay the open sheets.
	if ((navigator as Navigator & { standalone?: boolean }).standalone === true) healViewportAfterKeyboard();
	// A home screen app comes back from the background without loading a page: nothing else checks for a new version.
	document.addEventListener('visibilitychange', () => {
		if (document.visibilityState === 'visible') void checkVersion();
	});
	window.addEventListener('online', () => void checkVersion());
}

// WebKit bug in home screen apps: once the keyboard closes, the viewport can stay short for the session, leaving the
// fixed bars too high over an empty band. Hiding and showing the full height app makes WebKit measure it again.
function healViewportAfterKeyboard() {
	// A native list open counts too: its picker shortens the viewport like a keyboard.
	const typing = () => {
		const f = document.activeElement;
		return f instanceof HTMLInputElement || f instanceof HTMLTextAreaElement || f instanceof HTMLSelectElement;
	};
	// At most one remeasure per half second: one that changes nothing must not answer its own resize forever.
	let healedAt = -Infinity;
	const heal = (why: string) => {
		const short = window.innerHeight;
		if (typing() || !keptShort({ width: window.innerWidth, height: short }, screen)) return;
		if (performance.now() - healedAt < 500) return;
		healedAt = performance.now();
		const app = document.querySelector<HTMLElement>('.app');
		if (!app) return;
		const y = window.scrollY;
		app.style.display = 'none';
		void app.offsetHeight;
		app.style.display = '';
		window.scrollTo(0, y);
		console.info(`viewport remeasured (${why}): ${short}px, screen ${screen.width}x${screen.height}, now ${window.innerHeight}px`);
	};
	// The keyboard takes a few hundred milliseconds to leave: measured once it has, and once more in case it lingered.
	document.addEventListener('focusout', () => {
		for (const ms of [300, 900]) setTimeout(() => heal('keyboard'), ms);
	});
	window.visualViewport?.addEventListener('resize', () => heal('resize'));
	document.addEventListener('visibilitychange', () => {
		if (document.visibilityState === 'visible') setTimeout(() => heal('resume'), 300);
	});
}

// Compares the running build with the server's /_app/version.json; the worker update readies the new files meanwhile.
async function checkVersion() {
	if (updated.current) return;
	if (await updated.check()) {
		console.info('a new version is on the server');
		const reg = await navigator.serviceWorker?.getRegistration();
		await reg?.update().catch((e) => console.warn('service worker update:', e));
	}
}

export async function promptInstall(): Promise<void> {
	const prompt = pwa.installPrompt;
	if (!prompt) return;
	pwa.installPrompt = null;
	await prompt.prompt();
}
