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
			<button type="button" class="back" aria-label="Back to {back.label}" onclick={back.onclick}>
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
	{@render children?.()}
</header>

<style>
	.page-header {
		margin: 0 0 var(--space-3);
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
		border-bottom: var(--hairline) solid transparent;
		background: var(--background);
	}

	:global(.scrolled) .inline {
		border-bottom-color: var(--separator);
	}

	.inline .bar {
		display: grid;
		grid-template-columns: 44px minmax(0, 1fr) 44px;
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
		width: var(--control-height);
		height: var(--control-height);
		padding: 0;
		border: var(--hairline) solid var(--glass-edge);
		border-radius: 50%;
		background: var(--glass);
		box-shadow: var(--glass-shadow-sm);
		color: var(--foreground);
	}

	.back svg {
		fill: none;
		stroke: currentColor;
		stroke-width: 2.5;
		stroke-linecap: round;
		stroke-linejoin: round;
	}
</style>
