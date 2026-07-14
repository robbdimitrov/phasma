<script lang="ts">
	import { enhance } from '$app/forms';
	import { resolve } from '$app/paths';
	import { Users } from '@lucide/svelte';
	import { SvelteMap, SvelteSet } from 'svelte/reactivity';
	import Avatar from '$lib/components/Avatar.svelte';
	import Thumbnail from '$lib/components/Thumbnail.svelte';
	import type { Post, User } from '$lib/types';

	let { suggested, popular }: { suggested: User[]; popular: Post[] } = $props();

	let pendingFollowIds = new SvelteSet<string>();
	let followingOverrides = new SvelteMap<string, boolean>();

	let scrollEl: HTMLDivElement | undefined = $state();
	let atStart = $state(true);
	let atEnd = $state(true);

	function updateEdges() {
		if (!scrollEl) return;
		atStart = scrollEl.scrollLeft <= 0;
		atEnd = scrollEl.scrollLeft + scrollEl.clientWidth >= scrollEl.scrollWidth - 1;
	}

	// Only fade an edge once there's actually more to scroll toward, so the
	// row never looks cut off before the user has scrolled.
	function edgeMask(start: boolean, end: boolean): string {
		if (start && end) return 'none';
		const stops = [
			start ? 'black 0' : 'transparent 0, black 32px',
			end ? 'black 100%' : 'black calc(100% - 32px), transparent 100%'
		].join(', ');
		return `linear-gradient(to right, ${stops})`;
	}

	$effect(() => {
		if (scrollEl) updateEdges();
	});

	let mask = $derived(edgeMask(atStart, atEnd));
</script>

{#if suggested.length > 0}
	<div class="flex flex-col gap-3">
		<h2 class="text-sm font-bold text-base-content/60 uppercase tracking-wide">People to follow</h2>
		<div
			bind:this={scrollEl}
			onscroll={updateEdges}
			class="scrollbar-hide flex snap-x gap-3 overflow-x-auto"
			style:mask-image={mask}
			style:-webkit-mask-image={mask}
		>
			{#each suggested as user (user.id)}
				{@const isFollowing = followingOverrides.get(user.id) ?? user.isFollowing}
				<div
					class="flex w-40 shrink-0 snap-start flex-col items-center gap-2 rounded-2xl border border-base-300 bg-base-100 p-4 text-center"
				>
					<a
						href={resolve(`/@${user.username}`)}
						class="flex w-full min-w-0 flex-col items-center gap-2"
					>
						<Avatar username={user.username} avatar={user.avatar} size="h-16 w-16" />
						<span class="w-full min-w-0">
							<span class="block truncate text-sm font-bold text-base-content">
								{user.name || user.username}
							</span>
							<span class="block truncate text-xs text-base-content/60">@{user.username}</span>
						</span>
					</a>
					<form
						method="POST"
						action="?/{isFollowing ? 'unfollow' : 'follow'}"
						class="w-full"
						use:enhance={() => {
							pendingFollowIds.add(user.id);
							followingOverrides.set(user.id, !isFollowing);
							return async ({ result }) => {
								pendingFollowIds.delete(user.id);
								if (result.type === 'error' || result.type === 'failure') {
									followingOverrides.set(user.id, isFollowing);
								}
							};
						}}
					>
						<input type="hidden" name="username" value={user.username} />
						<button
							type="submit"
							disabled={pendingFollowIds.has(user.id)}
							class="btn btn-sm h-9 min-h-9 w-full rounded-full px-3 text-xs font-extrabold {isFollowing
								? 'btn-outline'
								: 'btn-neutral'}"
						>
							{#if pendingFollowIds.has(user.id)}
								<span class="loading loading-spinner loading-xs"></span>
							{:else}
								{isFollowing ? 'Unfollow' : 'Follow'}
							{/if}
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

<svelte:window onresize={updateEdges} />

<style>
	/* Tailwind has no utility for hiding the native scrollbar; the row still
	   scrolls by touch, drag, or wheel. */
	.scrollbar-hide {
		scrollbar-width: none;
		-ms-overflow-style: none;
	}
	.scrollbar-hide::-webkit-scrollbar {
		display: none;
	}
</style>
