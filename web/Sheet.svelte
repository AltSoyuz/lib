<script lang="ts">
	import type { Snippet } from 'svelte';

	// With onsave: Cancel and Save in the header, the content is the form; Cancel reads Close while nothing would be discarded, as some changes apply at once. Without: a read only sheet with a close button. Both close with a pull down.
	let {
		open = $bindable(false),
		title,
		dirty = false,
		busy = false,
		canSave = true,
		saveLabel = 'Save',
		onsave,
		onclosed,
		children
	}: {
		open: boolean;
		title: string;
		dirty?: boolean;
		busy?: boolean;
		canSave?: boolean;
		saveLabel?: string;
		onsave?: () => void;
		onclosed?: () => void;
		children: Snippet;
	} = $props();

	const id = $props.id();
	let dialog = $state<HTMLDialogElement>();
	let closing = $state(false);

	$effect(() => {
		if (!dialog) return;
		if (open && !dialog.open) {
			dialog.showModal();
			// showModal focuses the first button, which Safari rings as if by keyboard: the sheet takes the focus instead.
			dialog.focus({ preventScroll: true });
		}
		if (open) closing = false;
		else if (dialog.open) closing = true;
	});

	function requestClose() {
		if (busy) return;
		if (dirty && !confirm('Discard your changes?')) return;
		open = false;
	}

	function finish(e: AnimationEvent) {
		if (!closing || e.target !== dialog) return;
		closing = false;
		pulled = 0;
		dialog?.close();
		onclosed?.();
	}

	let start: { x: number; y: number; t: number } | null = null;
	let pulling = $state(false);
	let pulled = $state(0);

	// No pull from inside a scroll that is not at its top, nor from an element that handles its own touches (slider, map, photo framing).
	function touchStart(e: TouchEvent) {
		start = null;
		if (busy || e.touches.length !== 1) return;
		for (let el = e.target as Element | null; el && el !== dialog; el = el.parentElement) {
			if (el.scrollTop > 0 || el.matches('input[type=range]') || !['auto', 'manipulation'].includes(getComputedStyle(el).touchAction)) return;
		}
		start = { x: e.touches[0].clientX, y: e.touches[0].clientY, t: e.timeStamp };
	}

	// The browser scrolls from the first touchmove left uncancelled: the pull is decided on that one.
	function touchMove(e: TouchEvent) {
		if (!start) return;
		const dy = e.touches[0].clientY - start.y;
		if (!pulling) {
			if (dy <= 0 || Math.abs(e.touches[0].clientX - start.x) > dy) {
				start = null;
				return;
			}
			pulling = true;
		}
		e.preventDefault();
		pulled = Math.max(0, dy);
	}

	function touchEnd(e: TouchEvent) {
		if (!pulling || !start || !dialog) return;
		const far = pulled > dialog.offsetHeight / 4 || (pulled > 40 && pulled / (e.timeStamp - start.t) > 0.5);
		start = null;
		pulling = false;
		if (far) requestClose();
		if (open) pulled = 0;
	}

	// Svelte attaches touchmove as passive, which could not stop the scroll.
	$effect(() => {
		const d = dialog;
		if (!d) return;
		d.addEventListener('touchmove', touchMove, { passive: false });
		return () => d.removeEventListener('touchmove', touchMove);
	});
</script>

<dialog
	bind:this={dialog}
	class="sheet"
	tabindex="-1"
	class:closing
	class:pulling
	style:transform={pulled ? `translateY(${pulled}px)` : undefined}
	aria-labelledby="{id}-title"
	oncancel={(e) => {
		e.preventDefault();
		requestClose();
	}}
	onclick={(e) => {
		if (e.target === dialog) requestClose();
	}}
	onanimationend={finish}
	ontouchstart={touchStart}
	ontouchend={touchEnd}
	ontouchcancel={() => {
		start = null;
		pulling = false;
		pulled = 0;
	}}
