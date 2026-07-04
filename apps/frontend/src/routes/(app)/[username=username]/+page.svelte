<script lang="ts">
	import { resolve } from '$app/paths';
	import { createPagination } from '$lib/createPagination.svelte';
	import ProfileHeader from '$lib/components/ProfileHeader.svelte';
	import Thumbnail from '$lib/components/Thumbnail.svelte';
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
		(cursor) => fetchCursorPage<Post>(fetch, `/@${data.profileUser.username}`, cursor)
	);

	const isCurrentUser = $derived(data.currentUser?.id === profileUser.id);
	const username = $derived(profileUser.username);
</script>

<svelte:head>
	<title>@{data.profileUser.username} — Phasma</title>
</svelte:head>

<div class="mx-auto flex max-w-5xl flex-col gap-6">
	<ProfileHeader
		{profileUser}
		{isCurrentUser}
		isAuthenticated={!!data.currentUser}
		bind:isFollowPending
	/>

	<div class="h-px w-full bg-base-300" aria-hidden="true"></div>

	<div class="tabs tabs-bordered justify-center font-bold">
		<a class="tab tab-active" href={resolve(`/@${username}`)}>Posts</a>
		<a class="tab" href={resolve(`/@${username}/followers`)}>Followers</a>
		<a class="tab" href={resolve(`/@${username}/following`)}>Following</a>
	</div>

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
				icon="camera"
				title="No uploads yet"
				description={isCurrentUser
					? 'Your profile is ready. Share your first photo to start building your grid.'
					: 'This profile has not shared any photos yet.'}
				actionLabel={isCurrentUser ? 'Share Your First Photo' : 'Browse Feed'}
				actionRoute={isCurrentUser ? '/upload' : '/feed'}
				actionStyle={isCurrentUser ? 'primary' : 'outline'}
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
