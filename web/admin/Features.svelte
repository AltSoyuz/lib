<script lang="ts">
	import { onMount } from 'svelte';
	import { errorText } from '../api';
	import { features, keepFeatures, loadFeatures, type Audience, type Feature } from '../features.svelte';
	import { setFeature } from './admin';

	// The features of the back office, each for everyone or for the admins only; nothing shows for an app without any.
	const audienceLabels: Record<Audience, string> = { everyone: 'Everyone', admins: 'Admins' };

	let busy = $state(false);
	let error = $state<string | null>(null);

	onMount(() => void loadFeatures());

	async function change(feature: Feature, select: HTMLSelectElement) {
		busy = true;
		error = null;
		try {
			const saved = await setFeature(feature.name, select.value as Audience);
			keepFeatures(features.list.map((f) => (f.name === saved.name ? saved : f)));
		} catch (e) {
			select.value = feature.audience;
			error = errorText(e);
		} finally {
			busy = false;
		}
	}
</script>

{#if features.list.length > 0}
	<section class="section">
		<h2>Features</h2>
		<ul class="list">
			{#each features.list as feature (feature.name)}
				<li>
					<label class="cell picker">
						<span class="content">{feature.label}</span>
						<span class="value">{audienceLabels[feature.audience]}</span>
						<select aria-label="Who sees {feature.label}" value={feature.audience} disabled={busy} onchange={(e) => change(feature, e.currentTarget)}>
							<option value="everyone">Everyone</option>
							<option value="admins">Admins</option>
						</select>
					</label>
				</li>
			{/each}
		</ul>
		{#if error}
			<p class="footer error" role="alert">{error}</p>
		{:else}
			<p class="footer">A feature for the admins is hidden from everyone else, the next time they open the app; their data stays.</p>
		{/if}
	</section>
{/if}
