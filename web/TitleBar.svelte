<script lang="ts">
	import type { Snippet } from 'svelte';
	import { updated } from '$app/state';
	import { pwa } from './pwa.svelte';

	// inline: a compact bar that stays at the top; otherwise a large title that scrolls with the page.
	// back: a chevron left of an inline title, named "Back to <label>". trailing: what sits right of the title (an account button).
	// banners: the app's own banners, above the offline and new version ones; children: controls under them, which stay at the top with an inline bar.
	let {
		title,
		subtitle,
		inline = false,
		back,
		trailing,
		banners,
		children
	}: {
		title: string;
		subtitle?: string;
		inline?: boolean;
		back?: { label: string; onclick: () => void };
		trailing?: Snippet;
		banners?: Snippet;
		children?: Snippet;
	} = $props();
</script>

<header class="page-header" class:inline>
	<div class="bar">
		{#if back}
			<button type="button" class="back glass round" aria-label="Back to {back.label}" onclick={back.onclick}>
				<svg viewBox="0 0 12 20" width="12" height="20" aria-hidden="true"><path d="M10 2 2 10l8 8" /></svg>
			</button>
		{/if}
		<h1>{title}</h1>
		{#if trailing}
			<div class="trail">{@render trailing()}</div>
		{/if}
	</div>
	{#if subtitle}
		<p class="subtitle">{subtitle}</p>
	{/if}
	{@render banners?.()}
	{#if !pwa.online}
		<p class="banner" role="status">You are offline. Nothing new will load until the connection is back.</p>
	{:else if updated.current}
		<div class="banner" role="status">
			<p>A new version is available.</p>
			<button type="button" class="tinted" onclick={() => location.reload()}>Reload</button>
		</div>
	{:else if pwa.reconnected}
		<div class="banner" role="status">
			<p>The connection is back.</p>
			<button type="button" class="tinted" onclick={() => location.reload()}>Reload</button>
		</div>
	{/if}
	{#if children}
		<div class="controls">{@render children()}</div>
	{/if}
</header>

<style>
	.page-header {
		margin: 0 0 var(--space-4);
	}

	.bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
		min-height: var(--control-height);
	}

	h1 {
		min-width: 0;
		overflow-wrap: anywhere;
	}

	.subtitle {
		margin: 2px 0 0;
		color: var(--muted-foreground);
	}

	.trail {
		display: flex;
		flex: none;
		align-items: center;
		gap: var(--space-2);
	}

	/* A main action (Save, Done) keeps the height of the controls beside it. */
	.trail :global(.filled) {
		min-height: var(--control-height);
		padding: 0 var(--space-4);
	}

	/* 19 points from the title's baseline to a search field, as in Shazam: a 44px button of the bar touches it otherwise. */
	.controls {
		margin-top: var(--space-2);
	}

	.controls:empty {
		display: none;
	}

	.page-header :global(.banner) {
		margin: var(--space-3) 0 0;
	}

	.page-header :global(.banner + :not(.banner)) {
		margin-top: var(--space-3);
	}

	/* Opaque over the status strip it replaces; its title starts at a large title's height, below the blur iOS draws under the status bar. */
	.inline {
		position: sticky;
		top: 0;
		z-index: 21;
		margin: calc(-1 * var(--page-top)) calc(-1 * var(--space-4)) var(--space-4);
		padding: var(--page-top) var(--space-4) var(--space-1);
		background: var(--background);
	}

	.inline::after {
		content: '';
		position: absolute;
		top: 100%;
		right: 0;
		left: 0;
		height: var(--space-5);
		background: inherit;
		-webkit-mask: linear-gradient(#000, transparent);
		mask: linear-gradient(#000, transparent);
		opacity: 0;
		transition: opacity 200ms ease;
		pointer-events: none;
	}

	:global(.scrolled) .inline::after {
		opacity: 1;
	}

	/* Equal sides, each at least as wide as its buttons: the title stays centered on the screen while there is room. */
	.inline .bar {
		display: grid;
		grid-template-columns: minmax(max-content, 1fr) minmax(0, auto) minmax(max-content, 1fr);
		min-height: 44px;
	}

	.inline h1 {
		grid-column: 2;
		font-size: var(--text-body);
		font-weight: 600;
		line-height: var(--leading);
		letter-spacing: 0;
		text-align: center;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.inline .trail {
		grid-column: 3;
		justify-self: end;
	}

	.back {
		grid-column: 1;
		justify-self: start;
	}

	.back svg {
		fill: none;
		stroke: currentColor;
		stroke-width: 2.5;
		stroke-linecap: round;
		stroke-linejoin: round;
	}
</style>
