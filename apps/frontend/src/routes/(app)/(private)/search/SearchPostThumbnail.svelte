<script lang="ts">
	import Thumbnail from '$lib/components/Thumbnail.svelte';
	import type { SearchPostItem } from '$lib/server/api/search';

	let { post }: { post: SearchPostItem } = $props();
</script>

<Thumbnail
	post={{
		publicId: post.id,
		filename: post.filename,
		description: post.description,
		likes: post.likes
	}}
/>
