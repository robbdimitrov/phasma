<script lang="ts">
	import { resolve } from '$app/paths';
	import { FileText, Hash } from '@lucide/svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import type {
		SearchHashtagItem,
		SearchItem,
		SearchPostItem,
		SearchType,
		SearchUserItem
	} from '$lib/server/api/search';

	let { items, q, type }: { items: SearchItem[]; q: string; type: SearchType } = $props();
</script>

{#if items.length === 0}
	<EmptyState
		icon="triangle-alert"
		title="No results"
		description="Nothing matched &ldquo;{q}&rdquo;. Try a different query."
	/>
{:else if type === 'users'}
	<ul class="flex flex-col gap-2">
		{#each items as item (item.id)}
			{@const user = item as SearchUserItem}
			<li>
				<a
					href={resolve(`/@${user.username}`)}
					class="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 p-3 transition-colors hover:bg-base-200"
				>
					<Avatar username={user.username} avatar={null} size="h-10 w-10" />
					<div class="min-w-0">
						<p class="truncate font-bold">{user.name}</p>
						<p class="truncate text-sm text-base-content/60">@{user.username}</p>
					</div>
				</a>
			</li>
		{/each}
	</ul>
{:else if type === 'posts'}
	<ul class="flex flex-col gap-2">
		{#each items as item (item.id)}
			{@const post = item as SearchPostItem}
			<li>
				<a
					href={resolve(`/posts/${post.id}`)}
					class="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 p-3 transition-colors hover:bg-base-200"
				>
					<span
						class="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-base-300 text-base-content/60"
					>
						<FileText class="h-5 w-5" />
					</span>
					<div class="min-w-0">
						<p class="truncate text-sm font-bold text-base-content/60">@{post.username}</p>
						{#if post.description}
							<p class="truncate text-sm text-base-content/80">{post.description}</p>
						{/if}
					</div>
				</a>
			</li>
		{/each}
	</ul>
{:else}
	<ul class="flex flex-col gap-2">
		{#each items as item (item.id)}
			{@const hashtag = item as SearchHashtagItem}
			<li>
				<a
					href={resolve(`/search?q=%23${encodeURIComponent(hashtag.name)}&type=hashtags`)}
					class="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 p-3 transition-colors hover:bg-base-200"
				>
					<span
						class="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-base-300 text-base-content/60"
					>
						<Hash class="h-5 w-5" />
					</span>
					<div class="min-w-0">
						<p class="truncate font-bold">#{hashtag.name}</p>
						<p class="truncate text-sm text-base-content/60">{hashtag.post_count} posts</p>
					</div>
				</a>
			</li>
		{/each}
	</ul>
{/if}
