<script lang="ts">
	import { createPagination } from '$lib/createPagination.svelte';
	import ProfileHeader from '$lib/components/ProfileHeader.svelte';
	import Thumbnail from '$lib/components/Thumbnail.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import { pageTitle } from '$lib/pageTitle';
	import LoadMoreButton from '$lib/components/LoadMoreButton.svelte';
	import { fetchCursorPage } from '$lib/utils/clientFetch';
	import type { PageData } from './$types';
	import type { Post } from '$lib/types';

	let { data }: { data: PageData } = $props();

	let profileUser = $derived(data.profileUser);
	let isFollowPending = $state(false);
	const isCurrentUser = $derived(data.currentUser?.id === profileUser.id);

	const pagination = createPagination(
		() => ({ items: data.posts, nextCursor: data.nextCursor }),
		(cursor) => fetchCursorPage<Post>(fetch, `/@${data.profileUser.username}/likes`, cursor)
	);

	const emptyState = $derived({
		icon: 'heart' as const,
		title: 'No liked posts yet',
		description: 'Liked photos will appear here so they are easy to find again.',
		actionLabel: 'Browse Feed',
		actionRoute: '/feed' as const,
		actionStyle: 'primary' as const
	});
</script>

<svelte:head>
	<title>{pageTitle(`Liked posts by @${data.profileUser.username}`)}</title>
</svelte:head>

<div class="mx-auto flex max-w-5xl flex-col gap-6">
	<ProfileHeader
		{profileUser}
		{isCurrentUser}
		isAuthenticated={!!data.currentUser}
		bind:isFollowPending
		active={data.mode}
	/>

	<div class="h-px w-full bg-base-300" aria-hidden="true"></div>

	{#if pagination.items.length > 0}
		<div class="grid grid-cols-3 gap-2 sm:gap-4">
			{#each pagination.items as post (post.publicId)}
				<div class="aspect-square">
					<Thumbnail {post} />
				</div>
			{/each}
		</div>
	{:else}
		<div class="mx-auto w-full max-w-xl">
			<EmptyState
				icon={emptyState.icon}
				title={emptyState.title}
				description={emptyState.description}
				actionLabel={emptyState.actionLabel}
				actionRoute={emptyState.actionRoute}
				actionStyle={emptyState.actionStyle}
			/>
		</div>
	{/if}

	{#if !pagination.done && pagination.items.length > 0}
		<div class="flex flex-col items-center gap-2">
			{#if pagination.error}
				<p class="text-sm text-error">{pagination.error}</p>
			{/if}
			<LoadMoreButton loading={pagination.loading} onclick={pagination.more} />
		</div>
	{/if}
</div>
