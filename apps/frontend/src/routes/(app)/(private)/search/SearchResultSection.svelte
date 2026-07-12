<script lang="ts">
	import LoadMoreButton from '$lib/components/LoadMoreButton.svelte';
	import SearchResultList from './SearchResultList.svelte';
	import type { SearchItem, SearchType } from '$lib/server/api/search';

	let {
		label,
		type,
		items,
		done,
		loading,
		error,
		onMore
	}: {
		label: string;
		type: SearchType;
		items: SearchItem[];
		done: boolean;
		loading: boolean;
		error: string | null;
		onMore: () => void;
	} = $props();
</script>

{#if items.length > 0}
	<section class="flex flex-col gap-3">
		<h2 class="text-sm font-bold uppercase tracking-wide text-base-content/60">{label}</h2>
		<SearchResultList {type} {items} />
		{#if !done}
			<div class="flex flex-col items-center gap-2">
				{#if error}
					<p class="text-sm text-error">{error}</p>
				{/if}
				<LoadMoreButton {loading} onclick={onMore} />
			</div>
		{/if}
	</section>
{/if}
