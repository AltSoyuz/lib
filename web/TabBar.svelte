<script lang="ts">
	import type { Snippet } from 'svelte';
	import { page } from '$app/state';
	import { sidebar, toggleSidebar } from './sidebar.svelte';
	import { text } from './text';

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

	// Cmd or Ctrl and \ show and hide the sidebar, the View menu command of a Mac app; only where there is a sidebar.
	function onkeydown(e: KeyboardEvent) {
		if (e.key !== '\\' || !(e.metaKey || e.ctrlKey) || e.altKey || e.shiftKey) return;
		if (!matchMedia('(min-width: 1024px)').matches) return;
		e.preventDefault();
		toggleSidebar();
	}
</script>

<svelte:window {onkeydown} />

<nav class="tabbar" id="kit-sidebar" aria-label="Main">
	<div class="head">
		<span class="brand">
			<img src={brand.logo} alt="" width="28" height="28" />
			{brand.name}
		</span>
		<button
			type="button"
			class="toggle"
			aria-label={sidebar.collapsed ? text.showSidebar : text.hideSidebar}
			title={sidebar.collapsed ? text.showSidebar : text.hideSidebar}
			aria-expanded={!sidebar.collapsed}
			aria-controls="kit-sidebar"
			onclick={toggleSidebar}
		>
			<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="3.5" y="5" width="17" height="14" rx="3" /><path d="M9.5 5v14" /></svg>
		</button>
	</div>
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

	.head {
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
		/* A panel of glass floating 12px inside the window, as the phone's capsule does at the bottom: the rail is its width and that room on both sides. */
		.tabbar {
			/* Above the sticky title bar (21), whose opaque fill would cut the sidebar's shadow. */
			z-index: 30;
			top: var(--space-3);
			right: auto;
			bottom: var(--space-3);
			left: var(--space-3);
			flex-direction: column;
			justify-content: flex-start;
			gap: 2px;
			width: calc(var(--rail-width) - 2 * var(--space-3));
			max-width: none;
			height: auto;
			margin: 0;
			padding: var(--space-3) var(--space-2) var(--space-4);
			border: var(--hairline) solid var(--glass-edge);
			border-radius: var(--radius-lg);
			background: var(--glass);
			box-shadow: var(--glass-shadow);
			-webkit-backdrop-filter: var(--glass-blur);
			backdrop-filter: var(--glass-blur);
		}

		.head {
			display: flex;
			align-items: center;
			justify-content: space-between;
			margin: 0 0 var(--space-2);
		}

		.brand {
			display: flex;
			align-items: center;
			gap: var(--space-2);
			min-height: 48px;
			padding: 0 var(--space-2);
			font-size: var(--text-body);
			font-weight: 600;
		}

		.toggle {
			display: grid;
			place-items: center;
			flex: none;
			width: 32px;
			height: 32px;
			padding: 0;
			border-radius: var(--radius-pill);
			background: transparent;
			color: var(--muted-foreground);
		}

		.toggle svg {
			width: 18px;
			height: 18px;
			color: inherit;
			stroke-width: 1.7;
		}

		@media (hover: hover) {
			.toggle:hover {
				background: var(--fill);
				color: var(--foreground);
			}
		}

		/* Folded: a small floating panel that keeps the button, the sidebar's one way back. */
		:global(:root[data-sidebar='collapsed']) .tabbar {
			bottom: auto;
			padding: var(--space-1);
		}

		:global(:root[data-sidebar='collapsed']) .head {
			justify-content: center;
			margin-bottom: 0;
		}

		:global(:root[data-sidebar='collapsed']) .brand,
		:global(:root[data-sidebar='collapsed']) a {
			display: none;
		}
		.brand img {
			width: 28px;
			height: 28px;
			border-radius: var(--radius);
		}

		a {
			flex: none;
			flex-direction: row;
			justify-content: flex-start;
			gap: var(--space-2);
			max-width: none;
			min-height: calc(var(--control-height) + 4px);
			padding: 0 var(--space-3);
			border-radius: var(--radius-pill);
			color: var(--foreground);
			font-size: var(--text-body);
			font-weight: 400;
			line-height: var(--leading);
		}

		/* Sidebar icons take the accent color by default (HIG, Sidebars). */
		svg {
			width: 16px;
			height: 16px;
			color: var(--tint);
			stroke-width: 2;
		}

		/* The selected row, as in a SwiftUI sidebar: a rounded highlight in the accent color. */
		a[aria-current='page'] {
			background: var(--tint-soft);
			color: var(--tint);
			font-weight: 600;
		}

		[aria-current='page'] svg {
			fill: none;
			stroke-width: 2;
		}

		a:active {
			transform: none;
		}

		@media (prefers-reduced-motion: no-preference) {
			.tabbar {
				transition: width 0.2s ease;
			}
		}

		@media (hover: hover) {
			a:not([aria-current='page']):hover {
				background: var(--fill);
			}
		}
	}
</style>
