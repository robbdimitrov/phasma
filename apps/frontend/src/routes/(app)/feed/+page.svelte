<script lang="ts">
	import { createPagination } from '$lib/createPagination.svelte';
	import PostCard from '$lib/components/PostCard.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import LoadMoreButton from '$lib/components/LoadMoreButton.svelte';
	import { fetchJson } from '$lib/utils/clientFetch';
	import type { PageData } from './$types';
	import type { Post } from '$lib/types';

	let { data }: { data: PageData } = $props();

	const pagination = createPagination(
		() => ({ items: data.posts, nextCursor: data.nextCursor }),
		async (cursor) => {
			const res = await fetch(`/feed?cursor=${encodeURIComponent(cursor)}`);
			return fetchJson<{ items: Post[]; nextCursor: string | null }>(res);
		}
	);
</script>

<svelte:head>
	<title>Feed — Phasma</title>
</svelte:head>

<div class="mx-auto flex max-w-xl flex-col items-center gap-6">
	{#if pagination.items.length === 0}
		<EmptyState
			icon="camera"
			title="Your feed is empty"
			description="Follow people to see their posts here."
			actionLabel="Find People to Follow"
			actionRoute="/search"
			actionStyle="outline"
		/>
	{/if}

	<div class="flex w-full flex-col gap-6">
		{#each pagination.items as post (post.publicId)}
			<PostCard {post} currentUsername={data.currentUser?.username ?? null} singleView={false} />
		{/each}
	</div>

	{#if !pagination.done && pagination.items.length > 0}
		{#if pagination.error}
			<p class="text-sm text-error">{pagination.error}</p>
		{/if}
		<LoadMoreButton loading={pagination.loading} onclick={pagination.more} />
	{/if}
</div>
