<script lang="ts">
	import { onMount, type Snippet } from 'svelte';
	import type { TourStep, TourWelcome } from './tour';

	// A first-run tour as an iOS onboarding, over the whole screen: the welcome, then one swiped page per step with the app's scene of its
	// screen. Page 0 is the welcome, page i + 1 the step i; the app opens it, keeps the page it is at (onpage) and leaves it (ontry, onfinish).
	let {
		welcome,
		steps,
		start = 0,
		scene,
		icon,
		onpage,
		ontry,
		onfinish
	}: {
		welcome: TourWelcome;
		steps: TourStep[];
		start?: number;
		scene: Snippet<[TourStep]>;
		icon: Snippet<[string]>;
		onpage?: (page: number) => void;
		ontry: (step: TourStep, page: number) => void;
		onfinish: () => void;
	} = $props();

	const count = $derived(steps.length + 1);
	let index = $state(0);
	const at = $derived(Math.min(index, count - 1));
	const last = $derived(at === count - 1);
	let pages = $state<HTMLElement>();

	onMount(() => {
		const first = Math.min(Math.max(start, 0), count - 1);
		show(first);
		pages?.scrollTo({ left: first * pages.clientWidth, behavior: 'instant' });
	});

	function show(next: number) {
		index = next;
		onpage?.(next);
	}

	// A swipe moves the page: the one most in view is the current one.
	function scrolled() {
		if (!pages) return;
		const next = Math.round(pages.scrollLeft / pages.clientWidth);
		if (next !== index && next >= 0 && next < count) show(next);
	}

	function go(next: number) {
		const still = matchMedia('(prefers-reduced-motion: reduce)').matches;
		pages?.scrollTo({ left: next * pages.clientWidth, behavior: still ? 'instant' : 'smooth' });
	}
</script>

