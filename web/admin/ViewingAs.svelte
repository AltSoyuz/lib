<script lang="ts">
	import { errorText } from '../api';
	import { text } from '../text';
	import { stopImpersonation } from './admin';

	// The banner of a View as session; onstopped gets whether the admin's own session is back.
	let { email, onstopped }: { email: string; onstopped: (restored: boolean) => void } = $props();

	let stopping = $state(false);
	let error = $state<string | null>(null);

	async function stop() {
		stopping = true;
		error = null;
		try {
			onstopped(await stopImpersonation());
		} catch (e) {
			if ((e as Error).message === 'unauthorized') {
				onstopped(false);
				return;
			}
			error = errorText(e);
			stopping = false;
		}
	}
</script>

<div class="banner tint" role="status">
	<p>{text.viewingAs} <strong>{email}</strong> {text.readOnly}</p>
	<button type="button" class="tinted" onclick={stop} disabled={stopping}>{text.stop}</button>
	{#if error}
		<p class="error">{error}</p>
	{/if}
</div>
