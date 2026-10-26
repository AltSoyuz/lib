<script lang="ts">
	import type { Snippet } from 'svelte';
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
		/* min(): no line where the page does not draw under a status bar. */
		border-bottom: min(var(--hairline), var(--safe-top)) solid transparent;
		background: var(--background);
	}

	.scrolled .status-bar {
		border-bottom-color: var(--separator);
	}
</style>
