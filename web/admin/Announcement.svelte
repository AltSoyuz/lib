<script lang="ts">
	import Sheet from '../Sheet.svelte';
	import { errorText } from '../api';
	import AnnouncementBanner from './AnnouncementBanner.svelte';
	import { ANNOUNCEMENT_MAX_CHARS, clearAnnouncement, setAnnouncement, whenShort, type Announcement } from './admin';

	// The announcement row of the back office and its editor; onchanged gets the new one, null once cleared.
	let { announcement, onchanged }: { announcement: Announcement | null; onchanged: (a: Announcement | null) => void } = $props();

	const toneLabels: Record<Announcement['tone'], string> = { info: 'Info', warning: 'Warning' };
	const messages: Record<string, string> = {
		invalid_text: `The message must be 1 to ${ANNOUNCEMENT_MAX_CHARS} characters long.`,
		invalid_tone: 'Choose Info or Warning.'
	};

	let open = $state(false);
	let textDraft = $state('');
	let toneDraft = $state<Announcement['tone']>('info');
	let busy = $state(false);
	let error = $state<string | null>(null);

	let chars = $derived([...textDraft.trim()].length);
	let dirty = $derived(textDraft.trim() !== (announcement?.text ?? '') || toneDraft !== (announcement?.tone ?? 'info'));
	let valid = $derived(chars > 0 && chars <= ANNOUNCEMENT_MAX_CHARS);

	function edit() {
		error = null;
		textDraft = announcement?.text ?? '';
		toneDraft = announcement?.tone ?? 'info';
		open = true;
	}

	async function run(fn: () => Promise<void>) {
		busy = true;
		error = null;
		try {
			await fn();
			open = false;
		} catch (e) {
			error = messages[(e as Error).message] ?? errorText(e);
		} finally {
			busy = false;
		}
	}

	const post = () => run(async () => onchanged(await setAnnouncement(textDraft.trim(), toneDraft)));

	function clear() {
		if (!confirm('Clear the announcement? Nobody will see it any more.')) return;
		run(async () => {
			await clearAnnouncement();
			onchanged(null);
		});
	}
</script>

<section class="section">
	<h2>Announcement</h2>
	<ul class="list">
		<li>
			<button type="button" class="cell chevron" onclick={edit}>
				{#if announcement}
					<span class="content">
						<span class="title text">{announcement.text}</span>
						<span class="subtitle">{toneLabels[announcement.tone]} · posted {whenShort(announcement.updated_at)}</span>
					</span>
				{:else}
					<span class="content">None</span>
					<span class="value">Post one</span>
				{/if}
			</button>
		</li>
	</ul>
</section>

<Sheet title="Announcement" bind:open {dirty} {busy} canSave={dirty && valid} saveLabel="Post" onsave={post}>
	<section class="section">
		<h2>Message</h2>
		<ul class="list">
			<li>
				<textarea aria-label="Message" bind:value={textDraft} placeholder="What everyone should know" disabled={busy}></textarea>
			</li>
			<li>
				<label class="cell picker">
					<span class="content">Tone</span>
					<span class="value">{toneLabels[toneDraft]}</span>
					<select aria-label="Tone" bind:value={toneDraft} disabled={busy}>
						<option value="info">Info</option>
						<option value="warning">Warning</option>
					</select>
				</label>
			</li>
		</ul>
		{#if error}
			<p class="footer error" role="alert">{error}</p>
		{:else}
			<p class="footer num" class:error={chars > ANNOUNCEMENT_MAX_CHARS}>{chars} / {ANNOUNCEMENT_MAX_CHARS} characters</p>
		{/if}
	</section>
	<section class="section">
		<h2>Preview</h2>
		{#if textDraft.trim()}
			<AnnouncementBanner announcement={{ text: textDraft.trim(), tone: toneDraft }} />
		{:else}
			<p class="empty-text">Write a message to see it here.</p>
		{/if}
	</section>
	{#if announcement}
		<section class="section">
			<ul class="list">
				<li><button type="button" class="cell destructive centered" onclick={clear} disabled={busy}>Clear announcement</button></li>
			</ul>
		</section>
	{/if}
</Sheet>

<style>
	.text {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
</style>
