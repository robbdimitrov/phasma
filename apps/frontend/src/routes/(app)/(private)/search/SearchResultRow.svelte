<script lang="ts">
	import { resolve } from '$app/paths';
	import { Hash } from '@lucide/svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import { imageUrl } from '$lib/utils/imageUrl';
	import type { SearchAllItem } from '$lib/server/api/search';

	let {
		row,
		active = false,
		onmouseenter,
		onclick
	}: {
		row: SearchAllItem;
		active?: boolean;
		onmouseenter?: () => void;
		onclick?: () => void;
	} = $props();

</script>

{#if row.type === 'users'}
	<a
		href={resolve(`/@${row.item.username}`)}
		{onclick}
		{onmouseenter}
		class="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 p-3 transition-colors hover:bg-base-200 {active
			? 'bg-base-200'
			: ''}"
	>
		<Avatar username={row.item.username} avatar={row.item.avatar} size="h-10 w-10" />
		<span class="min-w-0 flex-1">
			<span class="block truncate font-bold">{row.item.name || row.item.username}</span>
			<span class="block truncate text-sm text-base-content/60">@{row.item.username}</span>
		</span>
	</a>
{:else if row.type === 'posts'}
	<a
		href={resolve(`/posts/${row.item.id}`)}
		{onclick}
		{onmouseenter}
		class="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 p-3 transition-colors hover:bg-base-200 {active
			? 'bg-base-200'
			: ''}"
	>
		<img
			class="h-10 w-10 shrink-0 rounded-md object-cover"
			src={imageUrl(row.item.filename)}
			alt=""
			loading="lazy"
		/>
		<span class="min-w-0 flex-1">
			<span class="block truncate font-bold">@{row.item.username}</span>
			{#if row.item.description}
				<span class="block truncate text-sm text-base-content/60">{row.item.description}</span>
			{/if}
		</span>
	</a>
{:else}
	<a
		href={resolve(`/search?q=%23${encodeURIComponent(row.item.name)}`)}
		{onclick}
		{onmouseenter}
		class="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 p-3 transition-colors hover:bg-base-200 {active
			? 'bg-base-200'
			: ''}"
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
