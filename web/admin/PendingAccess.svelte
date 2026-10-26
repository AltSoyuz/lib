<script lang="ts">
	import { errorText } from '../api';

	// The page of an account waiting for an admin: it can only check again or sign out.
	let { email, logo, onsignout }: { email: string; logo: string; onsignout: () => Promise<void> } = $props();

	let signingOut = $state(false);
	let error = $state<string | null>(null);

	async function signOut() {
		signingOut = true;
		error = null;
		try {
			await onsignout();
		} catch (e) {
			error = errorText(e);
			signingOut = false;
		}
	}
</script>

<section class="pending-access" aria-labelledby="pending-access-title">
	<img src={logo} alt="" width="72" height="72" />
	<h1 id="pending-access-title">Almost there</h1>
	<p>Your account is waiting for approval. An organizer will let you in soon.</p>
	<p class="muted">Signed in as <strong>{email}</strong></p>
	<button type="button" class="filled wide" onclick={() => location.reload()} disabled={signingOut}>Check again</button>
	<button type="button" class="wide" onclick={signOut} disabled={signingOut}>Sign out</button>
	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}
</section>

<style>
	.pending-access {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-3);
		max-width: 26rem;
		margin: var(--space-6) auto;
		text-align: center;
	}

	img {
		border-radius: var(--radius-lg);
	}

	h1 {
		margin-top: var(--space-2);
	}

	strong {
		color: var(--foreground);
		font-weight: 600;
		overflow-wrap: anywhere;
	}

	.filled {
		margin-top: var(--space-4);
	}
</style>
