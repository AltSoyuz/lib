<script lang="ts">
	import type { Snippet } from 'svelte';
	import { page } from '$app/state';

	// A capsule floating over the bottom of the page on a phone, a sidebar from 1024px; icon renders the paths of a tab's 24x24 icon.
	let {
		tabs,
		brand,
		icon
	}: {
		tabs: { href: string; label: string; icon: string }[];
		brand: { name: string; logo: string };
		icon: Snippet<[string]>;
	} = $props();

	const current = (href: string) =>
		href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(href);
</script>

<nav class="tabbar" aria-label="Main">
	<span class="brand">
		<img src={brand.logo} alt="" width="28" height="28" />
		{brand.name}
	</span>
	{#each tabs as t (t.href)}
		<a href={t.href} aria-current={current(t.href) ? 'page' : undefined}>
			<svg viewBox="0 0 24 24" aria-hidden="true">
				{@render icon(t.icon)}
			</svg>
			<span>{t.label}</span>
		</a>
	{/each}
</nav>

<style>
	.tabbar {
		position: fixed;
		z-index: 10;
		right: max(var(--space-4), var(--safe-right));
		bottom: var(--tabbar-inset);
		left: max(var(--space-4), var(--safe-left));
		display: flex;
		justify-content: center;
		gap: 2px;
		max-width: 480px;
		height: var(--tabbar-capsule);
		margin: 0 auto;
		padding: var(--space-1);
		border: var(--hairline) solid var(--glass-edge);
		border-radius: var(--radius-pill);
		background: var(--glass);
		box-shadow: var(--glass-shadow);
		-webkit-backdrop-filter: var(--glass-blur);
		backdrop-filter: var(--glass-blur);
	}

	.brand {
		display: none;
	}

	a {
		display: flex;
		flex: 1 1 0;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 1px;
		/* The pill's round corners cut about 8px into each side at the label's baseline: a tab never gets narrower than its label plus that. */
		padding: 0 8px;
		border-radius: var(--radius-pill);
		color: var(--foreground);
		font-size: 10px;
		font-weight: 500;
		line-height: 12px;
		white-space: nowrap;
		text-decoration: none;
		touch-action: manipulation;
		transition:
			background-color 0.2s,
			transform 0.15s;
	}

	a[aria-current='page'] {
		background: var(--fill);
		color: var(--tint);
	}

	a:active {
		transform: scale(0.94);
	}

	a:focus-visible {
		outline-offset: -2px;
	}

	svg {
		width: 26px;
		height: 26px;
		fill: none;
		stroke: currentColor;
		stroke-width: 1.6;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	[aria-current='page'] svg {
		fill: var(--tint-soft);
		stroke-width: 1.9;
	}

	@media (min-width: 1024px) {
		.tabbar {
			top: 0;
			right: auto;
			bottom: 0;
			left: 0;
			flex-direction: column;
			justify-content: flex-start;
			gap: 2px;
			width: var(--rail-width);
			max-width: none;
			height: auto;
			margin: 0;
			padding: calc(var(--safe-top) + var(--space-5)) var(--space-3) var(--space-5);
			border: 0;
			border-right: var(--hairline) solid var(--separator);
			border-radius: 0;
			background: var(--card);
			box-shadow: none;
			-webkit-backdrop-filter: none;
			backdrop-filter: none;
		}

		.brand {
			display: flex;
			align-items: center;
			gap: var(--space-2);
			margin: 0 var(--space-3) var(--space-4);
			font-size: var(--text-title3);
			font-weight: 700;
		}

		.brand img {
			border-radius: var(--radius-sm);
		}

		a {
			flex: none;
			flex-direction: row;
			justify-content: flex-start;
			gap: var(--space-3);
			max-width: none;
			min-height: var(--control-height);
			padding: 0 var(--space-3);
			border-radius: var(--radius);
			color: var(--foreground);
			font-size: var(--text-body);
			font-weight: 400;
			line-height: var(--leading);
		}

		svg {
			width: 24px;
			height: 24px;
			color: var(--tint);
		}

		a[aria-current='page'] {
			background: var(--primary);
			color: var(--primary-foreground);
			font-weight: 600;
		}

		[aria-current='page'] svg {
			color: var(--primary-foreground);
			fill: none;
		}

		@media (hover: hover) {
			a:not([aria-current='page']):hover {
				background: var(--card-pressed);
			}
		}
	}
</style>
