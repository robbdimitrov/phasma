<script lang="ts">
	import { resolve } from '$app/paths';
	import { Hash } from '@lucide/svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import type { SuggestionItem } from '$lib/utils/interleaveSuggestions';
	import SearchRowCard from './SearchRowCard.svelte';

	// `bare`: no card chrome, so RecentSearches.svelte can add a trailing button beside
	// this row's <a> instead of nesting it inside.
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
</script>

{#snippet content()}
	{#if row.type === 'users'}
		<Avatar username={row.item.username} avatar={row.item.avatar} size="h-10 w-10" />
		<span class="min-w-0 flex-1">
			<span class="block truncate font-bold">{row.item.name || row.item.username}</span>
			<span class="block truncate text-sm text-base-content/60">@{row.item.username}</span>
		</span>
	{:else}
		<span
			class="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-base-300 text-base-content"
		>
			<Hash class="h-5 w-5" />
		</span>
		<div class="min-w-0">
			<p class="truncate font-bold">#{row.item.name}</p>
			<p class="truncate text-sm text-base-content/60">{row.item.postCount} posts</p>
		</div>
	{/if}
{/snippet}

{#if row.type === 'users'}
	{@const href = resolve(`/@${row.item.username}`)}
	{#if bare}
		<a {href} {onclick} {onmouseenter} class="flex min-w-0 flex-1 items-center gap-3">
			{@render content()}
		</a>
	{:else}
		<SearchRowCard tag="a" {href} {active} {onclick} {onmouseenter}>
			{@render content()}
		</SearchRowCard>
	{/if}
{:else}
	{@const href = resolve(`/search?q=%23${encodeURIComponent(row.item.name)}`)}
	{#if bare}
		<a {href} {onclick} {onmouseenter} class="flex min-w-0 flex-1 items-center gap-3">
			{@render content()}
		</a>
	{:else}
		<SearchRowCard tag="a" {href} {active} {onclick} {onmouseenter}>
			{@render content()}
		</SearchRowCard>
	{/if}
{/if}
