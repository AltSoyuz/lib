<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { AccountStats } from './admin';

	// The account counts; children adds the app's own rows to the list.
	let { stats, children }: { stats: AccountStats; children?: Snippet } = $props();

	// "role: n", not a plural: a role name is the app's own word ("2 staffs" would be wrong).
	let roles = $derived(
		Object.entries(stats.by_role)
			.map(([role, n]) => `${role}: ${n}`)
			.join(', ')
	);
</script>

<section class="section">
	<h2>Users</h2>
	<ul class="list">
		<li class="cell">
			<span class="content"><span class="title">Total</span><span class="subtitle">{roles}</span></span>
			<span class="value">{stats.total}</span>
		</li>
		<li class="cell"><span class="content">Active in the last 24 hours</span><span class="value">{stats.active_24h}</span></li>
		<li class="cell"><span class="content">Active in the last 7 days</span><span class="value">{stats.active_7d}</span></li>
		<li class="cell"><span class="content">Never came back after signing up</span><span class="value">{stats.never_seen_since_sign_up}</span></li>
		<li class="cell"><span class="content">New in the last 7 days</span><span class="value">{stats.new_7d}</span></li>
		{#if stats.pending}
			<li class="cell"><span class="content">Waiting for approval</span><span class="value attention">{stats.pending}</span></li>
		{/if}
		<li class="cell"><span class="content">Restricted</span><span class="value">{stats.restrained}</span></li>
		{@render children?.()}
	</ul>
	<p class="footer">Active: the app was opened, to the nearest 10 minutes; View as does not count.</p>
</section>

<style>
	.attention {
		color: var(--tint);
		font-weight: 600;
	}
</style>
