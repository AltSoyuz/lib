<script lang="ts">
	import type { Snippet } from 'svelte';
	import Sheet from '../Sheet.svelte';
	import { errorText } from '../api';
	import { approve, impersonate, restrain, setRole, unrestrain, whenShort, type AdminUser } from './admin';

	// The accounts for an admin: pending ones first with Approve, then a sheet per account where a change applies at
	// once (role, access, View as). avatar, detail, sheet and footer add the app's own: a picture, facts in the row,
	// rows of the sheet, text after the counts. onviewas runs once View as started, to reload the app as the other account.
	let {
		users,
		roles,
		canImpersonate = false,
		onchanged,
		onviewas,
		avatar,
		detail,
		sheet,
		footer
	}: {
		users: AdminUser[];
		roles: string[];
		canImpersonate?: boolean;
		onchanged: () => Promise<void> | void;
		onviewas: () => void;
		avatar?: Snippet<[AdminUser, boolean]>;
		detail?: Snippet<[AdminUser]>;
		sheet?: Snippet<[AdminUser]>;
		footer?: Snippet;
	} = $props();

	const messages: Record<string, string> = {
		cannot_modify_last_admin: 'The app needs at least one admin.',
		super_admin_protected: 'This admin comes from the server settings: another admin cannot change them.',
		cannot_restrain_self: 'You cannot restrict your own access.',
		cannot_impersonate_admin: 'An admin cannot be viewed as.',
		user_restrained: 'Only an active account can be viewed as.',
		not_pending: 'This account is no longer waiting.'
	};

	let open = $state(false);
	let selectedId = $state<string | null>(null);
	let selected = $derived(users.find((u) => u.id === selectedId) ?? null);
	let busy = $state(false);
	let error = $state<string | null>(null);

	let connected = $derived(users.filter((u) => u.connected).length);
	let waiting = $derived(users.filter((u) => u.access_state === 'pending').length);

	async function run(fn: () => Promise<unknown>) {
		busy = true;
		error = null;
		try {
			await fn();
			await onchanged();
		} catch (e) {
			const code = (e as Error).message;
			error = messages[code] ?? errorText(e);
		} finally {
			busy = false;
		}
	}

	function openUser(u: AdminUser) {
		error = null;
		selectedId = u.id;
		open = true;
	}

	const connection = (u: AdminUser) =>
		!u.connected
			? u.last_seen_at
				? `seen ${whenShort(u.last_seen_at)}`
				: 'never seen'
			: u.active_session_count > 1
				? `connected, ${u.active_session_count} sessions`
				: 'connected';

	async function viewAs(u: AdminUser) {
		busy = true;
		error = null;
		try {
			await impersonate(u.id);
			onviewas();
		} catch (e) {
			error = messages[(e as Error).message] ?? errorText(e);
			busy = false;
		}
	}
</script>

{#snippet initial(u: AdminUser, large: boolean)}
	<span class="initial" class:large aria-hidden="true">{u.email.slice(0, 1).toUpperCase()}</span>
{/snippet}

<section class="section">
	<h2>Users</h2>
	{#if error && !open}
		<p class="banner error" role="alert">{error}</p>
	{/if}
	<ul class="list">
		{#each users as u (u.id)}
			{@const awaiting = u.access_state === 'pending'}
			<li class:with-action={awaiting}>
				<button type="button" class="cell" class:chevron={!awaiting} onclick={() => openUser(u)}>
					{@render (avatar ?? initial)(u, false)}
					<span class="content">
						<span class="title">{u.email}</span>
						<span class="subtitle meta">
							{#if awaiting}
								<span class="pending-tag">Pending</span>
								<span>signed up {whenShort(u.created_at)}</span>
							{:else}
								{@render detail?.(u)}
								<span>{u.role}</span>
								{#if u.access_state === 'restrained'}<span class="restricted">restricted</span>{/if}
								<span>{connection(u)}</span>
							{/if}
						</span>
					</span>
				</button>
				{#if awaiting}
					<button type="button" class="tinted" onclick={() => run(() => approve(u.id))} disabled={busy}>Approve</button>
				{/if}
			</li>
		{/each}
	</ul>
	<p class="footer">
		Users: {users.length}. Connected right now: {connected}.
		{#if waiting}Waiting for approval: {waiting}.{/if}
		{@render footer?.()}
	</p>
</section>

<Sheet title="User" bind:open onclosed={() => (selectedId = null)}>
	{#if selected}
		{@const u = selected}
		{@const restrained = u.access_state === 'restrained'}
		{@const awaiting = u.access_state === 'pending'}
		<section class="section">
			<ul class="list">
				<li class="cell">
					{@render (avatar ?? initial)(u, true)}
					<span class="content"><span class="title">{u.email}</span></span>
				</li>
				<li class="cell">
					<span class="content">Access</span>
					<span class="value" class:restricted={restrained}>{awaiting ? 'waiting for approval' : restrained ? 'restricted' : 'active'}</span>
				</li>
				<li class="cell"><span class="content">Signed up</span><span class="value">{whenShort(u.created_at)}</span></li>
				<li class="cell"><span class="content">Connection</span><span class="value">{connection(u)}</span></li>
			</ul>
		</section>
		<section class="section">
			<ul class="list">
				<li>
					<label class="cell picker">
						<span class="content">Role</span>
						<span class="value">{u.role}</span>
						<select
							aria-label="Role for {u.email}"
							value={u.role}
							disabled={busy}
							onchange={(e) => {
								const role = e.currentTarget.value;
								run(() => setRole(u.id, role));
							}}
						>
							{#each roles as r (r)}
								<option value={r}>{r}</option>
							{/each}
						</select>
					</label>
				</li>
				{@render sheet?.(u)}
			</ul>
			{#if error}
				<p class="footer error" role="alert">{error}</p>
			{/if}
		</section>
		<section class="section">
			<ul class="list">
				{#if awaiting}
					<li><button type="button" class="cell tint" onclick={() => run(() => approve(u.id))} disabled={busy}>Approve</button></li>
				{/if}
				{#if canImpersonate && u.role !== 'admin' && u.access_state === 'active'}
					<li><button type="button" class="cell tint" onclick={() => viewAs(u)} disabled={busy}>View as</button></li>
				{/if}
				<li>
					<button
						type="button"
						class="cell"
						class:destructive={!restrained}
						class:tint={restrained}
						disabled={busy}
						onclick={() => {
							if (!restrained && !confirm(`Restrict the access of ${u.email}? Their sessions end now.`)) return;
							run(() => (restrained ? unrestrain(u.id) : restrain(u.id)));
						}}
					>
						{restrained ? 'Restore access' : 'Restrict access'}
					</button>
				</li>
			</ul>
		</section>
	{/if}
</Sheet>

<style>
	.meta {
		display: flex;
		flex-wrap: wrap;
		column-gap: var(--space-1);
		font-variant-numeric: tabular-nums;
	}

	.meta :global(span) {
		white-space: nowrap;
	}

	.meta > :global(span:not(:last-child))::after {
		content: ' ·';
	}

	.pending-tag {
		color: var(--tint);
		font-weight: 600;
	}

	.restricted {
		color: var(--destructive);
	}

	.initial {
		display: inline-flex;
		flex: none;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 50%;
		background: linear-gradient(#a5abb8, #858994);
		color: #fff;
		font-size: 15px;
		font-weight: 600;
	}

	.initial.large {
		width: 44px;
		height: 44px;
		font-size: var(--text-title3);
	}
</style>
