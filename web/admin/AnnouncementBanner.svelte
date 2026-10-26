<script lang="ts">
	import type { Announcement } from './admin';

	// Without onclose: the back office preview, which has no close button.
	let { announcement, onclose }: { announcement: Pick<Announcement, 'text' | 'tone'>; onclose?: () => void } = $props();
</script>

<div class="banner announcement" class:warning={announcement.tone === 'warning'} role="status">
	<p>{announcement.text}</p>
	{#if onclose}
		<button type="button" class="close" aria-label="Close announcement" onclick={onclose}>
			<svg viewBox="0 0 30 30" aria-hidden="true"><circle cx="15" cy="15" r="15" /><path d="M10.5 10.5l9 9m0-9-9 9" /></svg>
		</button>
	{/if}
</div>

<style>
	.announcement {
		flex-wrap: nowrap;
		align-items: flex-start;
	}

	.announcement p {
		flex: 1 1 0;
		white-space: pre-line;
	}

	.close {
		width: var(--control-height);
		margin: -11px calc(-1 * var(--space-3)) -11px 0;
		padding: 0;
		border-radius: 50%;
	}

	.close svg {
		width: 24px;
		height: 24px;
	}

	.close circle {
		fill: var(--fill);
	}

	.close path {
		fill: none;
		stroke: var(--muted-foreground);
		stroke-width: 2.4;
		stroke-linecap: round;
	}
</style>
