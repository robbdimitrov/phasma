<script lang="ts">
	import { createPagination } from '$lib/createPagination.svelte';
	import ProfileHeader from '$lib/components/ProfileHeader.svelte';
	import PostCard from '$lib/components/PostCard.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import LoadMoreButton from '$lib/components/LoadMoreButton.svelte';
	import { fetchCursorPage } from '$lib/utils/clientFetch';
	import type { PageData } from './$types';
	import type { Post } from '$lib/types';

	let { data }: { data: PageData } = $props();

	let profileUser = $derived(data.profileUser);
	let isFollowPending = $state(false);

	const pagination = createPagination(
		() => ({ items: data.posts, nextCursor: data.nextCursor }),
		(cursor) => fetchCursorPage<Post>(fetch, `/@${data.profileUser.username}/likes`, cursor)
	);

	const isCurrentUser = $derived(data.currentUser?.id === profileUser.id);
</script>

<div class="mx-auto flex max-w-5xl flex-col gap-6">
	<ProfileHeader
		{profileUser}
		{isCurrentUser}
		isAuthenticated={!!data.currentUser}
		bind:isFollowPending
		active="likes"
	/>

	<div class="h-px w-full bg-base-300" aria-hidden="true"></div>

	<div class="mx-auto flex w-full max-w-xl flex-col gap-6">
		{#if pagination.items.length === 0}
			<EmptyState
				icon="heart"
				title="No liked posts yet"
				description="Liked photos will appear here so they are easy to find again."
				actionLabel="Browse Feed"
				actionRoute="/feed"
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
</div>
