<script lang="ts">
	import { onMount } from 'svelte';
	import { errorText } from '../api';
	import { accountLabels, dayGroups, dayTitle, describeAccount, timeOf, whenShort, type AuditEvent, type AuditFilter, type AuditPage } from './admin';

	// The audit log, filtered by action and person, a day per section. load fetches a page (the kit's fetchAudit, or
	// the app's own report); labels and describe name the app's own actions, the account ones are named here.
	let {
		load,
		people,
		labels = {},
		describe
	}: {
		load: (f: AuditFilter) => Promise<AuditPage>;
		people: { id: string; label: string }[];
		labels?: Record<string, string>;
		describe?: (e: AuditEvent) => string | null;
	} = $props();

	let action = $state('');
	let actor = $state('');
	let page = $state<AuditPage | null>(null);
	let events = $state<AuditEvent[]>([]);
	let busy = $state(false);
	let error = $state<string | null>(null);

	let groups = $derived(dayGroups(events));
	const label = (code: string) => labels[code] ?? accountLabels[code] ?? code;
	const sentence = (e: AuditEvent) => describe?.(e) ?? describeAccount(e) ?? `${e.actor}: ${label(e.action)}${e.target_label ? ` (${e.target_label})` : ''}`;

	async function fetchPage(more: boolean) {
		busy = true;
		error = null;
		try {
			const p = await load({ action, actor, before: more ? page?.next_before : undefined });
			events = more ? [...events, ...p.events] : p.events;
			page = p;
		} catch (e) {
			error = errorText(e);
		} finally {
			busy = false;
		}
	}

	onMount(() => void fetchPage(false));
</script>

<section class="section">
	<h2>Audit log</h2>
	<ul class="list">
		<li>
			<label class="cell picker">
				<span class="content">Action</span>
				<span class="value">{action ? label(action) : 'All'}</span>
				<select aria-label="Filter the audit log by action" bind:value={action} onchange={() => fetchPage(false)} disabled={busy}>
					<option value="">All</option>
					{#each page?.actions ?? [] as code (code)}
						<option value={code}>{label(code)}</option>
					{/each}
				</select>
			</label>
		</li>
		<li>
			<label class="cell picker">
				<span class="content">Person</span>
				<span class="value">{actor ? (people.find((p) => p.id === actor)?.label ?? actor) : 'Everyone'}</span>
				<select aria-label="Filter the audit log by person" bind:value={actor} onchange={() => fetchPage(false)} disabled={busy}>
					<option value="">Everyone</option>
					{#each people as p (p.id)}
						<option value={p.id}>{p.label}</option>
					{/each}
				</select>
			</label>
		</li>
	</ul>
	{#if error}
		<p class="footer error" role="alert">{error}</p>
	{:else if page && !events.length}
		<p class="footer">Nothing recorded{action || actor ? ' for this filter' : ''}.</p>
	{:else if page?.since}
		<p class="footer">Recorded since {whenShort(page.since)}.</p>
	{/if}
</section>

{#each groups as g (g.day)}
	<section class="section">
		<h2>{dayTitle(g.day)}</h2>
		<ul class="list">
			{#each g.events as e (e.id)}
				<li class="cell">
					<span class="content">
						<span class="title">{sentence(e)}</span>
						<span class="subtitle">{timeOf(e.at)}</span>
					</span>
				</li>
			{/each}
		</ul>
	</section>
{/each}

{#if page?.next_before}
	<section class="section">
		<ul class="list">
			<li>
				<button type="button" class="cell tint centered" onclick={() => fetchPage(true)} disabled={busy}>{busy ? 'Loading…' : 'Load more'}</button>
			</li>
		</ul>
	</section>
{/if}
