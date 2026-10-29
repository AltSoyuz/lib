<script lang="ts">
	import type { Snippet } from 'svelte';
	import { beforeNavigate } from '$app/navigation';
	import './pwa.svelte';

	// The frame of every page: .app is what pwa.svelte.ts remeasures after the keyboard; floating holds the tab bar and other fixed controls.
	let {
		tabbed = false,
		class: extra = '',
		children,
		floating
	}: {
		tabbed?: boolean;
		class?: string;
		children: Snippet;
		floating?: Snippet;
	} = $props();

	let scrollY = $state(0);

	// WebKit sends no focusout for a field leaving with its page, the cue pwa.svelte.ts remeasures a viewport the keyboard left short on.
	beforeNavigate(() => {
		const f = document.activeElement;
		if (f instanceof HTMLInputElement || f instanceof HTMLTextAreaElement || f instanceof HTMLSelectElement) f.blur();
	});
</script>

<svelte:window bind:scrollY />

<div class="app {extra}" class:tabbed class:scrolled={scrollY > 0}>
	<div class="status-bar" aria-hidden="true"></div>
	<main class="page">
		{@render children()}
	</main>
	{@render floating?.()}
</div>

<style>
	.tabbed {
		padding-left: var(--rail-width);
	}

	@media (display-mode: standalone) {
		.app {
			min-height: 100vh;
		}
	}

	/* The home screen app draws under the iPhone status bar: opaque, so no content shows through it. */
	.status-bar {
		position: fixed;
		z-index: 20;
		top: 0;
		right: 0;
		left: 0;
		height: var(--safe-top);
		background: var(--background);
	}

	/* Content scrolls under a soft edge, as in iOS 26, not under a line; min(): none where there is no status bar. */
	.status-bar::after {
		content: '';
		position: absolute;
		top: 100%;
		right: 0;
		left: 0;
		height: min(var(--space-5), var(--safe-top));
		background: inherit;
		-webkit-mask: linear-gradient(#000, transparent);
		mask: linear-gradient(#000, transparent);
		opacity: 0;
		transition: opacity 200ms ease;
		pointer-events: none;
	}

	.scrolled .status-bar::after {
		opacity: 1;
	}
</style>
