<script lang="ts">
	import { resolve } from '$app/paths';
	import { Hash } from '@lucide/svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import type { SuggestionItem } from '$lib/utils/interleaveSuggestions';
	import { SEARCH_ROW_CARD_CLASS } from './searchRowCard';

	let {
		row,
		active = false,
		bare = false,
		onmouseenter,
		onclick
	}: {
		row: SuggestionItem;
		active?: boolean;
		bare?: boolean;
		onmouseenter?: () => void;
		onclick?: () => void;
	} = $props();

	// `bare` drops the row's own card chrome (border/padding/hover) so a
	// parent can place a trailing action (e.g. remove) inside one shared
	// card without nesting a <button> inside this <a> — RecentSearches.svelte
	// does this; the live typeahead dropdown uses the default card mode.
	let rowClass = $derived(
		bare
			? 'flex min-w-0 flex-1 items-center gap-3'
			: `${SEARCH_ROW_CARD_CLASS} ${active ? 'bg-base-200' : ''}`
	);
</script>

{#if row.type === 'users'}
	<a href={resolve(`/@${row.item.username}`)} {onclick} {onmouseenter} class={rowClass}>
		<Avatar username={row.item.username} avatar={row.item.avatar} size="h-10 w-10" />
		<span class="min-w-0 flex-1">
			<span class="block truncate font-bold">{row.item.name || row.item.username}</span>
			<span class="block truncate text-sm text-base-content/60">@{row.item.username}</span>
		</span>
	</a>
{:else}
	<a
		href={resolve(`/search?q=%23${encodeURIComponent(row.item.name)}`)}
		{onclick}
		{onmouseenter}
		class={rowClass}
	>
		<span
			class="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-base-300 text-base-content/60"
		>
			<Hash class="h-5 w-5" />
		</span>
		<div class="min-w-0">
			<p class="truncate font-bold">#{row.item.name}</p>
			<p class="truncate text-sm text-base-content/60">{row.item.postCount} posts</p>
		</div>
	</a>
{/if}
