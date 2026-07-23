<script lang="ts">
	import PostCard from '$lib/components/PostCard.svelte';
	import { pageTitle } from '$lib/pageTitle';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const titleSegment = $derived(
		data.post.description ? data.post.description.slice(0, 60) : 'Post'
	);
</script>

<svelte:head>
	<title>{pageTitle(titleSegment)}</title>
</svelte:head>

<div class="mx-auto max-w-xl">
	<PostCard
		post={data.post}
		currentUsername={data.currentUser?.username ?? null}
		currentAvatar={data.currentUser?.avatar ?? null}
		singleView={true}
		comments={data.comments}
		nextCommentsCursor={data.nextCommentsCursor}
	/>
</div>