>
	<form
		onsubmit={(e) => {
			e.preventDefault();
			if (onsave && canSave && !busy) onsave();
		}}
	>
		<div class="grabber" aria-hidden="true"></div>
		<header>
			<div class="lead">
				{#if onsave}
					<button type="button" class="glass" onclick={requestClose} disabled={busy}>{dirty ? 'Cancel' : 'Close'}</button>
				{/if}
			</div>
			<h2 id="{id}-title">{title}</h2>
			<div class="trail">
				{#if onsave}
					<button type="submit" class="save filled" disabled={!canSave || busy}>{busy ? 'Saving…' : saveLabel}</button>
				{:else}
					<button type="button" class="close glass round" aria-label="Close" onclick={requestClose}>
						<svg viewBox="0 0 30 30" aria-hidden="true"><path d="M10.5 10.5l9 9m0-9-9 9" /></svg>
					</button>
				{/if}
			</div>
		</header>
		<div class="body">
			{@render children()}
		</div>
	</form>
</dialog>

<style>
	dialog:focus {
		outline: none;
	}

	dialog {
		--card: var(--sheet-card);
		--card-pressed: var(--sheet-card-pressed);
		position: fixed;
		inset: auto 0 0;
		width: 100%;
		max-width: none;
		/* Its top stays below the large title line, out of the blur iOS draws under the status bar. */
		height: calc(100% - var(--page-top) - var(--space-4));
		max-height: none;
		margin: 0;
		padding: 0;
		border: 0;
		border-radius: var(--radius-xl) var(--radius-xl) 0 0;
		background: var(--sheet);
		color: var(--foreground);
		overflow: hidden;
		animation: sheet-in 340ms cubic-bezier(0.32, 0.72, 0, 1);
		transition: transform 300ms cubic-bezier(0.32, 0.72, 0, 1);
	}

	dialog.pulling {
		transition: none;
	}

	dialog.closing {
		animation: sheet-out 220ms ease-in forwards;
	}

	dialog::backdrop {
		top: var(--safe-top);
		background: rgb(0 0 0 / 55%);
		animation: fade-in 340ms ease;
	}

	@keyframes sheet-in {
		from {
			transform: translateY(100%);
		}
	}

	@keyframes sheet-out {
		to {
			transform: translateY(100%);
		}
	}

	@keyframes fade-in {
		from {
			opacity: 0;
		}
	}

	form {
		display: flex;
		flex-direction: column;
		height: 100%;
	}

	.grabber {
		position: absolute;
		top: 5px;
		left: 50%;
		width: 36px;
		height: 5px;
		border-radius: 3px;
		background: var(--faint);
		transform: translateX(-50%);
	}

	header {
		display: grid;
		grid-template-columns: minmax(max-content, 1fr) minmax(0, auto) minmax(max-content, 1fr);
		align-items: center;
		gap: var(--space-2);
		min-height: 56px;
		/* 12px from the sheet's 34px corners: concentric with the 22px of a 44px capsule. */
		padding: var(--space-3) var(--space-3) 0;
	}

	.trail {
		justify-self: end;
	}

	h2 {
		margin: 0;
		color: var(--foreground);
		font-size: var(--text-body);
		font-weight: 600;
		text-align: center;
		text-transform: none;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.save {
		min-height: var(--control-height);
		padding: 0 var(--space-4);
	}

	.close svg {
		width: 30px;
		height: 30px;
	}

	.close path {
		fill: none;
		stroke: currentColor;
		stroke-width: 2.2;
		stroke-linecap: round;
	}

	.body {
		flex: 1;
		padding: var(--space-3) var(--space-4) calc(var(--safe-bottom) + var(--space-6));
		overflow-y: auto;
		overscroll-behavior: contain;
	}

	@media (min-width: 700px) {
		dialog {
			inset: 0;
			width: min(560px, 100% - 64px);
			height: min(760px, 100% - 64px);
			margin: auto;
			border-radius: var(--radius-xl);
		}

		.grabber {
			display: none;
		}
	}

	/* The grid shows the title first without moving it in the DOM, so Cancel and Save keep their focus order. */
	@media (min-width: 1024px) {
		dialog {
			width: min(512px, 100% - 64px);
			height: fit-content;
			max-height: min(760px, 100% - 64px);
			border: 1px solid var(--separator);
			border-radius: var(--radius-xl);
			box-shadow:
				0 10px 15px -3px rgb(0 0 0 / 10%),
				0 4px 6px -4px rgb(0 0 0 / 10%);
			animation: dialog-in 150ms ease-out;
		}

		dialog[open] {
			display: flex;
			flex-direction: column;
		}

		dialog.closing {
			animation: dialog-out 100ms ease-in forwards;
		}

		dialog::backdrop {
			top: 0;
			background: rgb(0 0 0 / 50%);
		}

		form {
			flex: 1;
			min-height: 0;
			height: auto;
		}

		header {
			grid-template-columns: minmax(0, 1fr) auto auto;
			min-height: 0;
			padding: var(--space-5) var(--space-5) 0;
		}

		h2 {
			grid-row: 1;
			grid-column: 1;
			font-size: var(--text-title2);
			text-align: left;
		}

		.lead {
			grid-row: 1;
			grid-column: 2;
		}

		.trail {
			grid-column: 3;
		}

		.close {
			margin: calc(-1 * var(--space-2)) calc(-1 * var(--space-2)) 0 0;
			border-color: transparent;
			background: transparent;
			box-shadow: none;
		}

		.close svg {
			width: 24px;
			height: 24px;
		}

		.body {
			padding: var(--space-4) var(--space-5) var(--space-5);
		}

		.body > :global(:last-child) {
			margin-bottom: 0;
		}
	}

	@keyframes dialog-in {
		from {
			opacity: 0;
			transform: scale(0.96);
		}
	}

	@keyframes dialog-out {
		to {
			opacity: 0;
			transform: scale(0.96);
		}
	}
</style>
