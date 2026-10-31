<script lang="ts">
	import AnnouncementBanner from './AnnouncementBanner.svelte';
	import ViewingAs from './ViewingAs.svelte';
	import type { Announcement } from './admin';

	// The banners /me asks for, for TitleBar's banners snippet: View as when viewing is the account viewed, and the
	// announcement until it is closed on this device (an edited announcement has a new updated_at and shows again).
	let {
		viewing,
		announcement,
		storageKey,
		onstopped
	}: { viewing?: string; announcement?: Announcement | null; storageKey: string; onstopped: (restored: boolean) => void } = $props();

	let closedAt = $state(readClosed());
	let shown = $derived(announcement && announcement.updated_at !== closedAt ? announcement : undefined);

	function readClosed(): string | null {
		try {
			return localStorage.getItem(storageKey);
		} catch (e) {
			console.warn('cannot read the announcement closed on this device:', e);
			return null;
		}
	}

	function close(updatedAt: string) {
		closedAt = updatedAt;
		try {
			localStorage.setItem(storageKey, updatedAt);
		} catch (e) {
			console.warn('cannot remember the announcement closed on this device:', e);
		}
	}
</script>

{#if viewing}
	<ViewingAs email={viewing} {onstopped} />
{/if}
{#if shown}
	{@const updatedAt = shown.updated_at}
	<AnnouncementBanner announcement={shown} onclose={() => close(updatedAt)} />
{/if}