<div class="tour">
	<header>
		{#if !last}<button type="button" class="glass" onclick={onfinish}>Skip</button>{/if}
	</header>

	<div class="pages" bind:this={pages} onscroll={scrolled}>
		<section class="page" aria-labelledby="tour-welcome" inert={at !== 0}>
			<img class="logo" src={welcome.logo} alt="" width="88" height="88" />
			<h1 id="tour-welcome" class="welcome">{welcome.title}</h1>
			<ul class="rows">
				{#each welcome.rows as row (row.title)}
					<li>
						<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">{@render icon(row.icon)}</svg>
						<span class="text"><strong>{row.title}</strong>{row.text}</span>
					</li>
				{/each}
			</ul>
			{#if welcome.note}<p class="note">{welcome.note}</p>{/if}
		</section>
		{#each steps as step, i (step.id)}
			<section class="page" aria-labelledby="tour-{step.id}" inert={at !== i + 1}>
				<div class="hero" inert>{@render scene(step)}</div>
				<h1 id="tour-{step.id}">{step.title}</h1>
				<p class="lead">{step.lead}</p>
				{#if step.try}
					<button type="button" class="try" onclick={() => ontry(step, i + 1)}
						>{step.try.label}<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 18 6-6-6-6" /></svg></button
					>
				{/if}
			</section>
		{/each}
	</div>

	<footer>
		<div class="dots">
			{#each { length: count }, i (i)}
				<button type="button" aria-label="Page {i + 1} of {count}" aria-current={i === at} onclick={() => go(i)}><span></span></button>
			{/each}
		</div>
		<button type="button" class="filled wide" onclick={() => (last ? onfinish() : go(at + 1))}>{last ? 'Get started' : 'Continue'}</button>
	</footer>
</div>

<style>
	/* Fixed over the page frame: the tour owns the whole screen, and the app's layout shows no tab bar under it. */
	.tour {
		position: fixed;
		inset: 0;
		display: flex;
		flex-direction: column;
		max-width: 480px;
		margin: 0 auto;
	}

	header {
		display: flex;
		flex: none;
		justify-content: flex-end;
		min-height: var(--control-height);
		padding: calc(var(--safe-top) + var(--space-2)) var(--space-4) 0;
	}

	header button {
		min-height: var(--control-height);
		font-weight: 600;
	}

	/* One page per step, side by side: a swipe moves one page, as in an iOS onboarding. */
	.pages {
		display: flex;
		flex: 1;
		min-height: 0;
		overflow-x: auto;
		overscroll-behavior-x: contain;
		scroll-snap-type: x mandatory;
		scrollbar-width: none;
	}

	.pages::-webkit-scrollbar {
		display: none;
	}

	.page {
		display: flex;
		flex: 0 0 100%;
		flex-direction: column;
		align-items: center;
		padding: var(--space-2) var(--space-5) 0;
		overflow-y: auto;
		scroll-snap-align: center;
		scroll-snap-stop: always;
		text-align: center;
	}

	h1 {
		margin: var(--space-5) 0 var(--space-2);
		font-size: calc(28rem / 17);
		font-weight: 700;
		line-height: var(--leading-tight);
		letter-spacing: -0.01em;
	}

	.lead {
		max-width: 32ch;
		margin: 0;
		color: var(--muted-foreground);
		font-size: var(--text-body);
		line-height: 1.4;
	}

	/* The welcome: the logo, then what the app does, one row each, as Apple's welcome sheets. */
	.logo {
		width: 72px;
		height: 72px;
		margin-top: var(--space-4);
		border-radius: 22%;
		box-shadow: 0 10px 28px rgb(0 0 0 / 18%);
	}

	h1.welcome {
		margin-top: var(--space-4);
		font-size: calc(30rem / 17);
	}

	.rows {
		display: grid;
		gap: var(--space-4);
		width: 100%;
		margin: var(--space-5) 0 0;
		padding: 0;
		list-style: none;
		text-align: left;
	}

	.rows li {
		display: flex;
		align-items: center;
		gap: var(--space-4);
	}

	.rows .icon {
		flex: none;
		width: 44px;
		height: 30px;
		color: var(--tint);
	}

	svg {
		fill: none;
		stroke: currentColor;
		stroke-width: 2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}

	.rows .text {
		display: flex;
		flex-direction: column;
		gap: 2px;
		color: var(--muted-foreground);
		font-size: var(--text-subhead);
		line-height: var(--leading);
	}

	.rows strong {
		color: var(--foreground);
		font-size: var(--text-body);
		font-weight: 600;
	}

	.note {
		margin: auto 0 var(--space-2);
		padding-top: var(--space-4);
		color: var(--muted-foreground);
		font-size: var(--text-footnote);
	}

	footer {
		display: grid;
		flex: none;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-5) max(var(--space-5), var(--safe-bottom));
	}

	.dots {
		display: flex;
		justify-content: center;
	}

	/* A dot is small, its button as tall as a control so a finger finds it. */
	.dots button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 22px;
		height: 24px;
		padding: 0;
	}

	.dots span {
		width: 8px;
		height: 8px;
		border-radius: var(--radius-pill);
		background: var(--fill);
		transition: width 0.25s ease;
	}

	.dots [aria-current='true'] span {
		width: 20px;
		background: var(--foreground);
	}

	.try {
		display: inline-flex;
		flex: none;
		align-items: center;
		gap: 2px;
		min-height: var(--control-height);
		margin-top: var(--space-1);
		color: var(--tint);
		font-weight: 600;
	}

	.try svg {
		width: 18px;
		height: 18px;
	}

	/* The stage of a scene: the app draws its feature's screen on .card, .label and .pill, with made-up people. */
	.hero {
		position: relative;
		display: grid;
		flex: 1 1 0;
		place-items: center;
		width: calc(100% + 2 * var(--space-1));
		min-height: 220px;
		max-height: 340px;
		border-radius: var(--radius-xl);
		background: radial-gradient(90% 70% at 50% 0%, var(--tint-soft), transparent), linear-gradient(var(--tint-soft), transparent 80%);
		overflow: hidden;
		text-align: left;
		user-select: none;
	}

	.hero :global(.card) {
		width: min(82%, 300px);
		padding: var(--space-4);
		border-radius: var(--radius-lg);
		background: var(--card);
		box-shadow: 0 12px 32px rgb(0 0 0 / 14%);
	}

	.hero :global(.label) {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		margin: 0 0 var(--space-2);
		color: var(--muted-foreground);
		font-size: var(--text-footnote);
		font-weight: 600;
	}

	.hero :global(.pill) {
		position: absolute;
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		border: var(--hairline) solid var(--glass-edge);
		border-radius: var(--radius-pill);
		background: var(--glass);
		box-shadow: var(--glass-shadow-sm);
		font-size: var(--text-footnote);
		font-weight: 600;
		-webkit-backdrop-filter: var(--glass-blur);
		backdrop-filter: var(--glass-blur);
	}

	/* A small phone in Safari: the welcome keeps its rows and drops the logo. */
	@media (max-height: 600px) {
		.logo {
			display: none;
		}

		h1.welcome {
			margin-top: 0;
		}

		.rows {
			gap: var(--space-3);
		}
	}
</style>
