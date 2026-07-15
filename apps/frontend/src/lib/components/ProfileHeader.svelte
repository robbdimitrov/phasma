<script lang="ts">
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import { Settings } from '@lucide/svelte';
	import { imageUrl } from '$lib/utils/imageUrl';
	import { pluralize } from '$lib/utils/pluralize';
	import type { User } from '$lib/types';
	import Linkified from '$lib/components/Linkified.svelte';

	let {
		profileUser = $bindable(),
		isCurrentUser,
		isAuthenticated,
		isFollowPending = $bindable(false),
		active
	}: {
		profileUser: User;
		isCurrentUser: boolean;
		isAuthenticated: boolean;
		isFollowPending?: boolean;
		active: 'posts' | 'likes' | 'followers' | 'following';
	} = $props();

	const stats = $derived([
		{
			key: 'posts' as const,
			suffix: '',
			count: profileUser.posts,
			label: pluralize(profileUser.posts, 'post')
		},
		{
			key: 'likes' as const,
			suffix: '/likes',
			count: profileUser.likes,
			label: pluralize(profileUser.likes, 'like')
		},
		{
			key: 'followers' as const,
			suffix: '/followers',
			count: profileUser.followers,
			label: pluralize(profileUser.followers, 'follower')
		},
		{
			key: 'following' as const,
			suffix: '/following',
			count: profileUser.following,
			label: 'following'
		}
	]);
</script>

<div
	class="flex w-full flex-col items-center gap-6 rounded-2xl border border-base-300 bg-base-100 p-6 text-base-content shadow-lg shadow-slate-900/5 sm:px-8 md:flex-row md:items-start"
>
	<div
		class="relative h-24 w-24 shrink-0 overflow-hidden rounded-full border border-base-300 shadow-md sm:h-28 sm:w-28"
	>
		<img
			class="h-full w-full object-cover"
			src={imageUrl(profileUser.avatar)}
			alt={profileUser.username}
		/>
	</div>

	<div class="flex w-full grow flex-col gap-4 text-center md:text-left">
		<div
			class="flex w-full flex-col items-center gap-4 sm:flex-row sm:items-start sm:justify-between sm:text-left"
		>
			<div class="grid min-w-0 gap-1">
				<h1
					class="wrap-break-word text-2xl font-black tracking-tight text-base-content sm:text-3xl"
				>
					{profileUser.name || profileUser.username}
				</h1>
				<p class="wrap-break-word text-sm font-bold text-base-content/60">
					@{profileUser.username}
				</p>
			</div>

			{#if isCurrentUser}
				<a
					href={resolve('/settings')}
					class="btn btn-neutral btn-sm h-10 min-h-10 shrink-0 gap-2 rounded-full px-5 font-extrabold shadow-md shadow-slate-900/15"
				>
					<Settings class="h-4 w-4" />
					Settings
				</a>
			{:else if isAuthenticated}
				<form
					method="POST"
					action={resolve(`/@${profileUser.username}`) +
						(profileUser.isFollowing ? '?/unfollow' : '?/follow')}
					use:enhance={() => {
						isFollowPending = true;
						const wasFollowing = profileUser.isFollowing;
						profileUser = {
							...profileUser,
							isFollowing: !wasFollowing,
							followers: profileUser.followers + (wasFollowing ? -1 : 1)
						};
						return async ({ result }) => {
							isFollowPending = false;
							if (result.type === 'error' || result.type === 'failure') {
								profileUser = {
									...profileUser,
									isFollowing: wasFollowing,
									followers: profileUser.followers + (wasFollowing ? 1 : -1)
								};
							}
						};
					}}
				>
					<button
						type="submit"
						disabled={isFollowPending}
						class="btn btn-sm h-10 min-h-10 shrink-0 gap-2 rounded-full px-5 font-extrabold shadow-md shadow-slate-900/15 {profileUser.isFollowing
							? 'btn-outline'
							: 'btn-neutral'}"
					>
						{profileUser.isFollowing ? 'Unfollow' : 'Follow'}
					</button>
				</form>
			{:else}
				<a
					href={resolve('/login')}
					class="btn btn-neutral btn-sm h-10 min-h-10 shrink-0 gap-2 rounded-full px-5 font-extrabold shadow-md shadow-slate-900/15"
				>
					Follow
				</a>
			{/if}
		</div>

		{#if profileUser.bio}
			<p class="w-full max-w-xl text-sm leading-relaxed text-base-content/70">
				<Linkified text={profileUser.bio} />
			</p>
		{/if}

		<div
			class="flex items-center justify-center gap-6 text-sm font-bold text-base-content/70 md:justify-start"
		>
			{#each stats as s (s.key)}
				<a
					href={resolve(`/@${profileUser.username}${s.suffix}`)}
					aria-current={active === s.key ? 'page' : undefined}
					class="transition-colors hover:text-primary {active === s.key ? 'text-primary/70' : ''}"
				>
					<strong class="font-black {active === s.key ? 'text-primary' : 'text-base-content'}"
						>{s.count}</strong
					>
					{s.label}
				</a>
			{/each}
		</div>
	</div>
</div>
