<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { Hash } from '@lucide/svelte';
	import { typeaheadNav } from '$lib/utils/typeaheadNav';
	import { imageUrl } from '$lib/utils/imageUrl';
	import Avatar from '$lib/components/Avatar.svelte';
	import type { UserSuggestion, HashtagSuggestion, SearchPostItem } from '$lib/server/api/search';

	let {
		users,
		posts,
		hashtags,
		onclose
	}: {
		users: UserSuggestion[];
		posts: SearchPostItem[];
		hashtags: HashtagSuggestion[];
		onclose: () => void;
	} = $props();

	type FlatItem =
		| { category: 'users'; user: UserSuggestion }
		| { category: 'posts'; post: SearchPostItem }
		| { category: 'hashtags'; hashtag: HashtagSuggestion };

	let flat = $derived<FlatItem[]>([
		...users.map((user): FlatItem => ({ category: 'users', user })),
		...posts.map((post): FlatItem => ({ category: 'posts', post })),
		...hashtags.map((hashtag): FlatItem => ({ category: 'hashtags', hashtag }))
	]);
	let postsOffset = $derived(users.length);
	let hashtagsOffset = $derived(users.length + posts.length);

	let activeIndex = $state(0);
	// Only true once the user has explicitly moved the highlight with arrow
	// keys; gates whether Enter selects a suggestion or reaches the form's
	// native submit for a plain free-text search.
	let hasHighlighted = $state(false);

	// Reset navigation state whenever the suggestion set changes.
	$effect(() => {
		if (flat) {
			activeIndex = 0;
			hasHighlighted = false;
		}
	});

	function activate(item: FlatItem | undefined) {
		if (!item) return;
		onclose();
		if (item.category === 'users') {
			goto(resolve(`/@${item.user.username}`));
		} else if (item.category === 'posts') {
			goto(resolve(`/posts/${item.post.id}`));
		} else {
			goto(resolve(`/search?q=%23${encodeURIComponent(item.hashtag.name)}`));
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !hasHighlighted) return;
		if (e.key === 'ArrowDown' || e.key === 'ArrowUp') hasHighlighted = true;
		const { index, action } = typeaheadNav(e.key, activeIndex, flat.length);
		if (action === 'none' && index === activeIndex) return;
		e.preventDefault();
		activeIndex = index;
		if (action === 'select') activate(flat[index]);
		else if (action === 'clear') onclose();
	}

	function handleFocusOut(e: FocusEvent) {
		const next = e.relatedTarget as Node | null;
		if (!next || !(e.currentTarget as HTMLElement).contains(next)) onclose();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if flat.length > 0}
	<div
		class="absolute top-full z-10 mt-1 w-full rounded-box border border-base-300 bg-base-100 p-2 shadow-lg shadow-slate-900/10"
		onfocusout={handleFocusOut}
	>
		<ul class="menu menu-sm w-full gap-1 p-0">
			{#if users.length > 0}
				<li class="menu-title px-2">Users</li>
				{#each users as user, i (user.username)}
					<li>
						<a
							href={resolve(`/@${user.username}`)}
							class="flex items-center gap-3 rounded-xl {i === activeIndex ? 'active' : ''}"
							onmouseenter={() => (activeIndex = i)}
						>
							<Avatar username={user.username} avatar={user.avatar} size="h-8 w-8" />
							<span class="min-w-0 flex-1">
								<span class="block truncate font-bold">{user.name || user.username}</span>
								<span class="block truncate text-xs text-base-content/60">@{user.username}</span>
							</span>
						</a>
					</li>
				{/each}
			{/if}

			{#if posts.length > 0}
				<li class="menu-title px-2">Posts</li>
				{#each posts as post, i (post.id)}
					<li>
						<a
							href={resolve(`/posts/${post.id}`)}
							class="flex items-center gap-3 rounded-xl {postsOffset + i === activeIndex
								? 'active'
								: ''}"
							onmouseenter={() => (activeIndex = postsOffset + i)}
						>
							<img
								class="h-8 w-8 shrink-0 rounded-md object-cover"
								src={imageUrl(post.filename)}
								alt=""
								loading="lazy"
							/>
							<span class="min-w-0 flex-1">
								<span class="block truncate font-bold">@{post.username}</span>
								{#if post.description}
									<span class="block truncate text-xs text-base-content/60">{post.description}</span>
								{/if}
							</span>
						</a>
					</li>
				{/each}
			{/if}

			{#if hashtags.length > 0}
				<li class="menu-title px-2">Hashtags</li>
				{#each hashtags as hashtag, i (hashtag.name)}
					<li>
						<a
							href={resolve(`/search?q=%23${encodeURIComponent(hashtag.name)}`)}
							class="flex items-center gap-3 rounded-xl {hashtagsOffset + i === activeIndex
								? 'active'
								: ''}"
							onmouseenter={() => (activeIndex = hashtagsOffset + i)}
							onclick={onclose}
						>
							<span
								class="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-base-300 text-base-content/60"
							>
								<Hash class="h-4 w-4" />
							</span>
							<span class="truncate font-bold">#{hashtag.name}</span>
						</a>
					</li>
				{/each}
			{/if}
		</ul>
	</div>
{/if}
