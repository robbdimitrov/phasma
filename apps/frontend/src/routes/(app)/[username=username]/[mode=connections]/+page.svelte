<script lang="ts">
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import { createPagination } from '$lib/createPagination.svelte';
	import ProfileHeader from '$lib/components/ProfileHeader.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import LoadMoreButton from '$lib/components/LoadMoreButton.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import { fetchCursorPage } from '$lib/utils/clientFetch';
	import type { PageData } from './$types';
	import type { User } from '$lib/types';

	let { data }: { data: PageData } = $props();

	let profileUser = $derived(data.profileUser);
	let isFollowPending = $state(false);
	let pendingFollowIds = $state(new Set<string>());
	let followingOverrides = $state(new Map<string, boolean>());

	const isCurrentUser = $derived(data.currentUser?.id === profileUser.id);
	const username = $derived(profileUser.username);

	const pagination = createPagination(
		() => ({ items: data.users, nextCursor: data.nextCursor }),
		(cursor) => fetchCursorPage<User>(fetch, `/@${username}/${data.mode}`, cursor)
	);

	const emptyTitle = $derived(
		data.mode === 'followers' ? 'No followers yet' : 'Not following anyone yet'
	);
	const emptyDesc = $derived(
		data.mode === 'followers'
			? 'Followers will appear here as people follow this profile.'
			: 'Profiles this user follows will appear here.'
	);
</script>

<div class="mx-auto flex max-w-5xl flex-col gap-6">
	<ProfileHeader
		{profileUser}
		{isCurrentUser}
		isAuthenticated={!!data.currentUser}
		bind:isFollowPending
	/>

	<div class="h-px w-full bg-base-300" aria-hidden="true"></div>

	<div class="tabs tabs-bordered justify-center font-bold">
		<a class="tab" href={resolve(`/@${username}`)}>Posts</a>
		<a
			class="tab"
			class:tab-active={data.mode === 'followers'}
			href={resolve(`/@${username}/followers`)}>Followers</a
		>
		<a
			class="tab"
			class:tab-active={data.mode === 'following'}
			href={resolve(`/@${username}/following`)}>Following</a
		>
	</div>

	{#if pagination.items.length > 0}
		<div class="mx-auto flex w-full max-w-xl flex-col gap-3">
			{#each pagination.items as user (user.id)}
				{@const isFollowing = followingOverrides.get(user.id) ?? user.isFollowing}
				<div
					class="flex items-start gap-3 rounded-lg border border-base-300 bg-base-100 p-4 shadow-sm shadow-slate-900/5"
				>
					<Avatar username={user.username} avatar={user.avatar} size="h-8 w-8" class="mt-0.5" />
					<a href={resolve(`/@${user.username}`)} class="min-w-0 flex-1">
						<div class="truncate text-sm font-black text-base-content">
							{user.name || user.username}
						</div>
						<div class="truncate text-xs font-bold text-base-content/60">@{user.username}</div>
						{#if user.bio}
							<p class="mt-2 max-h-10 overflow-hidden text-sm leading-5 text-base-content/70">
								{user.bio}
							</p>
						{/if}
					</a>
					{#if !data.currentUser || data.currentUser.id !== user.id}
						{#if data.currentUser}
							<form
								method="POST"
								action="/@{username}?/{isFollowing ? 'unfollow' : 'follow'}"
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
								<button
									type="submit"
									disabled={pendingFollowIds.has(user.id)}
									class="btn btn-sm h-9 min-h-9 shrink-0 rounded-full px-4 text-xs font-extrabold {isFollowing
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
						{:else}
							<a
								href={resolve('/login')}
								class="btn btn-neutral btn-sm h-9 min-h-9 shrink-0 rounded-full px-4 text-xs font-extrabold"
							>
								Follow
							</a>
						{/if}
					{/if}
				</div>
			{/each}
		</div>
	{:else}
		<div class="mx-auto w-full max-w-xl">
			<EmptyState
				icon="square-plus"
				title={emptyTitle}
				description={emptyDesc}
				actionLabel="Browse Feed"
				actionRoute="/feed"
				actionStyle="outline"
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
