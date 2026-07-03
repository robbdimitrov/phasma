<script lang="ts">
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import { Users } from '@lucide/svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import Thumbnail from '$lib/components/Thumbnail.svelte';
	import type { Post, User } from '$lib/types';

	let { suggested, popular }: { suggested: User[]; popular: Post[] } = $props();
</script>

{#if suggested.length > 0}
	<div class="flex flex-col gap-3">
		<h2 class="text-sm font-bold text-base-content/60 uppercase tracking-wide">People to follow</h2>
		<div class="flex flex-col gap-2">
			{#each suggested as user (user.id)}
				<div class="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 p-3">
					<Avatar username={user.username} avatar={user.avatar} size="h-10 w-10" />
					<a href={resolve(`/@${user.username}`)} class="min-w-0 flex-1">
						<p class="truncate font-bold">{user.name || user.username}</p>
						<p class="truncate text-sm text-base-content/60">@{user.username}</p>
					</a>
					<form method="POST" action="?/{user.isFollowing ? 'unfollow' : 'follow'}" use:enhance>
						<input type="hidden" name="username" value={user.username} />
						<button
							type="submit"
							class="btn btn-sm h-9 min-h-9 shrink-0 rounded-full px-4 text-xs font-extrabold {user.isFollowing
								? 'btn-outline'
								: 'btn-neutral'}"
						>
							{user.isFollowing ? 'Unfollow' : 'Follow'}
						</button>
					</form>
				</div>
			{/each}
		</div>
	</div>
{/if}

{#if popular.length > 0}
	<div class="flex flex-col gap-3">
		<h2 class="text-sm font-bold text-base-content/60 uppercase tracking-wide">Popular posts</h2>
		<div class="grid grid-cols-3 gap-2">
			{#each popular as post (post.publicId)}
				<Thumbnail {post} />
			{/each}
		</div>
	</div>
{/if}

{#if suggested.length === 0 && popular.length === 0}
	<div class="flex flex-col items-center gap-3 py-12 text-base-content/40">
		<Users class="h-12 w-12" />
		<p class="text-sm">Search for users, posts, or hashtags</p>
	</div>
{/if}
