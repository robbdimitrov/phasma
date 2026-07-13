<script lang="ts">
	import { resolve } from '$app/paths';
	import { Hash } from '@lucide/svelte';
	import UserLink from '$lib/components/UserLink.svelte';
	import { imageUrl } from '$lib/utils/imageUrl';
	import type {
		SearchHashtagItem,
		SearchItem,
		SearchPostItem,
		SearchType,
		SearchUserItem
	} from '$lib/server/api/search';

	let { items, type }: { items: SearchItem[]; type: SearchType } = $props();
</script>

{#if type === 'users'}
	{@const users = items as SearchUserItem[]}
	<ul class="flex flex-col gap-2">
		{#each users as user (user.username)}
			<li>
				<UserLink
					username={user.username}
					avatar={user.avatar}
					primary={user.name || user.username}
					secondary={`@${user.username}`}
					size="h-10 w-10"
					class="rounded-2xl border border-base-300 bg-base-100 p-3 transition-colors hover:bg-base-200"
				/>
			</li>
		{/each}
	</ul>
{:else if type === 'posts'}
	{@const posts = items as SearchPostItem[]}
	<div class="grid grid-cols-3 gap-2 sm:gap-4">
		{#each posts as post (post.id)}
			<a
				href={resolve(`/posts/${post.id}`)}
				class="block aspect-square overflow-hidden rounded-2xl border border-base-300 bg-base-300/30"
			>
				<img
					class="h-full w-full object-cover"
					src={imageUrl(post.filename)}
					alt={post.description}
					loading="lazy"
				/>
			</a>
		{/each}
	</div>
{:else}
	{@const tags = items as SearchHashtagItem[]}
	<ul class="flex flex-col gap-2">
		{#each tags as hashtag (hashtag.name)}
			<li>
				<a
					href={resolve(`/search?q=%23${encodeURIComponent(hashtag.name)}`)}
					class="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 p-3 transition-colors hover:bg-base-200"
				>
					<span
						class="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-base-300 text-base-content/60"
					>
						<Hash class="h-5 w-5" />
					</span>
					<div class="min-w-0">
						<p class="truncate font-bold">#{hashtag.name}</p>
						<p class="truncate text-sm text-base-content/60">{hashtag.postCount} posts</p>
					</div>
				</a>
			</li>
		{/each}
	</ul>
{/if}
